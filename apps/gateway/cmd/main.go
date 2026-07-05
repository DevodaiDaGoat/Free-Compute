package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/DevodaiDaGoat/Free-Compute/apps/gateway/internal/api"
	"github.com/DevodaiDaGoat/Free-Compute/apps/gateway/internal/config"
	"github.com/DevodaiDaGoat/Free-Compute/apps/gateway/internal/middleware"
	"github.com/DevodaiDaGoat/Free-Compute/apps/gateway/internal/utils"
	ws "github.com/DevodaiDaGoat/Free-Compute/apps/gateway/internal/websocket"
)

func main() {
	cfg := config.NewConfig()

	hub := ws.NewHub()
	stopHub := make(chan struct{})
	go hub.Run(stopHub)

	router := newRouter(cfg, hub)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Start the server in a goroutine so shutdown handling can run in main.
	go func() {
		log.Printf("gateway listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Wait for an interrupt or termination signal.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down gateway...")

	close(stopHub)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("graceful shutdown failed: %v", err)
	}
	log.Println("gateway stopped")
}

// newRouter builds the chi router with the full middleware chain and all routes.
func newRouter(cfg *config.Config, hub *ws.Hub) http.Handler {
	r := chi.NewRouter()

	// Base middleware chain: request ID, logging, recovery, CORS, rate limiting.
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(middleware.CORS(cfg.AllowedOrigins))
	r.Use(middleware.RateLimit(cfg.RateLimitRPS))

	// Health check (unauthenticated).
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		utils.WriteJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})

	// Handlers are wired with nil service dependencies for now; concrete
	// implementations will be injected as the services come online.
	authHandler := api.NewAuthHandler(nil)
	vmHandler := api.NewVMHandler(nil)
	queueHandler := api.NewQueueHandler(nil)
	creditsHandler := api.NewCreditsHandler(nil)
	filesHandler := api.NewFilesHandler(nil)

	r.Route("/v1", func(r chi.Router) {
		// Auth routes are public (no JWT required to register/login).
		r.Mount("/auth", authHandler.Routes())

		// Authenticated API routes.
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(cfg.JWTSecret))
			r.Mount("/vm", vmHandler.Routes())
			r.Mount("/queue", queueHandler.Routes())
			r.Mount("/credits", creditsHandler.Routes())
			r.Mount("/files", filesHandler.Routes())
			r.Mount("/admin", adminRoutes())
		})

		// WebSocket streaming endpoint for a given VM session.
		r.Get("/stream/{vmID}", ws.UpgradeHandler(hub, cfg.AllowedOrigins))
	})

	return r
}

// adminRoutes returns the admin router group.
func adminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/hosts", func(w http.ResponseWriter, _ *http.Request) {
		utils.WriteJSON(w, http.StatusOK, map[string]any{"hosts": []any{}})
	})
	r.Post("/hosts/{id}/restart", func(w http.ResponseWriter, req *http.Request) {
		id := chi.URLParam(req, "id")
		utils.WriteJSON(w, http.StatusAccepted, map[string]any{"host_id": id, "status": "restarting"})
	})
	return r
}
