package handlers

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	cursorVersion = 1
)

var errInvalidCursor = errors.New("invalid cursor")

type listingCursor struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Version   int       `json:"version"`
}

func encodeCursor(c listingCursor) (string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

func decodeCursor(after string) (listingCursor, error) {
	b, err := base64.RawURLEncoding.DecodeString(after)
	if err != nil {
		return listingCursor{}, errInvalidCursor
	}

	var c listingCursor
	if err := json.Unmarshal(b, &c); err != nil {
		return listingCursor{}, errInvalidCursor
	}

	if c.Version != cursorVersion || c.ID == uuid.Nil || c.CreatedAt.IsZero() {
		return listingCursor{}, errInvalidCursor
	}

	return c, nil
}
