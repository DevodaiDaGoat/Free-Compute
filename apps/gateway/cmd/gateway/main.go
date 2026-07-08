package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/freecompute/free-compute/apps/gateway/internal/tunnel"
)

func main() {
	_ = runtime.GOMAXPROCS(runtime.NumCPU())

	logger := log.New(os.Stdout, "gateway ", log.LstdFlags|log.LUTC|log.Lmicroseconds)

	cfg, err := tunnel.LoadConfigFromEnv()
	if err != nil {
		logger.Fatalf("config error: %v", err)
	}

	server, err := tunnel.NewServer(cfg, logger)
	if err != nil {
		logger.Fatalf("server init error: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := server.Start(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Fatalf("server stopped: %v", err)
	}
}
