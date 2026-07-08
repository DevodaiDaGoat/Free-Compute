package tunnel

import (
	"bytes"
	"container/list"
	"crypto/sha256"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	cacheDefaultCapacity    = 10000
	cacheDefaultTTL        = 5 * time.Minute
	cacheMaxBodyBytes      = 1 << 20
	cacheCleanupInterval   = 1 * time.Minute
	cacheBypassDirective   = "no-store"
)

type CacheEntry struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
	CreatedAt  time.Time
	ExpiresAt  time.Time
	ETag       string
}

type cacheItem struct {
	key   string
	entry *CacheEntry
	elem  *list.Element
}

type CacheStats struct {
	Size     int     `json:"size"`
	Capacity int     `json:"capacity"`
	Hits     int64   `json:"hits"`
	Misses   int64   `json:"misses"`
	HitRatio float64 `json:"hitRatio"`
}

type Cache struct {
	mu            sync.RWMutex
	maxSize       int
	defaultTTL    time.Duration
	entries       map[string]*cacheItem
	lruList       *list.List
	hits          int64
	misses        int64
	cleanupTicker *time.Ticker
	done          chan struct{}
}

func NewCache(maxSize int, defaultTTL time.Duration) *Cache {
	if maxSize <= 0 {
		maxSize = cacheDefaultCapacity
	}
	if defaultTTL <= 0 {
		defaultTTL = cacheDefaultTTL
	}

	c := &Cache{
		maxSize:       maxSize,
		defaultTTL:    defaultTTL,
		entries:       make(map[string]*cacheItem),
		lruList:       list.New(),
		cleanupTicker: time.NewTicker(cacheCleanupInterval),
		done:          make(chan struct{}),
	}

	go c.cleanupLoop()
	return c
}

func (c *Cache) Get(key string) (*CacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, ok := c.entries[key]
	if !ok {
		c.misses++
		return nil, false
	}

	if time.Now().After(item.entry.ExpiresAt) {
		c.removeLocked(key)
		c.misses++
		return nil, false
	}

	c.lruList.MoveToFront(item.elem)
	c.hits++
	return item.entry, true
}

func (c *Cache) Set(key string, entry *CacheEntry) {
	if len(entry.Body) > cacheMaxBodyBytes {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, ok := c.entries[key]; ok {
		c.lruList.Remove(existing.elem)
		delete(c.entries, key)
	}

	for c.lruList.Len() >= c.maxSize {
		back := c.lruList.Back()
		if back == nil {
			break
		}
		item := back.Value.(*cacheItem)
		c.removeLocked(item.key)
	}

	item := &cacheItem{
		key:   key,
		entry: entry,
	}
	item.elem = c.lruList.PushFront(item)
	c.entries[key] = item
}

func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.removeLocked(key)
}

func (c *Cache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	var ratio float64
	if total > 0 {
		ratio = float64(c.hits) / float64(total)
	}

	return CacheStats{
		Size:     len(c.entries),
		Capacity: c.maxSize,
		Hits:     c.hits,
		Misses:   c.misses,
		HitRatio: ratio,
	}
}

func (c *Cache) Stop() {
	close(c.done)
	c.cleanupTicker.Stop()
}

func (c *Cache) removeLocked(key string) {
	item, ok := c.entries[key]
	if !ok {
		return
	}
	c.lruList.Remove(item.elem)
	delete(c.entries, key)
}

func (c *Cache) cleanupLoop() {
	for {
		select {
		case <-c.cleanupTicker.C:
			c.mu.Lock()
			now := time.Now()
			for key, item := range c.entries {
				if now.After(item.entry.ExpiresAt) {
					c.removeLocked(key)
				}
			}
			c.mu.Unlock()
		case <-c.done:
			return
		}
	}
}

func CacheKeyFromRequest(r *http.Request) string {
	h := sha256.New()
	h.Write([]byte(r.Method))
	h.Write([]byte{0})
	h.Write([]byte(r.Host))
	h.Write([]byte{0})
	h.Write([]byte(r.URL.Path))
	h.Write([]byte{0})

	params := r.URL.Query()
	if len(params) > 0 {
		keys := make([]string, 0, len(params))
		for k := range params {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			h.Write([]byte(k))
			h.Write([]byte{'='})
			h.Write([]byte(strings.Join(params[k], ",")))
			h.Write([]byte{0})
		}
	}

	vary := r.Header.Get("Vary")
	if vary != "" {
		for _, field := range strings.Split(vary, ",") {
			field = strings.TrimSpace(field)
			if strings.EqualFold(field, "Accept-Encoding") {
				h.Write([]byte(r.Header.Get("Accept-Encoding")))
				h.Write([]byte{0})
			}
		}
	}

	return fmt.Sprintf("%x", h.Sum(nil)[:16])
}

type CacheControl struct {
	Public          bool
	Private         bool
	NoCache         bool
	NoStore         bool
	Immutable       bool
	MaxAge          time.Duration
	SMaxAge         time.Duration
	MustRevalidate  bool
	ProxyRevalidate bool
}

