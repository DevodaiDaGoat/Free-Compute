package tunnel

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/andybalholm/brotli"
)

const (
	brotliMinSize = 1400
	gzipMinSize   = 1400
)

type compressionWriter struct {
	http.ResponseWriter
	writer    io.Writer
	encoding  string
	committed bool
}

func (c *compressionWriter) WriteHeader(status int) {
	if c.committed {
		return
	}
	c.committed = true
	c.ResponseWriter.WriteHeader(status)
}

func (c *compressionWriter) Write(p []byte) (int, error) {
	if !c.committed {
		c.WriteHeader(http.StatusOK)
	}
	if len(p) > 0 {
		if c.Header().Get("Content-Type") == "" {
			c.Header().Set("Content-Type", http.DetectContentType(p))
		}
	}
	return c.writer.Write(p)
}

func (c *compressionWriter) Header() http.Header {
	return c.ResponseWriter.Header()
}

func newCompressionWriter(w http.ResponseWriter, encoding string) *compressionWriter {
	var writer io.Writer = w
	switch encoding {
	case "br":
		writer = brotli.NewWriter(w)
	case "gzip":
		writer = gzip.NewWriter(w)
	}
	return &compressionWriter{
		ResponseWriter: w,
		writer:         writer,
		encoding:       encoding,
	}
}

func (c *compressionWriter) Flush() {
	if f, ok := c.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (c *compressionWriter) close() error {
	if closer, ok := c.writer.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (c *compressionWriter) Unwrap() http.ResponseWriter {
	return c.ResponseWriter
}

// Hijack delegates to the underlying ResponseWriter so WebSocket / raw-conn
// upgrades still work when compression is enabled. Without this, gorilla
// websocket's upgrade path fails with
// "websocket: response does not implement http.Hijacker".
func (c *compressionWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := c.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("response writer does not support hijacking")
	}
	return h.Hijack()
}

type compressionMiddleware struct {
	minSize    int
	logger     *log.Logger
	skipPaths  map[string]bool
	skipPrefix map[string]bool
}

func newCompressionMiddleware(minSize int, logger *log.Logger) *compressionMiddleware {
	if minSize <= 0 {
		minSize = brotliMinSize
	}
	if logger == nil {
		logger = log.Default()
	}
	return &compressionMiddleware{
		minSize:    minSize,
		logger:     logger,
		skipPaths:  make(map[string]bool),
		skipPrefix: make(map[string]bool),
	}
}

func (m *compressionMiddleware) SkipPath(path string) {
	m.skipPaths[path] = true
}

func (m *compressionMiddleware) SkipPrefix(prefix string) {
	m.skipPrefix[prefix] = true
}

func (m *compressionMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.shouldSkip(r) {
			next.ServeHTTP(w, r)
			return
		}
		encoding := selectEncoding(r.Header.Get("Accept-Encoding"))
		if encoding == "" {
			next.ServeHTTP(w, r)
			return
		}
		cw := newCompressionWriter(w, encoding)
		cw.Header().Set("Content-Encoding", encoding)
		defer func() {
			if err := cw.close(); err != nil && m.logger != nil {
				m.logger.Printf("compression close error: %v", err)
			}
		}()
		next.ServeHTTP(cw, r)
	})
}

func (m *compressionMiddleware) shouldSkip(r *http.Request) bool {
	if m.skipPaths[r.URL.Path] {
		return true
	}
	for prefix := range m.skipPrefix {
		if strings.HasPrefix(r.URL.Path, prefix) {
			return true
		}
	}
	return false
}

func selectEncoding(acceptEncoding string) string {
	if strings.Contains(acceptEncoding, "br") {
		return "br"
	}
	if strings.Contains(acceptEncoding, "gzip") {
		return "gzip"
	}
	return ""
}

func CompressionMiddleware(next http.Handler) http.Handler {
	return newCompressionMiddleware(brotliMinSize, nil).Handler(next)
}

func CompressionMiddlewareWithLogger(next http.Handler, logger *log.Logger) http.Handler {
	return newCompressionMiddleware(brotliMinSize, logger).Handler(next)
}

var brotliWriterPool = sync.Pool{
	New: func() any {
		w := brotli.NewWriter(io.Discard)
		return &w
	},
}

var gzipWriterPool = sync.Pool{
	New: func() any {
		w := gzip.NewWriter(io.Discard)
		return &w
	},
}

func resetBrotliWriter(w *brotli.Writer) {
	var buf [1 << 12]byte
	w.Reset(io.Discard)
	w.Write(buf[:])
	_ = w.Close()
}

func resetGzipWriter(w *gzip.Writer) {
	var zeroGzip gzip.Writer
	*w = zeroGzip
}

func CompressBytes(data []byte, encoding string) ([]byte, error) {
	var buf bytes.Buffer
	switch encoding {
	case "br":
		writer := brotli.NewWriter(&buf)
		if _, err := writer.Write(data); err != nil {
			return nil, err
		}
		if err := writer.Close(); err != nil {
			return nil, err
		}
	case "gzip":
		writer := gzip.NewWriter(&buf)
		if _, err := writer.Write(data); err != nil {
			return nil, err
		}
		if err := writer.Close(); err != nil {
			return nil, err
		}
	default:
		return data, nil
	}
	return buf.Bytes(), nil
}

func DetectCompressionSavings(original, compressed []byte) float64 {
	if len(original) == 0 {
		return 0
	}
	ratio := float64(len(compressed)) / float64(len(original))
	return (1.0 - ratio) * 100
}

func IsCompressible(contentType string) bool {
	compressible := []string{
		"application/json",
		"application/javascript",
		"application/xml",
		"application/xhtml+xml",
		"text/plain",
		"text/html",
		"text/css",
		"text/javascript",
		"text/xml",
		"image/svg+xml",
	}
	for _, ct := range compressible {
		if strings.Contains(contentType, ct) {
			return true
		}
	}
	return false
}

func NewScanningResponseWriter(w http.ResponseWriter) *scanningWriter {
	return &scanningWriter{
		ResponseWriter: w,
		buffer:         new(bytes.Buffer),
	}
}

type scanningWriter struct {
	http.ResponseWriter
	buffer *bytes.Buffer
}

func (sw *scanningWriter) Write(p []byte) (int, error) {
	if sw.buffer != nil {
		return sw.buffer.Write(p)
	}
	return 0, errors.New("writer committed")
}

func (sw *scanningWriter) commit(data []byte, encoding string, contentType string) {
	sw.buffer = nil
	// Header() returns the same map each call. Iterating it and calling Add on
	// itself duplicated every header on every commit (Cache-Control repeated
	// N times, Set-Cookie appended, etc.). Headers already set on the wrapper
	// pass through to the underlying ResponseWriter — no copy is needed.
	h := sw.ResponseWriter.Header()
	// Content-Length is stale once we potentially compressed the body.
	h.Del("Content-Length")
	if len(data) >= brotliMinSize && IsCompressible(contentType) && encoding != "" {
		compressed, err := CompressBytes(data, encoding)
		if err == nil && DetectCompressionSavings(data, compressed) > 10 {
			h.Set("Content-Encoding", encoding)
			sw.ResponseWriter.WriteHeader(http.StatusOK)
			_, _ = sw.ResponseWriter.Write(compressed)
			return
		}
	}
	sw.ResponseWriter.WriteHeader(http.StatusOK)
	_, _ = sw.ResponseWriter.Write(data)
}
