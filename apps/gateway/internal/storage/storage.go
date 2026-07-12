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
	dirPath = strings.TrimPrefix(dirPath, "/")
	if _, err := s.isSafeUserPath(userID, dirPath); err != nil {
		return nil, err
	}
	s.mu.RLock()
	files := s.files[userID]
	// Copy the slice header under the lock so concurrent WriteFile/DeleteFile
	// can't mutate the underlying backing array while we iterate.
	snapshot := make([]*FileInfo, len(files))
	copy(snapshot, files)
	s.mu.RUnlock()

	// A raw HasPrefix match makes dirPath="foo" spuriously include "foobar/x".
	// Require an exact match OR that the file lives inside the directory
	// (prefix + "/"). An empty dirPath means "list everything".
	prefix := dirPath
	var result []*FileInfo
	for _, f := range snapshot {
		if prefix == "" || f.Path == prefix || strings.HasPrefix(f.Path, prefix+"/") {
			result = append(result, f)
		}
	}
	return result, nil
}

func (s *StorageManager) WriteFile(userID, filePath string, reader io.Reader, size int64, mimeType string) (*FileInfo, error) {
	filePath = strings.TrimPrefix(filePath, "/")

	// When a file is being overwritten, only the *delta* counts against quota.
	// The previous implementation charged the full new size, so a user with a
	// nearly-full disk couldn't overwrite an existing file even with an
	// identical or smaller replacement.
	var existingSize int64
	s.mu.RLock()
	for _, f := range s.files[userID] {
		if f.Path == filePath {
			existingSize = f.Size
			break
		}
	}
	s.mu.RUnlock()

	if s.quotaFn != nil {
		delta := size - existingSize
		if delta > 0 {
			if err := s.quotaFn(userID, delta); err != nil {
				return nil, ErrQuotaExceeded
			}
		}
	}

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

	// Replace any existing entry for the same path so overwrites don't accumulate
	// duplicate FileInfo records or double-count storage usage.
	var prevSize int64
	s.mu.Lock()
	files := s.files[userID]
	replaced := false
	for i, f := range files {
		if f.Path == filePath {
			prevSize = f.Size
			info.CreatedAt = f.CreatedAt
			info.ID = f.ID
			files[i] = info
			replaced = true
			break
		}
	}
	if !replaced {
		s.files[userID] = append(files, info)
	}
	s.mu.Unlock()

	if s.usageFn != nil {
		delta := written - prevSize
		if delta != 0 {
			s.usageFn(userID, delta)
		}
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

	// Stat before removing so we can decrement usage by the on-disk size.
	// Fall back to the cached FileInfo size if stat fails (e.g. concurrent
	// external delete).
	var freed int64
	if fi, statErr := os.Stat(cleanPath); statErr == nil {
		freed = fi.Size()
	}

	if err := os.Remove(cleanPath); err != nil {
		return fmt.Errorf("delete file: %w", err)
	}

	s.mu.Lock()
	files := s.files[userID]
	for i, f := range files {
		if f.Path == filePath {
			if freed == 0 {
				freed = f.Size
			}
			s.files[userID] = append(files[:i], files[i+1:]...)
			break
		}
	}
	s.mu.Unlock()

	if s.usageFn != nil && freed > 0 {
		s.usageFn(userID, -freed)
	}

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

	// r.ContentLength is -1 when the client uses chunked transfer encoding.
	// Passing a negative value to CheckStorageQuota trivially passes the quota
	// check (StorageUsed + negative < Quota), so treat unknown-length uploads
	// as the max allowed size for the pre-check. WriteFile still enforces the
	// hard maxFileSize via MaxBytesReader above.
	sizeHint := r.ContentLength
	if sizeHint < 0 {
		sizeHint = maxFileSize
	}
	if err := h.storage.CheckStorageQuota(userID, sizeHint); err != nil {
		http.Error(w, `{"error":"storage quota exceeded"}`, http.StatusRequestEntityTooLarge)
		return
	}

	mimeType := r.Header.Get("Content-Type")
	info, err := h.storage.WriteFile(userID, filePath, r.Body, sizeHint, mimeType)
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
	// Content-Disposition must be sanitized — an unescaped filename containing
	// " or \r\n can inject additional headers or break the header entirely.
	w.Header().Set("Content-Disposition", contentDispositionAttachment(info.Name))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size))
	if _, err := io.Copy(w, reader); err != nil {
		h.storage.logger.Printf("download copy error: %v", err)
	}
}

func contentDispositionAttachment(name string) string {
	const safe = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_.~"
	const hex = "0123456789ABCDEF"
	fb := make([]byte, 0, len(name))
	for i := 0; i < len(name) && i < 200; i++ {
		c := name[i]
		if c < 0x20 || c == 0x7f || c == '"' || c == '\\' {
			continue
		}
		fb = append(fb, c)
	}
	fallback := string(fb)
	if fallback == "" {
		fallback = "download"
	}
	var enc strings.Builder
	enc.Grow(len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if strings.IndexByte(safe, c) >= 0 {
			enc.WriteByte(c)
			continue
		}
		enc.WriteByte('%')
		enc.WriteByte(hex[c>>4])
		enc.WriteByte(hex[c&0x0f])
	}
	return `attachment; filename="` + fallback + `"; filename*=UTF-8''` + enc.String()
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
