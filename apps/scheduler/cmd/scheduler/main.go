package main

import (
	"context"
	"crypto/subtle"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/freecompute/free-compute/apps/scheduler/internal/config"
	"github.com/freecompute/free-compute/apps/scheduler/internal/host"
	"github.com/freecompute/free-compute/apps/scheduler/internal/scheduler"
)

// maxJSONBody is the per-request body cap for scheduler HTTP handlers.
// json.NewDecoder(r.Body).Decode does not bound the input by itself — an
// unbounded body lets a caller OOM the process with a giant payload, and
// arbitrarily-deep nesting can blow the stack. 1 MiB is generous for a
// heartbeat / register / enqueue payload.
const maxJSONBody = 1 << 20

func main() {
	logger := log.New(os.Stdout, "[scheduler] ", log.LstdFlags|log.Lshortfile)

	cfg := config.Load()
	logger.Printf("starting scheduler on %s (interval: %v)", cfg.Addr, cfg.ScheduleInterval)
	if cfg.AuthToken == "" {
		logger.Print("WARNING: FREECOMPUTE_SCHEDULER_AUTH_TOKEN is empty; refusing to serve non-health endpoints")
	}

	hostMgr := host.NewManager(logger)
	sched := scheduler.New(cfg, hostMgr, logger)

	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"scheduler"}`))
	})

	auth := requireAuth(cfg.AuthToken)
	body := boundedBody

	mux.Handle("/api/queue", auth(body(http.HandlerFunc(sched.HandleQueue))))
	mux.Handle("/api/queue/", auth(body(http.HandlerFunc(sched.HandleQueueItem))))
	mux.Handle("/api/hosts/register", auth(body(http.HandlerFunc(hostMgr.HandleRegister))))
	mux.Handle("/api/hosts/heartbeat", auth(body(http.HandlerFunc(hostMgr.HandleHeartbeat))))
	mux.Handle("/api/hosts", auth(http.HandlerFunc(hostMgr.HandleListHosts)))
	mux.Handle("/api/allocations", auth(http.HandlerFunc(sched.HandleAllocations)))
	mux.Handle("/api/schedule", auth(http.HandlerFunc(sched.HandleSchedule)))

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

// requireAuth returns middleware that rejects any request whose
// Authorization header does not carry the configured Bearer token. If the
// scheduler was started without a token, every non-health request is
// refused (fail-closed) rather than the previous fail-open behavior where
// the token was loaded from config but never checked.
func requireAuth(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token == "" {
				http.Error(w, `{"error":"scheduler auth token not configured"}`, http.StatusServiceUnavailable)
				return
			}
			hdr := r.Header.Get("Authorization")
			const prefix = "Bearer "
			if !strings.HasPrefix(hdr, prefix) {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			presented := strings.TrimPrefix(hdr, prefix)
			if subtle.ConstantTimeCompare([]byte(presented), []byte(token)) != 1 {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func boundedBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
		}
		next.ServeHTTP(w, r)
	})
}
