package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/DevodaiDaGoat/Free-Compute/apps/gateway/internal/utils"
)

// FilesService defines the file operations the handler depends on.
type FilesService interface {
	Upload(userID string, name string, content []byte) (fileID string, err error)
	Download(fileID string) (content []byte, err error)
	Delete(fileID string) error
}

// FilesHandler exposes file upload, download and delete endpoints.
type FilesHandler struct {
	svc FilesService
}

// NewFilesHandler constructs a FilesHandler with the given service dependency.
func NewFilesHandler(svc FilesService) *FilesHandler {
	return &FilesHandler{svc: svc}
}

// Routes returns a chi router group mounting the files endpoints.
func (h *FilesHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/upload", h.upload)
	r.Get("/download/{id}", h.download)
	r.Delete("/{id}", h.delete)
	return r
}

func (h *FilesHandler) upload(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, http.StatusCreated, map[string]any{
		"file_id": "file_placeholder",
	})
}

func (h *FilesHandler) download(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"file_id": id,
		"url":     "https://example.invalid/files/" + id,
	})
}

func (h *FilesHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"file_id": id,
		"deleted": true,
	})
}