func ParseCacheControl(header string) CacheControl {
	var cc CacheControl
	if header == "" {
		return cc
	}

	for _, directive := range strings.Split(header, ",") {
		directive = strings.TrimSpace(strings.ToLower(directive))
		switch {
		case directive == "public":
			cc.Public = true
		case directive == "private":
			cc.Private = true
		case directive == "no-cache":
			cc.NoCache = true
		case directive == "no-store":
			cc.NoStore = true
		case directive == "immutable":
			cc.Immutable = true
		case directive == "must-revalidate":
			cc.MustRevalidate = true
		case directive == "proxy-revalidate":
			cc.ProxyRevalidate = true
		case strings.HasPrefix(directive, "max-age="):
			if seconds, err := strconv.Atoi(strings.TrimPrefix(directive, "max-age=")); err == nil && seconds >= 0 {
				cc.MaxAge = time.Duration(seconds) * time.Second
			}
		case strings.HasPrefix(directive, "s-maxage="):
			if seconds, err := strconv.Atoi(strings.TrimPrefix(directive, "s-maxage=")); err == nil && seconds >= 0 {
				cc.SMaxAge = time.Duration(seconds) * time.Second
			}
		}
	}

	return cc
}

func isCacheableMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead
}

func responseTTL(respCC CacheControl, reqCC CacheControl, defaultTTL time.Duration) (time.Duration, bool) {
	if respCC.NoStore || respCC.Private {
		return 0, false
	}

	if respCC.SMaxAge > 0 {
		return respCC.SMaxAge, true
	}

	if respCC.MaxAge > 0 {
		return respCC.MaxAge, true
	}

	if respCC.Immutable {
		return 365 * 24 * time.Hour, true
	}

	if respCC.NoCache {
		return 0, false
	}

	if reqCC.NoCache {
		return 0, false
	}

	return defaultTTL, true
}

func cacheableResponse(status int) bool {
	return status == http.StatusOK ||
		status == http.StatusNoContent ||
		status == http.StatusMovedPermanently ||
		status == http.StatusFound ||
		status == http.StatusNotModified
}

func CacheHandler(next http.Handler, cache *Cache) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isCacheableMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		reqCC := ParseCacheControl(r.Header.Get("Cache-Control"))
		if reqCC.NoStore {
			next.ServeHTTP(w, r)
			return
		}

		key := CacheKeyFromRequest(r)
		if !reqCC.NoCache {
			if entry, ok := cache.Get(key); ok {
				for k, vals := range entry.Headers {
					for _, v := range vals {
						w.Header()[k] = append(w.Header()[k], v)
					}
				}
				w.Header().Set("X-Cache", "HIT")
				w.Header().Set("X-Cache-Key", key)
				w.WriteHeader(entry.StatusCode)
				w.Write(entry.Body)
				return
			}
		}

		crw := &cacheResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}
		next.ServeHTTP(crw, r)

		respCC := ParseCacheControl(w.Header().Get("Cache-Control"))
		if crw.captured {
			ttl, ok := responseTTL(respCC, reqCC, cache.defaultTTL)
			if ok && cacheableResponse(crw.statusCode) {
				entry := &CacheEntry{
					StatusCode: crw.statusCode,
					Headers:    crw.header.Clone(),
					Body:       crw.body.Bytes(),
					CreatedAt:  time.Now(),
					ETag:       crw.header.Get("ETag"),
				}
				entry.ExpiresAt = entry.CreatedAt.Add(ttl)
				cache.Set(key, entry)
			}
		} else {
			respCC := ParseCacheControl(w.Header().Get("Cache-Control"))
			if respCC.Immutable {
				ttl := 365 * 24 * time.Hour
				entry := &CacheEntry{
					StatusCode: http.StatusOK,
					Headers:    w.Header().Clone(),
					Body:       nil,
					CreatedAt:  time.Now(),
					ExpiresAt:  time.Now().Add(ttl),
				}
				cache.Set(key, entry)
			}
		}

		w.Header().Set("X-Cache", "MISS")
	})
}

type cacheResponseWriter struct {
	http.ResponseWriter
	statusCode int
	header     http.Header
	body       bytes.Buffer
	captured   bool
}

func (w *cacheResponseWriter) WriteHeader(code int) {
	if w.captured {
		return
	}
	w.captured = true
	w.statusCode = code
	w.header = w.ResponseWriter.Header().Clone()
	w.ResponseWriter.WriteHeader(code)
}

func (w *cacheResponseWriter) Write(b []byte) (int, error) {
	if !w.captured {
		w.WriteHeader(http.StatusOK)
	}
	if w.body.Len()+len(b) <= cacheMaxBodyBytes {
		w.body.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

func isCacheBypassPath(path string) bool {
	return strings.HasPrefix(path, "/proxy/") ||
		strings.HasPrefix(path, "/ws/") ||
		strings.HasPrefix(path, "/connect/") ||
		strings.HasPrefix(path, "/agent/") ||
		strings.HasPrefix(path, "/signal/")
}
