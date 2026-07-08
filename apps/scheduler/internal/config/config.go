package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr             string
	ScheduleInterval time.Duration
	MaxQueueSize     int
	DefaultTTL       time.Duration
	AuthToken        string
}

func Load() *Config {
	return &Config{
		Addr:             getEnv("FREECOMPUTE_SCHEDULER_ADDR", ":8083"),
		ScheduleInterval: getDuration("FREECOMPUTE_SCHEDULER_INTERVAL", 5*time.Second),
		MaxQueueSize:     getInt("FREECOMPUTE_SCHEDULER_MAX_QUEUE", 1000),
		DefaultTTL:       getDuration("FREECOMPUTE_SCHEDULER_TTL", 30*time.Minute),
		AuthToken:        getEnv("FREECOMPUTE_SCHEDULER_AUTH_TOKEN", ""),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		n, err := strconv.Atoi(val)
		if err == nil {
			return n
		}
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		d, err := time.ParseDuration(val)
		if err == nil {
			return d
		}
	}
	return fallback
}
