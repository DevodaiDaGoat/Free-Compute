package ratelimit

import (
	"sync"
	"time"
)

type bucket struct {
	mu       sync.Mutex
	tokens   float64
	lastSeen time.Time
}

type Limiter struct {
	rps      float64
	burst    float64
	items    sync.Map
	stop     chan struct{}
	stopOnce sync.Once
}

func NewLimiter(rps int, burst int) *Limiter {
	// Guard against config values that collapse to 0 after integer division
	// (e.g. RateLimitRPM<60 → rps=0), which would leave the bucket permanently
	// empty and block every request. Fall back to 1 rps / 1 burst.
	if rps <= 0 {
		rps = 1
	}
	if burst <= 0 {
		burst = 1
	}
	l := &Limiter{
		rps:   float64(rps),
		burst: float64(burst),
		stop:  make(chan struct{}),
	}
	go l.sweepLoop()
	return l
}

func (l *Limiter) Allow(key string) bool {
	return l.addToken(key)
}

func (l *Limiter) AllowUser(userID string) bool {
	return l.addToken("user:" + userID)
}

func (l *Limiter) addToken(key string) bool {
	now := time.Now()
	val, _ := l.items.LoadOrStore(key, &bucket{
		tokens:   l.burst,
		lastSeen: now,
	})
	b := val.(*bucket)
	b.mu.Lock()
	defer b.mu.Unlock()
	elapsed := now.Sub(b.lastSeen)
	b.tokens += l.rps * elapsed.Seconds()
	b.lastSeen = now
	if b.tokens > l.burst {
		b.tokens = l.burst
	}

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

func (l *Limiter) Stop() {
	l.stopOnce.Do(func() {
		close(l.stop)
	})
}

func (l *Limiter) sweepLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			l.evict()
		case <-l.stop:
			return
		}
	}
}

func (l *Limiter) evict() {
	cutoff := time.Now().Add(-1 * time.Hour)
	l.items.Range(func(key, value any) bool {
		b := value.(*bucket)
		b.mu.Lock()
		stale := b.lastSeen.Before(cutoff)
		b.mu.Unlock()
		if stale {
			l.items.Delete(key)
		}
		return true
	})
}
