package jobs

import (
	"github.com/google/uuid"
)

const KindProcessListingImage = "process_listing_images"

type ImageSource struct {
	UploadKey string `json:"upload_key"` // which client sent uploads/
	ObjectKey string `json:"object_key"` // listings/sdfds
}

type ProcessImagePayload struct {
	ListingID uuid.UUID     `json:"listing_id"`
	Sources   []ImageSource `json:"sources"`
}
