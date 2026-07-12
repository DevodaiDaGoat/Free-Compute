package handler

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/freecompute/free-compute/apps/file-service/internal/models"
	"github.com/freecompute/free-compute/apps/file-service/internal/storage"
)

type Handler struct {
	store     storage.Storage
	authToken string
	allowAnon bool
	logger    *log.Logger
}

func NewHandler(store storage.Storage, authToken string, logger *log.Logger) *Handler {
	if logger == nil {
		logger = log.Default()
	}
	// A misconfigured deployment (empty FREECOMPUTE_FILESERVICE_AUTH_TOKEN)
	// previously silently disabled auth entirely — any client could upload,
	// download, or delete on behalf of any userId. Now require an explicit
	// opt-in env var for anon operation; otherwise a missing token fails
	// closed at request time so the operator notices immediately.
	allowAnon := os.Getenv("FREECOMPUTE_FILESERVICE_ALLOW_ANON") == "1"
	if authToken == "" && allowAnon {
		logger.Printf("WARNING: file-service running with anonymous access — FREECOMPUTE_FILESERVICE_ALLOW_ANON=1 is set")
	} else if authToken == "" {
		logger.Printf("WARNING: FREECOMPUTE_FILESERVICE_AUTH_TOKEN is empty — all requests will be rejected. Set the token, or set FREECOMPUTE_FILESERVICE_ALLOW_ANON=1 for local dev.")
	}
	return &Handler{store: store, authToken: authToken, allowAnon: allowAnon, logger: logger}
}

func (h *Handler) authenticate(r *http.Request) (string, bool) {
	if h.authToken == "" {
		if h.allowAnon {
			return "anonymous", true
		}
		return "", false
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

	// Bound the upload at the storage layer's MaxUploadSize so a chunked or
	// unbounded body can't fill the disk / OOM the process.
	r.Body = http.MaxBytesReader(w, r.Body, storage.MaxUploadSize)
	info, err := h.store.Upload(userID, filePath, mimeType, r.Body)
	if err != nil {
		if err == storage.ErrPathTraversal {
			writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid path", Code: 400})
			return
		}
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
		if err == storage.ErrPathTraversal {
			writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid path", Code: 400})
			return
		}
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "file not found", Code: 404})
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	// Use RFC 5987 filename* form and strip control chars so a filename
	// containing " or \n can't inject additional headers.
	w.Header().Set("Content-Disposition", contentDispositionAttachment(info.Name))
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
	if _, err := io.Copy(w, reader); err != nil {
		h.logger.Printf("download copy error: %v", err)
	}
}

// contentDispositionAttachment builds a safe attachment header. Any character
// outside a conservative filename-safe set is stripped from the fallback quoted
// filename and represented via RFC 5987 filename*= for full fidelity.
func contentDispositionAttachment(name string) string {
	// Fallback: strip characters that could break the header (quotes, CR/LF,
	// backslashes) and truncate anything past a safe length.
	safe := make([]byte, 0, len(name))
	for i := 0; i < len(name) && i < 200; i++ {
		c := name[i]
		if c < 0x20 || c == 0x7f || c == '"' || c == '\\' {
			continue
		}
		safe = append(safe, c)
	}
	fallback := string(safe)
	if fallback == "" {
		fallback = "download"
	}
	// URL-encode the original name for filename*=. We hand-roll a small subset
	// of percent-encoding since we already imported io/strconv only.
	encoded := rfc5987Encode(name)
	return `attachment; filename="` + fallback + `"; filename*=UTF-8''` + encoded
}

func rfc5987Encode(s string) string {
	const hex = "0123456789ABCDEF"
	safe := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_.~"
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if strings.IndexByte(safe, c) >= 0 {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&0x0f])
	}
	return b.String()
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
