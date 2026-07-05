package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DevodaiDaGoat/Free-Compute/apps/scheduler/internal/config"
	"github.com/DevodaiDaGoat/Free-Compute/apps/scheduler/internal/database"
	"github.com/DevodaiDaGoat/Free-Compute/apps/scheduler/internal/host"
	"github.com/DevodaiDaGoat/Free-Compute/apps/scheduler/internal/scheduler"
)

func main() {
	cfg := config.Load()

	db := database.New(cfg.DatabaseURL)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := db.Connect(ctx); err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	hostMgr := host.NewManager(db, cfg.HeartbeatTTL)
	galaxy := scheduler.NewGalaxy(db, hostMgr)

	// Background workers.
	healthChecker := host.NewHealthChecker(db, cfg.HeartbeatTTL)
	go healthChecker.Run(ctx)

	queueMgr := scheduler.NewQueueManager(db, galaxy, cfg.QueuePollInterval)
	go queueMgr.Run(ctx)

	// HTTP health endpoint.
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","service":"scheduler"}`)
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Graceful shutdown.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down scheduler...")
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("http server shutdown error: %v", err)
		}
	}()

	log.Printf("scheduler listening on :%d", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("http server error: %v", err)
	}
}
