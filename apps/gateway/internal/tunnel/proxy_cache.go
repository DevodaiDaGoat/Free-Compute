package tunnel

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

const defaultProxyCacheTTL = 300
const defaultProxyCacheMaxSize = 512
const proxyCacheMaxEntrySize = 10 * 1024 * 1024

type proxyCacheEntry struct {
	data        []byte
	contentType string
	statusCode  int
	headers     http.Header
	timestamp   time.Time
	ttl         time.Duration
	size        int64
}

func (e *proxyCacheEntry) expired() bool {
	if e.ttl <= 0 {
		return false
	}
	return time.Since(e.timestamp) > e.ttl
}

type ProxyCache struct {
	mu       sync.RWMutex
	entries  map[string]*proxyCacheEntry
	lru      *lruCache
	maxSize  int64
	usedSize int64
	logger   *log.Logger
}

type lruCache struct {
	cap   int
	list  []string
	index map[string]int
}

func newLRUCache(cap int) *lruCache {
	return &lruCache{cap: cap, index: make(map[string]int)}
}

func (c *lruCache) touch(key string) {
	if idx, ok := c.index[key]; ok {
		if idx > 0 {
			c.list = append(c.list[:idx], c.list[idx+1:]...)
			c.list = append([]string{key}, c.list...)
			for i, k := range c.list {
				c.index[k] = i
			}
		}
	}
}

func (c *lruCache) add(key string) {
	if _, exists := c.index[key]; exists {
		c.touch(key)
		return
	}
	if len(c.list) >= c.cap {
		evict := c.list[len(c.list)-1]
		delete(c.index, evict)
		c.list = c.list[:len(c.list)-1]
	}
	c.list = append([]string{key}, c.list...)
	for i, k := range c.list {
		c.index[k] = i
	}
}

func (c *lruCache) remove(key string) {
	if idx, ok := c.index[key]; ok {
		c.list = append(c.list[:idx], c.list[idx+1:]...)
		delete(c.index, key)
		for i, k := range c.list {
			c.index[k] = i
		}
	}
}

func NewProxyCache(maxSizeMB int, logger *log.Logger) *ProxyCache {
	if logger == nil {
		logger = log.Default()
	}
	maxSize := int64(maxSizeMB) * 1024 * 1024
	if maxSize <= 0 {
		maxSize = int64(defaultProxyCacheMaxSize) * 1024 * 1024
	}
	cap := 1024
	if maxSize > 0 {
		cap = int(maxSize / (64 * 1024))
		if cap > 65536 {
			cap = 65536
		}
	}
	return &ProxyCache{
		entries: make(map[string]*proxyCacheEntry),
		lru:     newLRUCache(cap),
		maxSize: maxSize,
		logger:  logger,
	}
}

func (c *ProxyCache) ServeCached(w http.ResponseWriter, r *http.Request, route *Route) bool {
	c.mu.RLock()
	entry, ok := c.cacheKey(route, r)
	c.mu.RUnlock()
	if !ok || entry == nil {
		return false
	}
	if entry.expired() {
		c.Invalidate(route, r)
		return false
	}
	if c.shouldBypassCache(entry, r) {
		return false
	}
	c.mu.Lock()
	c.lru.touch(c.key(route, r))
	c.mu.Unlock()

	w.Header().Set("X-Cache", "HIT")
	for k, v := range entry.headers {
		for _, vv := range v {
			w.Header().Add(k, vv)
		}
	}
	if entry.contentType != "" {
		w.Header().Set("Content-Type", entry.contentType)
	}
	if etag := entry.headers.Get("ETag"); etag != "" {
		w.Header().Set("ETag", etag)
	}
	if lm := entry.headers.Get("Last-Modified"); lm != "" {
		w.Header().Set("Last-Modified", lm)
	}
	w.WriteHeader(entry.statusCode)
	_, _ = w.Write(entry.data)
	return true
}

