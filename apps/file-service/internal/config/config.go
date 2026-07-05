package config

import "os"

type Config struct {
	Port         string
	Environment  string
	StorageType  string // "local" or "s3"
	StoragePath  string // Local storage path
	S3Bucket     string
	S3Region     string
	S3Endpoint   string
	MaxFileSize  int64 // bytes
}

func Load() *Config {
	return &Config{
		Port:        getEnv("PORT", "8082"),
		Environment: getEnv("ENVIRONMENT", "development"),
		StorageType: getEnv("STORAGE_TYPE", "local"),
		StoragePath: getEnv("STORAGE_PATH", "/data/files"),
		S3Bucket:    getEnv("S3_BUCKET", ""),
		S3Region:    getEnv("S3_REGION", ""),
		S3Endpoint:  getEnv("S3_ENDPOINT", ""),
		MaxFileSize: 100 * 1024 * 1024, // 100 MB
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
