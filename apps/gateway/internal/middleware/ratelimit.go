package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/DevodaiDaGoat/Free-Compute/apps/gateway/internal/utils"
)

// tokenBucket is a simple token-bucket rate limiter state for a single client.
type tokenBucket struct {
	tokens     float64
	lastRefill time.Time
}

// rateLimiter tracks per-IP token buckets and refills them at a fixed rate.
type rateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*tokenBucket
	rps      float64
	capacity float64
}

// RateLimit returns middleware that enforces a per-IP request rate using a
// token-bucket algorithm. rps is the sustained requests-per-second allowed,
// which also serves as the burst capacity.
func RateLimit(rps int) func(http.Handler) http.Handler {
	if rps <= 0 {
		rps = 1
	}
	rl := &rateLimiter{
		buckets:  make(map[string]*tokenBucket),
		rps:      float64(rps),
		capacity: float64(rps),
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rl.allow(clientIP(r)) {
				utils.WriteError(w, http.StatusTooManyRequests, utils.CodeTooManyRequests, "rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// allow consumes a token for the given key, refilling based on elapsed time.
func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[key]
	if !ok {
		rl.buckets[key] = &tokenBucket{tokens: rl.capacity - 1, lastRefill: now}
		return true
	}

	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * rl.rps
	if b.tokens > rl.capacity {
		b.tokens = rl.capacity
	}
	b.lastRefill = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// clientIP extracts the client IP, preferring the X-Forwarded-For header.
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if idx := indexComma(fwd); idx >= 0 {
			return trimSpace(fwd[:idx])
		}
		return trimSpace(fwd)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func indexComma(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