func (c *ProxyCache) key(route *Route, r *http.Request) string {
	path := r.URL.Path
	if route != nil {
		path = route.ID + ":" + r.URL.String()
	} else {
		path = r.URL.String()
	}
	h := sha256.Sum256([]byte(path))
	return hex.EncodeToString(h[:])
}

func (c *ProxyCache) cacheKey(route *Route, r *http.Request) (*proxyCacheEntry, bool) {
	key := c.key(route, r)
	entry, ok := c.entries[key]
	return entry, ok
}

func (c *ProxyCache) shouldBypassCache(entry *proxyCacheEntry, r *http.Request) bool {
	if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete || r.Method == http.MethodPatch {
		return true
	}
	if cacheControl := entry.headers.Get("Cache-Control"); strings.Contains(cacheControl, "no-store") || strings.Contains(cacheControl, "private") {
		return true
	}
	auth := r.Header.Get("Authorization")
	if auth != "" {
		return true
	}
	return false
}

func (c *ProxyCache) Store(route *Route, r *http.Request, resp *http.Response, body []byte) {
	if c == nil {
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return
	}
	contentLength := int64(len(body))
	if contentLength > proxyCacheMaxEntrySize {
		return
	}
	if c.shouldBypassCache(nil, r) {
		return
	}
	ttl := c.getTTL(route, resp)
	if ttl <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	key := c.key(route, r)
	if existing, ok := c.entries[key]; ok {
		c.usedSize -= existing.size
		c.lru.remove(key)
		delete(c.entries, key)
	}

	if c.usedSize+contentLength > c.maxSize {
		c.evictLocked(contentLength)
	}
	headers := resp.Header.Clone()
	entry := &proxyCacheEntry{
		data:        body,
		contentType: resp.Header.Get("Content-Type"),
		statusCode:  resp.StatusCode,
		headers:     headers,
		timestamp:   time.Now(),
		ttl:         ttl,
		size:        contentLength,
	}
	c.entries[key] = entry
	c.lru.add(key)
	c.usedSize += contentLength
}

func (c *ProxyCache) getTTL(route *Route, resp *http.Response) time.Duration {
	if route != nil && route.Cache != nil && route.Cache.TTLSeconds > 0 {
		return time.Duration(route.Cache.TTLSeconds) * time.Second
	}
	if route != nil && route.Cache != nil && route.Cache.MaxSizeMB > 0 {
		return time.Duration(defaultProxyCacheTTL) * time.Second
	}
	cacheControl := resp.Header.Get("Cache-Control")
	if strings.Contains(cacheControl, "no-cache") || strings.Contains(cacheControl, "no-store") || strings.Contains(cacheControl, "private") {
		return 0
	}
	if maxAge := parseMaxAge(cacheControl); maxAge > 0 {
		return maxAge
	}
	if expires := resp.Header.Get("Expires"); expires != "" {
		if t, err := time.Parse(time.RFC1123, expires); err == nil {
			d := time.Until(t)
			if d > 0 && d < 24*time.Hour {
				return d
			}
		}
	}
	if route != nil && route.Cache != nil && route.Cache.CacheControl == "respect-origin" {
		return time.Duration(defaultProxyCacheTTL) * time.Second
	}
	return time.Duration(defaultProxyCacheTTL) * time.Second
}

func parseMaxAge(cacheControl string) time.Duration {
	for _, part := range strings.Split(cacheControl, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "max-age=") {
			seconds, err := parsePositiveInt(strings.TrimPrefix(part, "max-age="))
			if err == nil {
				return time.Duration(seconds) * time.Second
			}
		}
	}
	return 0
}

func parsePositiveInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (c *ProxyCache) Invalidate(route *Route, r *http.Request) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := c.key(route, r)
	if entry, ok := c.entries[key]; ok {
		c.usedSize -= entry.size
	}
	c.lru.remove(key)
	delete(c.entries, key)
}

