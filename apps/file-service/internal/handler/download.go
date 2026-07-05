package handler

import (
	"net/http"

	"github.com/DevodaiDaGoat/Free-Compute/apps/file-service/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type DownloadHandler struct {
	cfg *config.Config
}

func NewDownloadHandler(cfg *config.Config) *DownloadHandler {
	return &DownloadHandler{cfg: cfg}
}

func (h *DownloadHandler) Handle(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "id")

	// SECURITY: Validate UUID format to prevent path traversal
	if _, err := uuid.Parse(fileID); err != nil {
		http.Error(w, `{"error":"invalid file ID"}`, http.StatusBadRequest)
		return
	}

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, `{"error":"X-User-ID header required"}`, http.StatusBadRequest)
		return
	}

	// TODO: Verify file ownership (file.user_id == userID)
	// TODO: Retrieve file from storage (local or S3)
	// TODO: Set proper headers:

	// SECURITY: Force download — prevent browsers from executing uploaded content inline
	w.Header().Set("Content-Disposition", "attachment; filename=\""+fileID+"\"")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'")

	// TODO: Stream file content to response
	_ = fileID
	http.Error(w, `{"error":"not implemented"}`, http.StatusNotImplemented)
}
