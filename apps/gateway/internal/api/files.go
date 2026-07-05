package api

import (
	"net/http"

	"github.com/DevodaiDaGoat/Free-Compute/apps/gateway/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const maxUploadSize = 100 * 1024 * 1024 // 100 MB

// UploadFile handles file upload with security validation.
func UploadFile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// SECURITY: Enforce max upload size at the HTTP level
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10 MB in memory
		respondError(w, http.StatusBadRequest, "file too large or invalid multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "file field required")
		return
	}
	defer file.Close()

	// SECURITY: Validate file size
	if header.Size > maxUploadSize {
		respondError(w, http.StatusBadRequest, "file exceeds maximum size of 100MB")
		return
	}

	// SECURITY: Detect MIME type from magic bytes, not Content-Type header
	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	mimeType := http.DetectContentType(buf[:n])

	allowedTypes := map[string]bool{
		"image/png":               true,
		"image/jpeg":              true,
		"image/gif":               true,
		"application/pdf":         true,
		"text/plain; charset=utf-8": true,
		"application/zip":         true,
	}

	if !allowedTypes[mimeType] {
		respondError(w, http.StatusBadRequest, "file type not allowed")
		return
	}

	// SECURITY: Store with UUID, not user-supplied filename
	fileID := uuid.New().String()

	// TODO: Proxy to file service with fileID, userID, mimeType
	_ = fileID
	respondJSON(w, http.StatusCreated, map[string]string{
		"file_id":  fileID,
		"filename": header.Filename,
		"size":     string(rune(header.Size)),
	})
}

// DownloadFile serves a file owned by the authenticated user.
func DownloadFile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	fileID := chi.URLParam(r, "id")
	if _, err := uuid.Parse(fileID); err != nil {
		respondError(w, http.StatusBadRequest, "invalid file ID")
		return
	}

	// TODO: Verify ownership and proxy download from file service
	// SECURITY: Set Content-Disposition to prevent inline execution
	w.Header().Set("Content-Disposition", "attachment")
	_ = userID
	respondJSON(w, http.StatusOK, map[string]string{"message": "download placeholder"})
}

// DeleteFile removes a file owned by the authenticated user.
func DeleteFile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	fileID := chi.URLParam(r, "id")
	if _, err := uuid.Parse(fileID); err != nil {
		respondError(w, http.StatusBadRequest, "invalid file ID")
		return
	}

	// TODO: Verify ownership and delete via file service
	_ = userID
	respondJSON(w, http.StatusOK, map[string]string{"message": "file deleted"})
}
