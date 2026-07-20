package handlers

type PresignRequest struct {
	Files []PresignFile `json:"files"`
}

type PresignFile struct {
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
}
