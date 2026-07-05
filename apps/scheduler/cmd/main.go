package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DevodaiDaGoat/Free-Compute/apps/scheduler/internal/config"
	"github.com/DevodaiDaGoat/Free-Compute/apps/scheduler/internal/database"
	"github.com/DevodaiDaGoat/Free-Compute/apps/scheduler/internal/host"
	"github.com/DevodaiDaGoat/Free-Compute/apps/scheduler/internal/scheduler"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	cfg := config.Load()

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	if cfg.Environment == "development" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	// Initialize database
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer db.Close()

	// Initialize components
	hostManager := host.NewManager(db)
	galaxyScheduler := scheduler.NewGalaxy(db, hostManager)

	// Start background workers
	go hostManager.RunHealthChecks(context.Background())
	go galaxyScheduler.ProcessQueue(context.Background())

	// HTTP API (internal — not exposed to public)
	r := chi.NewRouter()

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Internal API for gateway
	r.Post("/schedule", galaxyScheduler.HandleScheduleRequest)
	r.Get("/hosts", hostManager.HandleListHosts)
	r.Post("/hosts/{id}/restart", hostManager.HandleRestartHost)
	r.Get("/queue/{user_id}", galaxyScheduler.HandleQueueStatus)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		log.Info().Str("port", cfg.Port).Msg("scheduler starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
