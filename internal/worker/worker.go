package worker

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/codersgyan/olx-api/internal/imaging"
	"github.com/codersgyan/olx-api/internal/jobs"
	"github.com/codersgyan/olx-api/internal/storage"
	"github.com/google/uuid"
)

const (
	maxAttempts  = 3
	pollInterval = time.Second * 3
	maxEdge      = 2048
	quality      = 85

	statusReject = "rejected"
	statusReady  = "ready"
)

var ErrPermanent = errors.New("permanent failure")

func permanentf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrPermanent, fmt.Sprintf(format, args...))
}

type Worker struct {
	db      *sql.DB
	storage *storage.Client
	logger  *slog.Logger
}

func New(db *sql.DB, storage *storage.Client, logger *slog.Logger) *Worker {
	return &Worker{
		db:      db,
		storage: storage,
		logger:  logger,
	}
}

func (w *Worker) Run(ctx context.Context) {
	w.logger.Info("worker started...")
	for {
		isEmpty, err := w.claimAndProcess(ctx)
		if err != nil {
			w.logger.Error("job failed", "err", err)
		}

		if isEmpty {
			time.Sleep(pollInterval)
		}
	}
}

func (w *Worker) claimAndProcess(ctx context.Context) (bool, error) {

	var (
		id       int64
		kind     string
		payload  []byte
		attempts int
	)

	err := w.db.QueryRowContext(ctx, `
	UPDATE jobs
		SET attempts = attempts + 1,
		run_after = NOW() + LEAST(
			(INTERVAL '30 seconds') * POWER(2, attempts),
			INTERVAL '1 hour')
		WHERE id = (
			SELECT id FROM jobs
			WHERE run_after <= NOW()
			AND attempts < $1
			ORDER BY run_after, id
			FOR UPDATE SKIP LOCKED
			LIMIT 1)
		RETURNING id, kind, payload, attempts`, maxAttempts).Scan(&id, &kind, &payload, &attempts)

	if errors.Is(err, sql.ErrNoRows) {
		// means queue is empty
		w.logger.Info("queue is empty", "retry", pollInterval)
		return true, nil
	}

	if err != nil {
		return false, err
	}

	w.logger.Info("job processing started", "job_id", id, "kind", kind, "attempt", attempts)

	if err := w.process(ctx, id, kind, payload); err != nil {
		if errors.Is(err, ErrPermanent) || attempts >= maxAttempts {
			w.deadletter(ctx, id, kind, payload)
			return false, err
		}

		if _, err := w.db.ExecContext(ctx, `
		UPDATE jobs SET last_error = $1 WHERE id = $2`, err.Error(), id); err != nil {
			w.logger.Error("recording job error failed", "err", err)
		}
		return false, err
	}

	w.logger.Info("job completed", "job_id", id)
	return false, nil
}

func (w *Worker) process(ctx context.Context, jobID int64, kind string, payload []byte) error {
	switch kind {
	case jobs.KindProcessListingImage:
		var p jobs.ProcessImagePayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return permanentf("unmarshal payload %v", err)
		}
		return w.processImages(ctx, jobID, p)
	default:
		return permanentf("unknown job kind %q", kind)
	}
}

func (w *Worker) processImages(ctx context.Context, jobID int64, payload jobs.ProcessImagePayload) error {
	log := w.logger.With("job_id", jobID, "listing_id", payload.ListingID)

	normalized := make([][]byte, 0, len(payload.Sources)) //tradeof - memory consumption - all 3 images are stored here..
	for _, s := range payload.Sources {
		raw, err := w.storage.Get(ctx, s.UploadKey)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				log.Error("upload gone or never uploaded", "upload_key", s.UploadKey)
				return w.finish(ctx, jobID, payload.ListingID, statusReject)
			}

			return fmt.Errorf("get upload %q: %w", s.UploadKey, err)
		}

		b, err := io.ReadAll(raw)
		raw.Close()
		if err != nil {
			return fmt.Errorf("read upload %q: %w", s.UploadKey, err)
		}

		out, err := imaging.Normalize(bytes.NewReader(b), maxEdge, quality)
		if err != nil {
			return permanentf("normalize %q: %v", s.UploadKey, err)
		}

		normalized = append(normalized, out)
	}

	for i, img := range normalized {
		key := payload.Sources[i].ObjectKey
		if err := w.storage.Put(ctx, key, bytes.NewReader(img)); err != nil {
			// todo: add image delete job to outbox
			return fmt.Errorf("put %q: %w", key, err)
		}
	}

	return w.finish(ctx, jobID, payload.ListingID, statusReady)
}

func (w *Worker) finish(ctx context.Context, jobID int64, listingID uuid.UUID, status string) error {
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, `
	UPDATE listings SET status = $1 WHERE id = $2`, status, listingID); err != nil {
		return fmt.Errorf("update listing %q status %q: %w", listingID, status, err)
	}

	if _, err := tx.ExecContext(ctx, `
	DELETE FROM jobs WHERE id = $1`, jobID); err != nil {
		return fmt.Errorf("delete job %d: %w", jobID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

func (w *Worker) deadletter(ctx context.Context, jobID int64, kind string, payload []byte) {
	// todo: 1. make a new job and put it in jobs table - type
	// todo: 2. delete the job, mark the listing as rejected -> finish()

	// first check if listing id exists yes -> call finish() no -> delete the job
	log := w.logger.With("job_id", jobID, "kind", kind)

	listingID, ok := w.listingIDFor(kind, payload)
	if !ok {
		log.Error("job dead lettered no resolvable listing")
		if _, err := w.db.ExecContext(ctx, `DELETE FROM jobs WHERE id = $1`, jobID); err != nil {
			log.Error("deleting dead job faild", "err", err)
		}
		return
	}

	log.Error("job dead-lettered", "listing_id", listingID)

	if err := w.finish(ctx, jobID, listingID, statusReject); err != nil {
		log.Error("dead lettering failed", "err", err)
	}
}

func (w *Worker) listingIDFor(kind string, payload []byte) (uuid.UUID, bool) {
	switch kind {
	case jobs.KindProcessListingImage:
		var p jobs.ProcessImagePayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return uuid.Nil, false
		}
		return p.ListingID, true
	default:
		return uuid.Nil, false
	}
}
