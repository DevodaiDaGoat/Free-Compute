package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DevodaiDaGoat/Free-Compute/apps/gateway/internal/api"
	"github.com/DevodaiDaGoat/Free-Compute/apps/gateway/internal/config"
	mw "github.com/DevodaiDaGoat/Free-Compute/apps/gateway/internal/middleware"
	"github.com/DevodaiDaGoat/Free-Compute/apps/gateway/internal/websocket"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	cfg := config.Load()

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	if cfg.Environment == "development" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(mw.RequestLogger)
	r.Use(mw.SecurityHeaders)
	r.Use(mw.CORS(cfg))
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))

	// Health check (no auth, no sensitive info)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Public auth routes with aggressive rate limiting
	r.Group(func(r chi.Router) {
		r.Use(mw.AuthRateLimit())
		r.Post("/auth/register", api.Register)
		r.Post("/auth/login", api.Login)
		r.Post("/auth/verify", api.Verify)
		r.Post("/auth/logout", api.Logout)
	})

	// Authenticated routes
	r.Group(func(r chi.Router) {
		r.Use(mw.Authenticate(cfg))
		r.Use(mw.GeneralRateLimit())

		// VM management
		r.Get("/vm", api.ListVMs)
		r.Post("/vm/launch", api.LaunchVM)
		r.Post("/vm/{id}/pause", api.PauseVM)
		r.Post("/vm/{id}/resume", api.ResumeVM)
		r.Post("/vm/{id}/stop", api.StopVM)
		r.Delete("/vm/{id}", api.DeleteVM)

		// Queue
		r.Get("/queue/status", api.QueueStatus)
		r.Post("/queue/join", api.JoinQueue)
		r.Post("/queue/leave", api.LeaveQueue)

		// Credits
		r.Get("/credits", api.GetCredits)
		r.Post("/credits/purchase", api.PurchaseCredits)

		// File operations
		r.Post("/files/upload", api.UploadFile)
		r.Get("/files/{id}", api.DownloadFile)
		r.Delete("/files/{id}", api.DeleteFile)
	})

	// Admin routes — requires admin role
	r.Group(func(r chi.Router) {
		r.Use(mw.Authenticate(cfg))
		r.Use(mw.RequireRole("admin"))
		r.Use(mw.AuditLog)

		r.Get("/admin/hosts", api.ListHosts)
		r.Post("/admin/hosts/{id}/restart", api.RestartHost)
	})

	// WebSocket streaming — authenticated with connection token
	hub := websocket.NewHub()
	go hub.Run()

	r.Get("/stream/{vm_id}", func(w http.ResponseWriter, r *http.Request) {
		websocket.HandleConnection(hub, cfg, w, r)
	})

	// Debug endpoints — development only
	if cfg.Environment == "development" {
		r.Mount("/debug", chimw.Profiler())
	}

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		log.Info().Str("port", cfg.Port).Msg("gateway server starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("server forced shutdown")
	}
}
