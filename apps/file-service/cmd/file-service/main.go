package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/freecompute/free-compute/apps/file-service/internal/config"
	"github.com/freecompute/free-compute/apps/file-service/internal/handler"
	"github.com/freecompute/free-compute/apps/file-service/internal/storage"
)

func main() {
	logger := log.New(os.Stdout, "[file-service] ", log.LstdFlags|log.Lshortfile)

	cfg := config.Load()
	logger.Printf("starting file-service on %s (storage: %s, base: %s)", cfg.Addr, cfg.StorageType, cfg.BasePath)

	var store storage.Storage
	switch cfg.StorageType {
	case "local":
		s, err := storage.NewLocalStorage(cfg.BasePath)
		if err != nil {
			logger.Fatalf("failed to init local storage: %v", err)
		}
		store = s
	case "s3":
		s, err := storage.NewS3Storage(cfg.S3Bucket, cfg.S3Region, cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey)
		if err != nil {
			logger.Fatalf("failed to init S3 storage: %v", err)
		}
		store = s
	default:
		logger.Fatalf("unsupported storage type: %s", cfg.StorageType)
	}

	h := handler.NewHandler(store, cfg.AuthToken, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", h.Health)
	mux.HandleFunc("/api/files/upload", h.Upload)
	mux.HandleFunc("/api/files/download/", h.Download)
	mux.HandleFunc("/api/files/", h.FileOps)
	mux.HandleFunc("/api/files", h.List)

	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      withCORS(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 300 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Printf("listening on %s", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	logger.Print("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
