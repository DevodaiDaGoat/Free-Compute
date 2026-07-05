package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port              int
	DatabaseURL       string
	RedisURL          string
	GatewayURL        string
	HeartbeatTTL      time.Duration
	QueuePollInterval time.Duration
	LogLevel          string
}

func Load() *Config {
	return &Config{
		Port:              envInt("PORT", 8081),
		DatabaseURL:       env("DATABASE_URL", "postgres://localhost:5432/freecompute?sslmode=disable"),
		RedisURL:          env("REDIS_URL", "redis://localhost:6379"),
		GatewayURL:        env("GATEWAY_URL", "http://localhost:8080"),
		HeartbeatTTL:      envDuration("HEARTBEAT_TTL", 30*time.Second),
		QueuePollInterval: envDuration("QUEUE_POLL_INTERVAL", 5*time.Second),
		LogLevel:          env("LOG_LEVEL", "info"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
