// Package logger provides a shared structured logger factory for all Go services.
package logger

import (
	"log/slog"
	"os"
	"strings"
)

// New creates a structured slog.Logger configured by level and environment.
// In production it outputs JSON; otherwise it uses a human-readable text format.
func New(level, environment string) *slog.Logger {
	var slogLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn", "warning":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: slogLevel}

	var handler slog.Handler
	if environment == "production" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

// FromConfig creates a logger from BaseConfig fields.
func FromConfig(level, environment, serviceName string) *slog.Logger {
	return New(level, environment).With("service", serviceName)
}
