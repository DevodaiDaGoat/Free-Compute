package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/httprate"
)

// AuthRateLimit applies aggressive rate limiting to authentication endpoints.
// 5 requests per minute per IP to prevent brute-force and credential stuffing.
func AuthRateLimit() func(http.Handler) http.Handler {
	return httprate.Limit(
		5,
		time.Minute,
		httprate.WithKeyFuncs(httprate.KeyByIP),
		httprate.WithLimitHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"error":"rate limit exceeded, try again later"}`, http.StatusTooManyRequests)
		})),
	)
}

// GeneralRateLimit applies standard rate limiting to authenticated API endpoints.
// 100 requests per minute per IP.
func GeneralRateLimit() func(http.Handler) http.Handler {
	return httprate.Limit(
		100,
		time.Minute,
		httprate.WithKeyFuncs(httprate.KeyByIP),
		httprate.WithLimitHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
		})),
	)
}
