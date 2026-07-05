package middleware

import (
	"sync"
	"time"
)

// RateLimiter implements a per-key token bucket rate limiter.
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     int           // tokens per interval
	interval time.Duration // refill interval
	burst    int           // max tokens
}

type bucket struct {
	tokens    int
	lastCheck time.Time
}

// NewRateLimiter creates a rate limiter allowing `rate` requests per `interval` with a max burst.
func NewRateLimiter(rate int, interval time.Duration, burst int) *RateLimiter {
	if burst < rate {
		burst = rate
	}
	return &RateLimiter{
		buckets:  make(map[string]*bucket),
		rate:     rate,
		interval: interval,
		burst:    burst,
	}
}

// Allow checks if a request from the given key should be allowed.
func (rl *RateLimiter) Allow(key string) bool {
	return rl.AllowAt(key, time.Now())
}

// AllowAt checks if a request is allowed at a specific time (for testing).
func (rl *RateLimiter) AllowAt(key string, now time.Time) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, exists := rl.buckets[key]
	if !exists {
		rl.buckets[key] = &bucket{
			tokens:    rl.burst - 1,
			lastCheck: now,
		}
		return true
	}

	elapsed := now.Sub(b.lastCheck)
	refills := int(elapsed / rl.interval)
	if refills > 0 {
		b.tokens += refills * rl.rate
		if b.tokens > rl.burst {
			b.tokens = rl.burst
		}
		b.lastCheck = b.lastCheck.Add(time.Duration(refills) * rl.interval)
	}

	if b.tokens > 0 {
		b.tokens--
		return true
	}
	return false
}

// Remaining returns the number of tokens left for a key.
func (rl *RateLimiter) Remaining(key string) int {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, exists := rl.buckets[key]
	if !exists {
		return rl.burst
	}

	elapsed := time.Since(b.lastCheck)
	refills := int(elapsed / rl.interval)
	tokens := b.tokens + refills*rl.rate
	if tokens > rl.burst {
		tokens = rl.burst
	}
	return tokens
}

// Reset clears the bucket for a key.
func (rl *RateLimiter) Reset(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.buckets, key)
}

// Cleanup removes stale buckets older than the given duration.
func (rl *RateLimiter) Cleanup(staleAfter time.Duration) int {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-staleAfter)
	removed := 0
	for k, b := range rl.buckets {
		if b.lastCheck.Before(cutoff) {
			delete(rl.buckets, k)
			removed++
		}
	}
	return removed
}
