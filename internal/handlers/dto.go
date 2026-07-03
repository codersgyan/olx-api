package handlers

import (
	"fmt"
	"strings"
	"time"
)

type CreateListingRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Price       int64  `json:"price"`
	City        string `json:"city"`
}

type CreateListingResponse struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
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
