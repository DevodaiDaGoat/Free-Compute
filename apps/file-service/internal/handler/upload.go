package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/DevodaiDaGoat/Free-Compute/apps/file-service/internal/config"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// SECURITY: Allowed MIME types — detected from file content (magic bytes), not headers.
var allowedMIMETypes = map[string]bool{
	"image/png":                  true,
	"image/jpeg":                 true,
	"image/gif":                  true,
	"image/webp":                 true,
	"application/pdf":            true,
	"text/plain; charset=utf-8":  true,
	"application/zip":            true,
	"application/x-tar":          true,
	"application/gzip":           true,
	"application/octet-stream":   true,
}

type UploadHandler struct {
	cfg *config.Config
}

func NewUploadHandler(cfg *config.Config) *UploadHandler {
	return &UploadHandler{cfg: cfg}
}

func (h *UploadHandler) Handle(w http.ResponseWriter, r *http.Request) {
	// SECURITY: Enforce max file size at HTTP level
	r.Body = http.MaxBytesReader(w, r.Body, h.cfg.MaxFileSize)

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, `{"error":"file too large or invalid form"}`, http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `{"error":"file field required"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, `{"error":"X-User-ID header required"}`, http.StatusBadRequest)
		return
	}

	// SECURITY: Validate file size
	if header.Size > h.cfg.MaxFileSize {
		http.Error(w, `{"error":"file exceeds maximum size"}`, http.StatusBadRequest)
		return
	}

	// SECURITY: Detect actual MIME type from magic bytes
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		http.Error(w, `{"error":"failed to read file"}`, http.StatusInternalServerError)
		return
	}

	detectedMIME := http.DetectContentType(buf[:n])
	if !allowedMIMETypes[detectedMIME] {
		http.Error(w, `{"error":"file type not allowed"}`, http.StatusBadRequest)
		return
	}

	// Reset file reader position
	if seeker, ok := file.(io.Seeker); ok {
		seeker.Seek(0, io.SeekStart)
	}

	// SECURITY: Store with UUID — never use user-supplied filename for storage path
	fileID := uuid.New().String()
	ext := sanitizeExtension(filepath.Ext(header.Filename))
	storageName := fileID + ext

	// TODO: Store file (local or S3) using storageName
	// TODO: Record metadata in database (fileID, userID, originalName, mimeType, size)
	_ = storageName

	log.Info().
		Str("file_id", fileID).
		Str("user_id", userID).
		Int64("size", header.Size).
		Str("mime", detectedMIME).
		Msg("file uploaded")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"file_id":  fileID,
		"filename": header.Filename,
		"size":     header.Size,
		"mime":     detectedMIME,
	})
}

// SECURITY: Sanitize file extension — strip path traversal and null bytes.
func sanitizeExtension(ext string) string {
	// Remove null bytes
	ext = strings.ReplaceAll(ext, "\x00", "")
	// Only allow alphanumeric extensions
	if len(ext) > 10 {
		ext = ext[:10]
	}
	for _, c := range ext[1:] { // Skip the leading dot
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return "" // Invalid extension chars — drop it
		}
	}
	return strings.ToLower(ext)
}
