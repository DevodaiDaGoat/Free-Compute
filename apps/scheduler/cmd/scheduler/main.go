package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/freecompute/free-compute/apps/scheduler/internal/config"
	"github.com/freecompute/free-compute/apps/scheduler/internal/host"
	"github.com/freecompute/free-compute/apps/scheduler/internal/scheduler"
)

func main() {
	logger := log.New(os.Stdout, "[scheduler] ", log.LstdFlags|log.Lshortfile)

	cfg := config.Load()
	logger.Printf("starting scheduler on %s (interval: %v)", cfg.Addr, cfg.ScheduleInterval)

	hostMgr := host.NewManager(logger)
	sched := scheduler.New(cfg, hostMgr, logger)

	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"scheduler"}`))
	})

	mux.HandleFunc("/api/queue", sched.HandleQueue)
	mux.HandleFunc("/api/queue/", sched.HandleQueueItem)
	mux.HandleFunc("/api/hosts/register", hostMgr.HandleRegister)
	mux.HandleFunc("/api/hosts/heartbeat", hostMgr.HandleHeartbeat)
	mux.HandleFunc("/api/hosts", hostMgr.HandleListHosts)
	mux.HandleFunc("/api/allocations", sched.HandleAllocations)
	mux.HandleFunc("/api/schedule", sched.HandleSchedule)

	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      withCORS(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
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

	go sched.Run(ctx)

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
