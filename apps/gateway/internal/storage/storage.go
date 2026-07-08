package storage

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const maxFileSize = 10 * 1024 * 1024 * 1024
const maxChunkSize = 64 * 1024

// isSafeUserPath verifies that the resolved path stays within basePath/userID.
// It returns the cleaned absolute path, or an error if the path escapes the
// user's directory (e.g. via "../").
func (s *StorageManager) isSafeUserPath(userID, filePath string) (string, error) {
	cleanPath := filepath.Clean(filepath.Join(s.basePath, userID, filePath))
	prefix := filepath.Clean(filepath.Join(s.basePath, userID))
	if cleanPath != prefix && !strings.HasPrefix(cleanPath, prefix+string(os.PathSeparator)) {
		return "", fmt.Errorf("path traversal detected")
	}
	return cleanPath, nil
}

type FileInfo struct {
	ID         string    `json:"id"`
	UserID     string    `json:"userId"`
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	MimeType   string    `json:"mimeType"`
	IsDir      bool      `json:"isDir"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type StorageManager struct {
	logger    *log.Logger
	mu        sync.RWMutex
	basePath  string
	files     map[string][]*FileInfo
	quotaFn   func(userID string, additionalBytes int64) error
	usageFn   func(userID string, bytes int64)
}

func NewStorageManager(logger *log.Logger, basePath string) *StorageManager {
	if logger == nil {
		logger = log.Default()
	}
	os.MkdirAll(basePath, 0755)
	return &StorageManager{
		logger:   logger,
		basePath: basePath,
		files:    make(map[string][]*FileInfo),
	}
}

func (s *StorageManager) SetQuotaCheck(fn func(userID string, additionalBytes int64) error) {
	s.quotaFn = fn
}

func (s *StorageManager) SetUsageFunc(fn func(userID string, bytes int64)) {
	s.usageFn = fn
}

var ErrQuotaExceeded = errors.New("storage quota exceeded")

// CheckStorageQuota enforces the configured quota (if any) for an additional
// write of additionalBytes. It returns ErrQuotaExceeded when the quota is
// exceeded.
func (s *StorageManager) CheckStorageQuota(userID string, additionalBytes int64) error {
	if s.quotaFn == nil {
		return nil
	}
	if err := s.quotaFn(userID, additionalBytes); err != nil {
		return ErrQuotaExceeded
	}
	return nil
}

func (s *StorageManager) ListFiles(userID, dirPath string) ([]*FileInfo, error) {
	s.mu.RLock()
	files := s.files[userID]
	s.mu.RUnlock()

	dirPath = strings.TrimPrefix(dirPath, "/")
	if _, err := s.isSafeUserPath(userID, dirPath); err != nil {
		return nil, err
	}
	var result []*FileInfo
	for _, f := range files {
		if strings.HasPrefix(f.Path, dirPath) {
			result = append(result, f)
		}
	}
	return result, nil
}

func (s *StorageManager) WriteFile(userID, filePath string, reader io.Reader, size int64, mimeType string) (*FileInfo, error) {
	if s.quotaFn != nil {
		if err := s.quotaFn(userID, size); err != nil {
			return nil, ErrQuotaExceeded
		}
	}

	filePath = strings.TrimPrefix(filePath, "/")
	fullPath, err := s.isSafeUserPath(userID, filePath)
	if err != nil {
		return nil, err
	}
	os.MkdirAll(filepath.Dir(fullPath), 0755)

	f, err := os.Create(fullPath)
	if err != nil {
		return nil, fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	written, err := io.Copy(f, reader)
	if err != nil {
		os.Remove(fullPath)
		return nil, fmt.Errorf("write file: %w", err)
	}

	fileID := generateFileID()
	info := &FileInfo{
		ID:        fileID,
		UserID:    userID,
		Name:      filepath.Base(filePath),
		Path:      filePath,
		Size:      written,
		MimeType:  mimeType,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	s.mu.Lock()
	s.files[userID] = append(s.files[userID], info)
	s.mu.Unlock()

	if s.usageFn != nil {
		s.usageFn(userID, written)
	}

	return info, nil
}

func (s *StorageManager) ReadFile(userID, filePath string) (io.ReadCloser, *FileInfo, error) {
	filePath = strings.TrimPrefix(filePath, "/")
	cleanPath, err := s.isSafeUserPath(userID, filePath)
	if err != nil {
		return nil, nil, err
	}

	info, err := os.Stat(cleanPath)
	if err != nil {
		return nil, nil, fmt.Errorf("file not found: %w", err)
	}

	f, err := os.Open(cleanPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open file: %w", err)
	}

	fileInfo := &FileInfo{
		Name:      filepath.Base(filePath),
		Path:      filePath,
		Size:      info.Size(),
		UpdatedAt: info.ModTime(),
	}

	return f, fileInfo, nil
}

func (s *StorageManager) DeleteFile(userID, filePath string) error {
	filePath = strings.TrimPrefix(filePath, "/")
	cleanPath, err := s.isSafeUserPath(userID, filePath)
	if err != nil {
		return err
	}

	if err := os.Remove(cleanPath); err != nil {
		return fmt.Errorf("delete file: %w", err)
	}

	s.mu.Lock()
	files := s.files[userID]
	for i, f := range files {
		if f.Path == filePath {
			s.files[userID] = append(files[:i], files[i+1:]...)
			break
		}
	}
	s.mu.Unlock()

	return nil
}

type StorageHandler struct {
	storage *StorageManager
}

func NewStorageHandler(storage *StorageManager) *StorageHandler {
	return &StorageHandler{storage: storage}
}

func (h *StorageHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userId")
	if userID == "" {
		http.Error(w, `{"error":"userId required"}`, http.StatusBadRequest)
		return
	}
	dirPath := r.URL.Query().Get("path")
	files, err := h.storage.ListFiles(userID, dirPath)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	if files == nil {
		files = []*FileInfo{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": files})
}

func (h *StorageHandler) Upload(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userId")
	if userID == "" {
		http.Error(w, `{"error":"userId required"}`, http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxFileSize)
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		http.Error(w, `{"error":"path required"}`, http.StatusBadRequest)
		return
	}

	if err := h.storage.CheckStorageQuota(userID, r.ContentLength); err != nil {
		http.Error(w, `{"error":"storage quota exceeded"}`, http.StatusRequestEntityTooLarge)
		return
	}

	mimeType := r.Header.Get("Content-Type")
	info, err := h.storage.WriteFile(userID, filePath, r.Body, r.ContentLength, mimeType)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrQuotaExceeded) {
			status = http.StatusRequestEntityTooLarge
		}
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), status)
		return
	}

	writeJSON(w, http.StatusCreated, info)
}

func (h *StorageHandler) Download(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userId")
	if userID == "" {
		http.Error(w, `{"error":"userId required"}`, http.StatusBadRequest)
		return
	}
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		http.Error(w, `{"error":"path required"}`, http.StatusBadRequest)
		return
	}

	reader, info, err := h.storage.ReadFile(userID, filePath)
	if err != nil {
		http.Error(w, `{"error":"file not found"}`, http.StatusNotFound)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, info.Name))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size))
	io.Copy(w, reader)
}

func (h *StorageHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userId")
	if userID == "" {
		http.Error(w, `{"error":"userId required"}`, http.StatusBadRequest)
		return
	}
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		http.Error(w, `{"error":"path required"}`, http.StatusBadRequest)
		return
	}

	if err := h.storage.DeleteFile(userID, filePath); err != nil {
		http.Error(w, `{"error":"delete failed"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func generateFileID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "file_" + fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return "file_" + hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
