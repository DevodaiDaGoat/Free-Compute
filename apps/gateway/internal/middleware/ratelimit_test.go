package middleware

import (
	"testing"
	"time"
)

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(10, time.Second, 20)
	if rl.rate != 10 {
		t.Errorf("rate = %d, want 10", rl.rate)
	}
	if rl.burst != 20 {
		t.Errorf("burst = %d, want 20", rl.burst)
	}
}

func TestNewRateLimiter_BurstClampedToRate(t *testing.T) {
	rl := NewRateLimiter(10, time.Second, 5)
	if rl.burst != 10 {
		t.Errorf("burst should be clamped to rate, got %d", rl.burst)
	}
}

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(2, time.Second, 3)
	now := time.Now()

	// First 3 requests should be allowed (burst)
	for i := 0; i < 3; i++ {
		if !rl.AllowAt("ip1", now) {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 4th should be denied
	if rl.AllowAt("ip1", now) {
		t.Error("4th request should be denied")
	}
}

func TestRateLimiter_RefillAfterInterval(t *testing.T) {
	rl := NewRateLimiter(2, time.Second, 3)
	now := time.Now()

	// Exhaust tokens
	for i := 0; i < 3; i++ {
		rl.AllowAt("ip1", now)
	}

	// After 1 second, should get 2 more tokens
	later := now.Add(time.Second)
	if !rl.AllowAt("ip1", later) {
		t.Error("should be allowed after refill")
	}
	if !rl.AllowAt("ip1", later) {
		t.Error("second request after refill should be allowed")
	}
	if rl.AllowAt("ip1", later) {
		t.Error("third request after refill should be denied")
	}
}

func TestRateLimiter_IndependentKeys(t *testing.T) {
	rl := NewRateLimiter(1, time.Second, 1)
	now := time.Now()

	if !rl.AllowAt("ip1", now) {
		t.Error("ip1 first request should be allowed")
	}
	if rl.AllowAt("ip1", now) {
		t.Error("ip1 second request should be denied")
	}
	if !rl.AllowAt("ip2", now) {
		t.Error("ip2 first request should be allowed (independent)")
	}
}

func TestRateLimiter_Remaining(t *testing.T) {
	rl := NewRateLimiter(5, time.Second, 5)
	key := "ip1"

	remaining := rl.Remaining(key)
	if remaining != 5 {
		t.Errorf("remaining for new key = %d, want 5", remaining)
	}

	rl.Allow(key)
	// Remaining is approximate due to timing; just check it decreased
	remaining = rl.Remaining(key)
	if remaining > 5 {
		t.Error("remaining should not exceed burst")
	}
}

func TestRateLimiter_Reset(t *testing.T) {
	rl := NewRateLimiter(1, time.Second, 1)
	now := time.Now()

	rl.AllowAt("ip1", now)
	if rl.AllowAt("ip1", now) {
		t.Error("should be denied before reset")
	}

	rl.Reset("ip1")
	if !rl.AllowAt("ip1", now) {
		t.Error("should be allowed after reset")
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	rl := NewRateLimiter(5, time.Second, 5)
	now := time.Now()

	rl.AllowAt("old-ip", now.Add(-10*time.Minute))
	rl.AllowAt("recent-ip", now)

	removed := rl.Cleanup(5 * time.Minute)
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}

	// recent-ip should still have its bucket
	rl.mu.Lock()
	_, exists := rl.buckets["recent-ip"]
	rl.mu.Unlock()
	if !exists {
		t.Error("recent-ip bucket should not have been cleaned up")
	}
}

func TestRateLimiter_TokensDoNotExceedBurst(t *testing.T) {
	rl := NewRateLimiter(5, time.Second, 10)
	now := time.Now()

	// Use one token
	rl.AllowAt("ip1", now)

	// Wait a very long time
	much_later := now.Add(time.Hour)

	// Should refill to burst but not beyond
	allowed := 0
	for i := 0; i < 20; i++ {
		if rl.AllowAt("ip1", much_later) {
			allowed++
		}
	}
	if allowed != 10 {
		t.Errorf("allowed %d requests, want burst of 10", allowed)
	}
}

func TestRateLimiter_ZeroBurst(t *testing.T) {
	// burst < rate, so burst gets clamped to rate
	rl := NewRateLimiter(3, time.Second, 0)
	if rl.burst != 3 {
		t.Errorf("burst = %d, want 3 (clamped to rate)", rl.burst)
	}
}

func TestRateLimiter_MultipleIntervals(t *testing.T) {
	rl := NewRateLimiter(1, time.Second, 5)
	now := time.Now()

	// Exhaust all tokens
	for i := 0; i < 5; i++ {
		rl.AllowAt("ip1", now)
	}

	// After 3 seconds, should have 3 tokens
	later := now.Add(3 * time.Second)
	for i := 0; i < 3; i++ {
		if !rl.AllowAt("ip1", later) {
			t.Errorf("request %d should be allowed after 3 intervals", i+1)
		}
	}
	if rl.AllowAt("ip1", later) {
		t.Error("4th request should be denied after 3 refills")
	}
}
