package middleware

import (
	"net/http"
	"sync"

	"github.com/DevodaiDaGoat/Free-Compute/internal/errors"
	"golang.org/x/time/rate"
)

// RateLimiter provides per-IP rate limiting.
type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*rate.Limiter
	rps      rate.Limit
	burst    int
}

// NewRateLimiter creates a limiter allowing rps requests/sec with the given burst.
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	return &RateLimiter{
		visitors: make(map[string]*rate.Limiter),
		rps:      rate.Limit(rps),
		burst:    burst,
	}
}

func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.visitors[ip]
	if !exists {
		limiter = rate.NewLimiter(rl.rps, rl.burst)
		rl.visitors[ip] = limiter
	}
	return limiter
}

// Middleware returns an HTTP middleware that enforces the rate limit.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			ip = fwd
		}

		if !rl.getLimiter(ip).Allow() {
			errors.Wrap(http.StatusTooManyRequests, "rate limit exceeded", nil).WriteJSON(w)
			return
		}

		next.ServeHTTP(w, r)
	})
}
