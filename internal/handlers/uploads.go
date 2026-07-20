package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

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
		// Actually presigning
		// uh.store.Presign

	}

	w.Write([]byte("ok"))
}
