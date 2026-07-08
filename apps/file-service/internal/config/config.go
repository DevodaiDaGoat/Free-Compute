package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr        string
	StorageType string
	BasePath    string
	S3Bucket    string
	S3Region    string
	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	AuthToken   string
	MaxFileSize int64
	ReadTimeout time.Duration
}

func Load() *Config {
	return &Config{
		Addr:        getEnv("FREECOMPUTE_FILESERVICE_ADDR", ":8082"),
		StorageType: getEnv("FREECOMPUTE_FILESERVICE_STORAGE", "local"),
		BasePath:    getEnv("FREECOMPUTE_FILESERVICE_BASE_PATH", "/data/files"),
		S3Bucket:    getEnv("FREECOMPUTE_FILESERVICE_S3_BUCKET", ""),
		S3Region:    getEnv("FREECOMPUTE_FILESERVICE_S3_REGION", "us-east-1"),
		S3Endpoint:  getEnv("FREECOMPUTE_FILESERVICE_S3_ENDPOINT", ""),
		S3AccessKey: getEnv("FREECOMPUTE_FILESERVICE_S3_ACCESS_KEY", ""),
		S3SecretKey: getEnv("FREECOMPUTE_FILESERVICE_S3_SECRET_KEY", ""),
		AuthToken:   getEnv("FREECOMPUTE_FILESERVICE_AUTH_TOKEN", ""),
		MaxFileSize: getEnvInt("FREECOMPUTE_FILESERVICE_MAX_FILE_SIZE", 10*1024*1024*1024),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int64) int64 {
	if val := os.Getenv(key); val != "" {
		n, err := strconv.ParseInt(val, 10, 64)
		if err == nil {
			return n
		}
	}
	return fallback
}
