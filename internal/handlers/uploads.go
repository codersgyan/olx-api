package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/codersgyan/olx-api/internal/httpx"
	"github.com/codersgyan/olx-api/internal/middleware"
	"github.com/codersgyan/olx-api/internal/storage"
)

type UploadHandler struct {
	logger *slog.Logger
	store  *storage.Client
}

func NewUploadHandler(logger *slog.Logger, store *storage.Client) *UploadHandler {
	return &UploadHandler{
		logger: logger,
		store:  store,
	}
}

func (uh UploadHandler) Presign(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestId := middleware.RequestIDFromContext(ctx)
	log := uh.logger.With("request_id", requestId)

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		log.Error("no userid found in context")
		httpx.Error(w, http.StatusInternalServerError, "something went wrong", httpx.CodeInternalError)
		return
	}

	var req PresignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error("failed to decode", "err", err)
		httpx.Error(w, http.StatusBadRequest, "invalid body", httpx.CodeMalformedJSON)
		return
	}

	if len(req.Files) == 0 {
		// todo: log it
		httpx.Error(w, http.StatusBadRequest, "files must not be empty", httpx.CodeValidationFailed)
		return
	}

	if len(req.Files) > maxImagePerListing {
		httpx.Error(w, http.StatusBadRequest, fmt.Sprintf("a listing can have at most %d images", maxImagePerListing), httpx.CodeValidationFailed)
		return
	}

	uploads := make([]PresignUpload, 0, len(req.Files))
	for _, f := range req.Files {
		ext, ok := allowdContentTypes[f.ContentType]
		if !ok {
			httpx.Error(w, http.StatusBadRequest, "only image/jpeg, image/png, image/webp are allowed", httpx.CodeValidationFailed)
			return
		}

		if f.SizeBytes <= 0 || f.SizeBytes > maxImageBytes {
			httpx.Error(w, http.StatusBadRequest, fmt.Sprintf("size_bytes must be between 1 and %d", maxImageBytes), httpx.CodeValidationFailed)
			return
		}

		key := mintUploadKey(userID, ext)
		url, err := uh.store.PresignUpload(ctx, key, f.ContentType, presignTTL)
		if err != nil {
			log.Error("presign failed", "err", err, "key", key)
			httpx.Error(w, http.StatusInternalServerError, "something went wrong", httpx.CodeInternalError)
			return
		}

		uploads = append(uploads, PresignUpload{
			UploadURL: url,
			ObjectKey: key,
			ExpiresAt: time.Now().Add(presignTTL),
		})
	}

	log.Info("presigned upload issued", "count", len(uploads))
	w.Header().Set("Content-Type", "application/json")

	_ = json.NewEncoder(w).Encode(PresignResponse{Uploads: uploads})
}