func (c *ProxyCache) evictLocked(needed int64) {
	for c.usedSize+needed > c.maxSize && len(c.entries) > 0 {
		if len(c.lru.list) == 0 {
			break
		}
		evictKey := c.lru.list[len(c.lru.list)-1]
		if entry, ok := c.entries[evictKey]; ok {
			c.usedSize -= entry.size
		}
		delete(c.entries, evictKey)
		c.lru.remove(evictKey)
	}
}

func (c *ProxyCache) Sweep() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, entry := range c.entries {
		if entry.expired() {
			c.usedSize -= entry.size
			c.lru.remove(key)
			delete(c.entries, key)
		}
	}
}

func (c *ProxyCache) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	totalEntries := len(c.entries)
	totalSize := c.usedSize
	expired := 0
	for _, entry := range c.entries {
		if entry.expired() {
			expired++
		}
	}
	return map[string]interface{}{
		"entries":     totalEntries,
		"usedSizeMB":  float64(totalSize) / (1024 * 1024),
		"maxSizeMB":   float64(c.maxSize) / (1024 * 1024),
		"expired":     expired,
	}
}

type cachingWriter struct {
	route       *Route
	cache       *ProxyCache
	statusCode  int
	headers     http.Header
	body        *bytes.Buffer
	writer      io.Writer
	snapshot    *http.Response
	committed   bool
	logger      *log.Logger
}

func newCachingWriter(route *Route, cache *ProxyCache, w http.ResponseWriter, logger *log.Logger) *cachingWriter {
	return &cachingWriter{
		route:   route,
		cache:   cache,
		headers: make(http.Header),
		body:    new(bytes.Buffer),
		writer:  w,
		logger:  logger,
	}
}

func (cw *cachingWriter) Header() http.Header {
	if cw.committed {
		return cw.headers
	}
	return cw.headers
}

func (cw *cachingWriter) WriteHeader(status int) {
	if cw.committed {
		return
	}
	cw.statusCode = status
	cw.committed = true
}

func (cw *cachingWriter) Write(p []byte) (int, error) {
	if !cw.committed {
		cw.WriteHeader(http.StatusOK)
	}
	return cw.body.Write(p)
}

func (cw *cachingWriter) writeTo(w http.ResponseWriter) {
	if !cw.committed {
		cw.WriteHeader(http.StatusOK)
	}
	for k, v := range cw.headers {
		for _, vv := range v {
			w.Header().Add(k, vv)
		}
	}
	w.WriteHeader(cw.statusCode)
	_, _ = w.Write(cw.body.Bytes())
}

func (cw *cachingWriter) commit() {
	if !cw.committed {
		cw.WriteHeader(http.StatusOK)
	}
	if cw.statusCode >= 200 && cw.statusCode < 400 {
		resp := &http.Response{
			StatusCode: cw.statusCode,
			Header:     cw.headers,
		}
		cw.cache.Store(cw.route, nil, resp, cw.body.Bytes())
	}
}

func (c *ProxyCache) WrapTransport(route *Route, transport http.RoundTripper) http.RoundTripper {
	return &cachingTransport{
		route:    route,
		cache:    c,
		base:     transport,
		logger:   c.logger,
	}
}

type cachingTransport struct {
	route  *Route
	cache  *ProxyCache
	base   http.RoundTripper
	logger *log.Logger
}

func (ct *cachingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		return ct.base.RoundTrip(req)
	}
	cw := newCachingWriter(ct.route, ct.cache, nil, ct.logger)
	if err := copyResponse(cw, req, ct.base); err != nil {
		return nil, err
	}
	cw.commit()
	resp := &http.Response{
		StatusCode: cw.statusCode,
		Header:     cw.headers,
		Body:       io.NopCloser(bytes.NewReader(cw.body.Bytes())),
	}
	return resp, nil
}

func copyResponse(dst *cachingWriter, req *http.Request, rt http.RoundTripper) error {
	resp, err := rt.RoundTrip(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		for _, vv := range v {
			dst.Header().Add(k, vv)
		}
	}
	dst.Header().Set("X-Cache", "MISS")
	dst.WriteHeader(resp.StatusCode)
	_, err = io.Copy(dst, resp.Body)
	return err
}
