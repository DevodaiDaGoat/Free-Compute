package ratelimit

import (
	"sync"
	"time"
)

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

type Limiter struct {
	rps   float64
	burst float64
	items sync.Map
	stop  chan struct{}
}

func NewLimiter(rps int, burst int) *Limiter {
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
	close(l.stop)
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
		if b.lastSeen.Before(cutoff) {
			l.items.Delete(key)
		}
		return true
	})
}
