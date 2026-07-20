package handlers

import (
	"fmt"

	"github.com/google/uuid"
)

const (
	maxImageBytes      = 5 * 1024 * 1024 // 5mb
	maxImagePerListing = 10
	uploadPrefix       = "uploads"
)

var allowdContentTypes = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
}

func mintUploadKey(userID uuid.UUID, ext string) string {
	return fmt.Sprintf("%s/%s/%s.%s", uploadPrefix, userID, uuid.New(), ext)
}
