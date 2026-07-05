// Package config provides shared configuration loading for all Go services.
// Services embed BaseConfig and extend it with service-specific fields.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// BaseConfig holds configuration common to all services.
type BaseConfig struct {
	ServiceName string
	Environment string // "development", "staging", "production"
	Port        int
	LogLevel    string

	DatabaseURL string
	RedisURL    string

	JWTSecret     string
	JWTExpiration time.Duration
}

// Load populates a BaseConfig from environment variables with sensible defaults.
func Load(serviceName string) (*BaseConfig, error) {
	cfg := &BaseConfig{
		ServiceName: serviceName,
		Environment: getEnv("ENVIRONMENT", "development"),
		Port:        getEnvInt("PORT", 8080),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    getEnv("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
	}

	expStr := getEnv("JWT_EXPIRATION", "24h")
	exp, err := time.ParseDuration(expStr)
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_EXPIRATION %q: %w", expStr, err)
	}
	cfg.JWTExpiration = exp

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *BaseConfig) validate() error {
	var missing []string
	if c.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if c.JWTSecret == "" {
		missing = append(missing, "JWT_SECRET")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}
	return nil
}

// IsProd returns true when running in production.
func (c *BaseConfig) IsProd() bool {
	return c.Environment == "production"
}

// Addr returns the listen address string (e.g. ":8080").
func (c *BaseConfig) Addr() string {
	return fmt.Sprintf(":%d", c.Port)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
