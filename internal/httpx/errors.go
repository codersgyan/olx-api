package httpx

import (
	"encoding/json"
	"net/http"
)

type Code string

const (
	CodeInvalidID     Code = "invalid_id"
	CodeInternalError Code = "internal_error"
)

type errorEnvelope struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
}

func Error(w http.ResponseWriter, status int, message string, code Code) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(errorEnvelope{Error: errorPayload{
		Code:    code,
		Message: message,
	}})
}
