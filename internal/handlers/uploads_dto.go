package handlers

import "time"

type PresignRequest struct {
	Files []PresignFile `json:"files"`
}

type PresignFile struct {
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
}

type PresignUpload struct {
	UploadURL string    `json:"upload_url"`
	ObjectKey string    `json:"object_key"`
	ExpiresAt time.Time `json:"expires_at"`
}

type PresignResponse struct {
	Uploads []PresignUpload `json:"uploads"`
}
