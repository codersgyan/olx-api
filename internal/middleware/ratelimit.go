package middleware

import (
	"log/slog"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/codersgyan/olx-api/internal/httpx"
	"golang.org/x/time/rate"
)

// todo: move this to config (.env)
const (
	trustedHeader   = "X-Real-IP"
	cleanupInterval = time.Second * 10
	bucketTTL       = time.Minute * 10
)

type bucket struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type limiter struct {
	mu    sync.Mutex
	items map[string]*bucket
	limit rate.Limit
	burst int
}

// {
// 	"userip": bucket
// }

func (l *limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	item, ok := l.items[key]
	if !ok {
		item = &bucket{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.items[key] = item
	}
	item.lastSeen = time.Now()

	return item.limiter.Allow()
}

func RateLimit(logger *slog.Logger, limit rate.Limit, burst int) func(next http.Handler) http.Handler {
	l := limiter{
		items: make(map[string]*bucket),
		limit: limit,
		burst: burst,
	}

	go l.cleanup()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// will be called on every request
			userIP := clientIP(r, trustedHeader)
			if !l.allow(userIP) {
				// todo: add request id to this log
				logger.Info("rate limited", "key", userIP, "path", r.URL.Path, "request_id", RequestIDFromContext(r.Context()))

				retryAfter := "1"
				if limit > 0 && limit < 1 {
					retryAfter = strconv.Itoa(int(math.Ceil(float64(1 / limit))))
				}

				w.Header().Set("Retry-After", retryAfter)
				httpx.Error(w, http.StatusTooManyRequests, "too many requests, please slow down", httpx.CodeRateLimited)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// X-Real-IP
func clientIP(r *http.Request, trustedHeader string) string {
	if trustedHeader != "" {
		return strings.TrimSpace(r.Header.Get(trustedHeader))
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return ip
}

func (l *limiter) cleanup() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for now := range ticker.C {
		l.mu.Lock()
		for key, item := range l.items {
			if now.Sub(item.lastSeen) > bucketTTL {
				delete(l.items, key)
			}
		}
		l.mu.Unlock()
	}
}
