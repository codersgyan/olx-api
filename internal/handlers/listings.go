package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/codersgyan/olx-api/internal/httpx"
	"github.com/codersgyan/olx-api/internal/jobs"
	"github.com/codersgyan/olx-api/internal/middleware"
	"github.com/codersgyan/olx-api/internal/storage"
	"github.com/google/uuid"
)

type listing struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Price       int64     `json:"price"`
	City        string    `json:"city"`
	UserID      uuid.UUID `json:"user_id"`
	CreatedAt   time.Time `json:"created_at"`
}

type imageRow struct {
	ID        uuid.UUID
	ObjectKey string
	Position  int16
}

type ListingHandler struct {
	db      *sql.DB
	logger  *slog.Logger
	storage *storage.Client
}

func NewListingHandler(db *sql.DB, logger *slog.Logger, storage *storage.Client) *ListingHandler {
	return &ListingHandler{
		db:      db,
		logger:  logger,
		storage: storage,
	}
}

func (lh ListingHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Just for graceful shutdown demo
	// lh.db.QueryContext(ctx, `SELECT pg_sleep(10) FROM listings LIMIT 1`)

	rows, err := lh.db.QueryContext(ctx,
		`SELECT l.id, l.title, l.description, l.price, l.city, l.created_at, l.user_id, i.id as image_id, i.object_key, i.position
FROM listings l
LEFT JOIN images i ON i.listing_id = l.id
WHERE l.id IN (SELECT id FROM listings ORDER BY created_at DESC LIMIT 100)
ORDER BY created_at DESC, i.position`)
	if err != nil {
		lh.logger.Error("listings query error", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "Something went wrong", httpx.CodeInternalError)
		return
	}
	defer rows.Close()

	listings, err := lh.collapse(rows)
	if err != nil {
		lh.logger.Error("collapse listings failed", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "Something went wrong", httpx.CodeInternalError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(listings)
}

func (lh ListingHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestId := middleware.RequestIDFromContext(ctx)
	id := r.PathValue("id")

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		lh.logger.Error("no userid found in context", "request_id", requestId)
		httpx.Error(w, http.StatusInternalServerError, "something went wrong", httpx.CodeInternalError)
		return
	}

	// lh.logger.Debug("debug log", "listing_id", id)
	// lh.logger.Info("starting query", "listing_id", id)
	// lh.logger.Warn("warn log", "listing_id", id)

	_, err := lh.db.ExecContext(ctx,
		`DELETE FROM listings WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		lh.logger.Error("delete failed", "listing_id", id, "request_id", requestId, "err", err)
		httpx.Error(w, http.StatusInternalServerError, "Something went wrong", httpx.CodeInternalError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (lh ListingHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestId := middleware.RequestIDFromContext(ctx)
	log := lh.logger.With("request_id", requestId)
	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		log.Error("no userid found in context", "request_id", requestId)
		httpx.Error(w, http.StatusInternalServerError, "something went wrong", httpx.CodeInternalError)
		return
	}

	var req CreateListingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error("failed to decode", "request_id", requestId, "err", err)
		httpx.Error(w, http.StatusBadRequest, "invalid body", httpx.CodeMalformedJSON)
		return
	}

	if err := req.Validate(); err != nil {
		var verr *ValidationError
		errors.As(err, &verr)
		httpx.ValidationError(w, http.StatusUnprocessableEntity, err.Error(), httpx.CodeValidationFailed, verr.Field)
		return
	}

	listingID := uuid.New()
	// uploads/68cab9b8-3109-4f07-b2e9-25fc953c6cac/e6bebd4a-7478-4802-bca4-78befa971c65.jpg
	sources := make([]jobs.ImageSource, 0, len(req.ImageKeys))
	imageKeys := make([]string, 0, len(req.ImageKeys))
	for _, key := range req.ImageKeys {
		imageID, _, err := parseUploadKeys(key, userID)
		if err != nil {
			log.Error("failed to parse image key", "request_id", requestId, "err", err)
			httpx.Error(w, http.StatusBadRequest, "invalid image keys", httpx.CodeMalformedJSON)
			return
		}

		imageKeys = append(imageKeys, mintImageKey(imageID))
		sources = append(sources, jobs.ImageSource{
			UploadKey: key,
			ObjectKey: mintFinalObjectKey(listingID, imageID),
		})
	}

	for _, s := range sources {
		contentLength, contentType, err := lh.storage.Head(ctx, s.UploadKey)
		if errors.Is(err, storage.ErrNotFound) {
			// todo: log
			httpx.ValidationError(w, http.StatusBadRequest, "no object found at object_key", httpx.CodeMalformedJSON, "image_keys")
			return
		}

		if err != nil {
			// todo: log <= important
			httpx.ValidationError(w, http.StatusInternalServerError, "something went wrong", httpx.CodeInternalError, "image_keys")
			return
		}

		if contentLength <= 0 || contentLength > maxImageBytes {
			// todo: maybe delete the object ?
			httpx.ValidationError(w, http.StatusBadRequest, "uploaded object size violates the size limit", httpx.CodeValidationFailed, "image_keys")
			return
		}

		if _, ok := allowdContentTypes[contentType]; !ok {
			// todo: delete the object ?
			httpx.ValidationError(w, http.StatusBadRequest, "uploaded object has an unsupported content type", httpx.CodeValidationFailed, "image_keys")
			return
		}
	}

	tx, err := lh.db.BeginTx(ctx, nil)
	if err != nil {
		log.Error("begin tx failed", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "something went wrong", httpx.CodeInternalError)
		return
	}

	defer func() {
		_ = tx.Rollback()
	}()

	row := tx.QueryRowContext(ctx, `
	INSERT INTO listings (id, user_id, title, description, price, city) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, title, status, created_at`, listingID, userID, req.Title, req.Description, req.Price, req.City)

	var out CreateListingResponse
	if err := row.Scan(&out.ID, &out.Title, &out.Status, &out.CreatedAt); err != nil {
		lh.logger.Error("failed to insert", "request_id", requestId, "err", err)
		httpx.Error(w, http.StatusInternalServerError, "something went wrong", httpx.CodeInternalError)
		return
	}

	// images
	for i, imageKey := range imageKeys {
		_, err := tx.ExecContext(ctx, `
		INSERT INTO images (listing_id, object_key, position) VALUES ($1, $2, $3)`, listingID, imageKey, int16(i))
		if err != nil {
			log.Error("insert image failed", "object_key", imageKey, "err", err)
			httpx.Error(w, http.StatusInternalServerError, "something went wrong", httpx.CodeInternalError)
			return
		}
	}

	payload, err := json.Marshal(jobs.ProcessImagePayload{
		ListingID: listingID,
		Sources:   sources,
	})
	if err != nil {
		log.Error("marshal job payload failed", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "something went wrong", httpx.CodeInternalError)
		return
	}

	_, err = tx.ExecContext(ctx, `
	INSERT INTO jobs (kind, payload) VALUES ($1, $2)`, jobs.KindProcessListingImage, payload)
	if err != nil {
		log.Error("insert job failed", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "something went wrong", httpx.CodeInternalError)
		return
	}

	if err := tx.Commit(); err != nil {
		log.Error("commit failed", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "something went wrong", httpx.CodeInternalError)
		return
	}

	log.Info("listing created", "listing_id", out.ID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	_ = json.NewEncoder(w).Encode(out)
}

func (lh ListingHandler) collapse(rows *sql.Rows) ([]GetListingResponse, error) {
	listings := []GetListingResponse{}
	images := map[string][]imageRow{}
	seen := make(map[string]struct{})

	for rows.Next() {
		var l GetListingResponse
		var imgID uuid.NullUUID
		var objKey sql.NullString
		var position sql.NullInt16

		if err := rows.Scan(&l.ID, &l.Title, &l.Description, &l.Price, &l.City, &l.CreatedAt, &l.UserID, &imgID, &objKey, &position); err != nil {
			lh.logger.Error("rows scan error", "err", err)
			return nil, err
		}

		if _, ok := seen[l.ID]; !ok {
			seen[l.ID] = struct{}{}
			listings = append(listings, l)
		}

		if imgID.Valid && objKey.Valid && position.Valid {
			images[l.ID] = append(images[l.ID], imageRow{
				ID:        imgID.UUID,
				ObjectKey: objKey.String,
				Position:  position.Int16,
			})
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range listings {
		listings[i].Images = lh.toImageResponse(images[listings[i].ID])
	}

	return listings, nil
}

func (lh ListingHandler) toImageResponse(imgRows []imageRow) []ImageResponse {
	out := make([]ImageResponse, 0, len(imgRows))
	for _, img := range imgRows {
		out = append(out, ImageResponse{
			ID:        img.ID.String(),
			ObjectKey: img.ObjectKey,
			Position:  img.Position,
		})
	}
	return out
}
