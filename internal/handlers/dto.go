package handlers

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type CreateListingRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Price       int64    `json:"price"`
	City        string   `json:"city"`
	ImageKeys   []string `json:"image_keys"`
}

type CreateListingResponse struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type ImageResponse struct {
	ID        string `json:"id"`
	ObjectKey string `json:"object_key"`
	Position  int16  `json:"position"`
}

// {
// 	data: [{all fields data}],
// 	nextCursor: null
// }

type ListListingsResponse struct {
	Data       []GetListingResponse `json:"data"`
	NextCursor *string              `json:"next_cursor"`
}

type GetListingResponse struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Price       int64           `json:"price"`
	City        string          `json:"city"`
	Status      string          `json:"status"`
	UserID      uuid.UUID       `json:"user_id"`
	Images      []ImageResponse `json:"images"`
	CreatedAt   time.Time       `json:"created_at"`
}

type ValidationError struct {
	Field string
	Msg   string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Msg)
}

func (req CreateListingRequest) Validate() error {
	if strings.TrimSpace(req.Title) == "" {
		return &ValidationError{Field: "title", Msg: "must not be empty"}
	}
	if len(req.Title) > 200 {
		return &ValidationError{Field: "title", Msg: "must be at most 200 characters"}
	}
	if strings.TrimSpace(req.Description) == "" {
		return &ValidationError{Field: "description", Msg: "must not be empty"}
	}
	if len(req.Description) > 5000 {
		return &ValidationError{Field: "description", Msg: "must be at most 5000 characters"}
	}
	if req.Price <= 0 {
		return &ValidationError{Field: "price", Msg: "must be greater than zero"}
	}
	if strings.TrimSpace(req.City) == "" {
		return &ValidationError{Field: "city", Msg: "must not be empty"}
	}
	return nil
}
