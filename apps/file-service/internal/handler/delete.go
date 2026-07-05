package handler

import (
	"encoding/json"
	"net/http"

	"github.com/DevodaiDaGoat/Free-Compute/apps/file-service/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type DeleteHandler struct {
	cfg *config.Config
}

func NewDeleteHandler(cfg *config.Config) *DeleteHandler {
	return &DeleteHandler{cfg: cfg}
}

func (h *DeleteHandler) Handle(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "id")

	// SECURITY: Validate UUID format
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
	// TODO: Delete file from storage and remove DB record

	log.Info().Str("file_id", fileID).Str("user_id", userID).Msg("file deleted")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "file deleted",
		"file_id": fileID,
	})
}
