package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/freecompute/free-compute/apps/file-service/internal/models"
)

// MaxUploadSize caps a single upload to 10 GiB. S3Storage.Upload uses this to
// bound io.ReadAll so a malicious client can't OOM the process.
const MaxUploadSize int64 = 10 * 1024 * 1024 * 1024

var ErrPathTraversal = errors.New("path traversal detected")

// safeUserPath resolves basePath/userID/filePath and guarantees the result
// stays inside the user's directory (rejects ../ escapes and absolute paths).
func safeUserPath(basePath, userID, filePath string) (string, error) {
	if userID == "" {
		return "", errors.New("empty userID")
	}
	// Reject absolute paths outright — filepath.Join would silently accept them
	// on some platforms and Clean can't recover the intended prefix.
	if filepath.IsAbs(filePath) {
		return "", ErrPathTraversal
	}
	root := filepath.Clean(filepath.Join(basePath, userID))
	full := filepath.Clean(filepath.Join(root, filePath))
	if full != root && !strings.HasPrefix(full, root+string(os.PathSeparator)) {
		return "", ErrPathTraversal
	}
	return full, nil
}

type Storage interface {
	Upload(userID, filePath, mimeType string, reader io.Reader) (*models.FileInfo, error)
	Download(userID, filePath string) (io.ReadCloser, *models.FileInfo, error)
	Delete(userID, filePath string) error
	List(userID, prefix string, page, pageSize int) ([]*models.FileInfo, int, error)
	Info(userID, filePath string) (*models.FileInfo, error)
}

type LocalStorage struct {
	basePath string
	mu       sync.RWMutex
	index    map[string][]*models.FileInfo
}

func NewLocalStorage(basePath string) (*LocalStorage, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("create base path: %w", err)
	}
	return &LocalStorage{
		basePath: basePath,
		index:    make(map[string][]*models.FileInfo),
	}, nil
}

func (s *LocalStorage) Upload(userID, filePath, mimeType string, reader io.Reader) (*models.FileInfo, error) {
	fullPath, err := safeUserPath(s.basePath, userID, filePath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return nil, fmt.Errorf("create dir: %w", err)
	}

	f, err := os.Create(fullPath)
	if err != nil {
		return nil, fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	hasher := sha256.New()
	// Cap the copy at MaxUploadSize so a chunked-encoding client can't fill
	// the disk. The handler layer also wraps r.Body in http.MaxBytesReader.
	tee := io.TeeReader(io.LimitReader(reader, MaxUploadSize), hasher)
	written, err := io.Copy(f, tee)
	if err != nil {
		os.Remove(fullPath)
		return nil, fmt.Errorf("write file: %w", err)
	}

	checksum := hex.EncodeToString(hasher.Sum(nil))
	now := time.Now()
	info := &models.FileInfo{
		ID:        fmt.Sprintf("file_%x", now.UnixNano()),
		Name:      filepath.Base(filePath),
		Path:      filePath,
		Size:      written,
		MimeType:  mimeType,
		UserID:    userID,
		Checksum:  checksum,
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.mu.Lock()
	s.index[userID] = append(s.index[userID], info)
	s.mu.Unlock()

	return info, nil
}

func (s *LocalStorage) Download(userID, filePath string) (io.ReadCloser, *models.FileInfo, error) {
	fullPath, err := safeUserPath(s.basePath, userID, filePath)
	if err != nil {
		return nil, nil, err
	}
	stat, err := os.Stat(fullPath)
	if err != nil {
		return nil, nil, fmt.Errorf("file not found: %w", err)
	}

	f, err := os.Open(fullPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open file: %w", err)
	}

	info := &models.FileInfo{
		Name:      filepath.Base(filePath),
		Path:      filePath,
		Size:      stat.Size(),
		UpdatedAt: stat.ModTime(),
	}

	return f, info, nil
}

func (s *LocalStorage) Delete(userID, filePath string) error {
	fullPath, err := safeUserPath(s.basePath, userID, filePath)
	if err != nil {
		return err
	}
	if err := os.Remove(fullPath); err != nil {
		return fmt.Errorf("delete file: %w", err)
	}

	s.mu.Lock()
	files := s.index[userID]
	for i, f := range files {
		if f.Path == filePath {
			s.index[userID] = append(files[:i], files[i+1:]...)
			break
		}
	}
	s.mu.Unlock()

	return nil
}

func (s *LocalStorage) List(userID, prefix string, page, pageSize int) ([]*models.FileInfo, int, error) {
	userDir := filepath.Join(s.basePath, userID)
	var result []*models.FileInfo

	filepath.Walk(userDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		relPath, _ := filepath.Rel(userDir, path)
		if relPath == "." {
			return nil
		}
		if prefix != "" && !strings.HasPrefix(relPath, prefix) {
			return nil
		}

		result = append(result, &models.FileInfo{
			Name:      info.Name(),
			Path:      relPath,
			Size:      info.Size(),
			IsDir:     info.IsDir(),
			UpdatedAt: info.ModTime(),
		})
		return nil
	})

	total := len(result)
	start := (page - 1) * pageSize
	if start >= total {
		return []*models.FileInfo{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	return result[start:end], total, nil
}

func (s *LocalStorage) Info(userID, filePath string) (*models.FileInfo, error) {
	fullPath, err := safeUserPath(s.basePath, userID, filePath)
	if err != nil {
		return nil, err
	}
	stat, err := os.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	return &models.FileInfo{
		Name:      stat.Name(),
		Path:      filePath,
		Size:      stat.Size(),
		IsDir:     stat.IsDir(),
		UpdatedAt: stat.ModTime(),
	}, nil
}

type S3Storage struct {
	bucket string
	region string
	endpoint string
	accessKey string
	secretKey string
	client   *s3Client
}

type s3Client struct{}

// errS3NotImplemented is returned by every S3Storage method. The S3 code path
// is a scaffold — Upload would previously hash the input and return
// success without persisting anything, silently discarding user data. Making
// every method (including construction) return an error keeps deployments
// from footgunning themselves by setting FREECOMPUTE_FILESERVICE_STORAGE=s3
// before the S3 client is actually implemented.
var errS3NotImplemented = fmt.Errorf("S3 storage backend is not yet implemented; set FREECOMPUTE_FILESERVICE_STORAGE=local")

func NewS3Storage(bucket, region, endpoint, accessKey, secretKey string) (*S3Storage, error) {
	return nil, errS3NotImplemented
}

func (s *S3Storage) Upload(userID, filePath, mimeType string, reader io.Reader) (*models.FileInfo, error) {
	return nil, errS3NotImplemented
}

func (s *S3Storage) Download(userID, filePath string) (io.ReadCloser, *models.FileInfo, error) {
	return nil, nil, errS3NotImplemented
}

func (s *S3Storage) Delete(userID, filePath string) error {
	return errS3NotImplemented
}

func (s *S3Storage) List(userID, prefix string, page, pageSize int) ([]*models.FileInfo, int, error) {
	return nil, 0, errS3NotImplemented
}

func (s *S3Storage) Info(userID, filePath string) (*models.FileInfo, error) {
	return nil, errS3NotImplemented
}
