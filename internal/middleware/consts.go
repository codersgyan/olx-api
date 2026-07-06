package middleware

type ctxKey int

const (
	requestIDKey ctxKey = iota
	userIDKey
)
