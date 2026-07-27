package handlers

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	maxImageBytes      = 5 * 1024 * 1024 // 5mb
	maxImagePerListing = 10
	uploadPrefix       = "uploads"
	listingPrefix      = "listings"
	presignTTL         = 5 * time.Minute
)

var allowdContentTypes = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
}

func mintUploadKey(userID uuid.UUID, ext string) string {
	return fmt.Sprintf("%s/%s/%s.%s", uploadPrefix, userID, uuid.New(), ext)
}

// {listingPrefix}/{listingId}/{imageID}.jpg
func mintFinalObjectKey(listingId, imageID uuid.UUID, ext string) string {
	return fmt.Sprintf("%s/%s/%s.%s", listingPrefix, listingId, imageID, ext)
}

// uploads/68cab9b8-3109-4f07-b2e9-25fc953c6cac/e6bebd4a-7478-4802-bca4-78befa971c65.jpg
func parseUploadKeys(key string, userID uuid.UUID) (uuid.UUID, string, error) {
	rest, ok := strings.CutPrefix(key, fmt.Sprintf("%s/%s/", uploadPrefix, userID))
	if !ok {
		return uuid.Nil, "", errors.New("object_key does not belongs to this user")
	}

	if strings.Contains(rest, "/") {
		return uuid.Nil, "", errors.New("object_key has unexpected shape")
	}

	// e6bebd4a-7478-4802-bca4-78befa971c65.jpg
	base, ext, ok := strings.Cut(rest, ".")
	if !ok || !allowedExt(ext) {
		return uuid.Nil, "", errors.New("object key invalid extention")
	}

	id, err := uuid.Parse(base)
	if err != nil {
		return uuid.Nil, "", errors.New("Object_key is not a well-formed upload key")
	}

	return id, ext, nil
}

func allowedExt(ext string) bool {
	for _, e := range allowdContentTypes {
		if e == ext {
			return true
		}
	}
	return false
}
