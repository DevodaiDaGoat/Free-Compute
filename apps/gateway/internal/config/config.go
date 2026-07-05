package config

import "os"

type Config struct {
	Port            string
	Environment     string
	JWTPublicKey    string
	AllowedOrigins  []string
	RedisURL        string
	AuthServiceURL  string
	SchedulerURL    string
	BillingURL      string
	FileServiceURL  string
}

func Load() *Config {
	return &Config{
		Port:           getEnv("PORT", "8080"),
		Environment:    getEnv("ENVIRONMENT", "development"),
		JWTPublicKey:   getEnv("JWT_PUBLIC_KEY", ""),
		AllowedOrigins: getEnvSlice("ALLOWED_ORIGINS", []string{"http://localhost:3000"}),
		RedisURL:       getEnv("REDIS_URL", "redis://localhost:6379"),
		AuthServiceURL: getEnv("AUTH_SERVICE_URL", "http://localhost:3001"),
		SchedulerURL:   getEnv("SCHEDULER_URL", "http://localhost:8081"),
		BillingURL:     getEnv("BILLING_URL", "http://localhost:3002"),
		FileServiceURL: getEnv("FILE_SERVICE_URL", "http://localhost:8082"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvSlice(key string, fallback []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	// Simple comma-separated parsing
	var result []string
	start := 0
	for i := 0; i <= len(v); i++ {
		if i == len(v) || v[i] == ',' {
			s := v[start:i]
			if s != "" {
				result = append(result, s)
			}
			start = i + 1
		}
	}
	return result
}
