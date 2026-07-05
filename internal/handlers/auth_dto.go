package handlers

import (
	"net/mail"
	"strings"
	"time"
)

type SignupRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (req SignupRequest) Validate() error {
	if strings.TrimSpace(req.Name) == "" {
		return &ValidationError{Field: "name", Msg: "must not be empty"}
	}

	if _, err := mail.ParseAddress(req.Email); err != nil {
		return &ValidationError{Field: "email", Msg: "must be a valid email address"}
	}

	if len(req.Password) < 8 {
		return &ValidationError{Field: "password", Msg: "must be at least 8 characters"}
	}

	return nil
}

type SignupResponse struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

type SigninRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (req SigninRequest) Validate() error {
	if _, err := mail.ParseAddress(req.Email); err != nil {
		return &ValidationError{Field: "email", Msg: "must be a valid email address"}
	}

	return nil
}

type SigninResponse struct {
	Token     string
	ExpiresIn int
}
