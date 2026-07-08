package handler

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/freecompute/free-compute/apps/file-service/internal/models"
	"github.com/freecompute/free-compute/apps/file-service/internal/storage"
)

type Handler struct {
	store     storage.Storage
	authToken string
	logger    *log.Logger
}

func NewHandler(store storage.Storage, authToken string, logger *log.Logger) *Handler {
	if logger == nil {
		logger = log.Default()
	}
	return &Handler{store: store, authToken: authToken, logger: logger}
}

func (h *Handler) authenticate(r *http.Request) (string, bool) {
	if h.authToken == "" {
		return "anonymous", true
	}
	token := r.Header.Get("Authorization")
	if strings.HasPrefix(token, "Bearer ") {
		token = strings.TrimPrefix(token, "Bearer ")
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(h.authToken)) == 1 {
		userID := r.URL.Query().Get("userId")
		if userID == "" {
			userID = "anonymous"
		}
		return userID, true
	}
	return "", false
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "file-service"})
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	userID, ok := h.authenticate(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized", Code: 401})
		return
	}

	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "path query param required", Code: 400})
		return
	}

	mimeType := r.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	info, err := h.store.Upload(userID, filePath, mimeType, r.Body)
	if err != nil {
		h.logger.Printf("upload error: %v", err)
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: err.Error(), Code: 500})
		return
	}

	writeJSON(w, http.StatusCreated, models.UploadResponse{File: info})
}

func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	userID, ok := h.authenticate(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized", Code: 401})
		return
	}

	filePath := strings.TrimPrefix(r.URL.Path, "/api/files/download/")
	if filePath == "" {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "file path required", Code: 400})
		return
	}

	reader, info, err := h.store.Download(userID, filePath)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "file not found", Code: 404})
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, info.Name))
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
	io.Copy(w, reader)
}

func (h *Handler) FileOps(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.authenticate(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized", Code: 401})
		return
	}

	filePath := strings.TrimPrefix(r.URL.Path, "/api/files/")
	if filePath == "" {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "file path required", Code: 400})
		return
	}

	switch r.Method {
	case "GET":
		info, err := h.store.Info(userID, filePath)
		if err != nil {
			writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "file not found", Code: 404})
			return
		}
		writeJSON(w, http.StatusOK, info)

	case "DELETE":
		if err := h.store.Delete(userID, filePath); err != nil {
			writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: err.Error(), Code: 500})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	userID, ok := h.authenticate(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized", Code: 401})
		return
	}

	prefix := r.URL.Query().Get("prefix")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	files, total, err := h.store.List(userID, prefix, page, pageSize)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: err.Error(), Code: 500})
		return
	}

	writeJSON(w, http.StatusOK, models.ListResponse{
		Files:    files,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
