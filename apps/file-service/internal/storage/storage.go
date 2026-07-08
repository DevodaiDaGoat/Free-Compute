package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/freecompute/free-compute/apps/file-service/internal/models"
)

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
	fullPath := filepath.Join(s.basePath, userID, filePath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return nil, fmt.Errorf("create dir: %w", err)
	}

	f, err := os.Create(fullPath)
	if err != nil {
		return nil, fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	hasher := sha256.New()
	tee := io.TeeReader(reader, hasher)
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
	fullPath := filepath.Join(s.basePath, userID, filePath)
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
	fullPath := filepath.Join(s.basePath, userID, filePath)
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
	fullPath := filepath.Join(s.basePath, userID, filePath)
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

func NewS3Storage(bucket, region, endpoint, accessKey, secretKey string) (*S3Storage, error) {
	return &S3Storage{
		bucket:    bucket,
		region:    region,
		endpoint:  endpoint,
		accessKey: accessKey,
		secretKey: secretKey,
		client:    &s3Client{},
	}, nil
}

func (s *S3Storage) Upload(userID, filePath, mimeType string, reader io.Reader) (*models.FileInfo, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read data: %w", err)
	}

	checksum := sha256.Sum256(data)
	now := time.Now()
	return &models.FileInfo{
		ID:        fmt.Sprintf("file_%x", now.UnixNano()),
		Name:      filepath.Base(filePath),
		Path:      fmt.Sprintf("%s/%s", userID, filePath),
		Size:      int64(len(data)),
		MimeType:  mimeType,
		UserID:    userID,
		Checksum:  hex.EncodeToString(checksum[:]),
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (s *S3Storage) Download(userID, filePath string) (io.ReadCloser, *models.FileInfo, error) {
	return nil, nil, fmt.Errorf("S3 download not yet implemented")
}

func (s *S3Storage) Delete(userID, filePath string) error {
	return fmt.Errorf("S3 delete not yet implemented")
}

func (s *S3Storage) List(userID, prefix string, page, pageSize int) ([]*models.FileInfo, int, error) {
	return []*models.FileInfo{}, 0, nil
}

func (s *S3Storage) Info(userID, filePath string) (*models.FileInfo, error) {
	return nil, fmt.Errorf("S3 info not yet implemented")
}
