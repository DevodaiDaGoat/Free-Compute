package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/DevodaiDaGoat/Free-Compute/host-agent/internal/agent"
	"github.com/DevodaiDaGoat/Free-Compute/host-agent/internal/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	cfg := config.Load()

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	a, err := agent.New(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize agent")
	}

	// Register with the scheduler (requires admin approval)
	if err := a.Register(ctx); err != nil {
		log.Fatal().Err(err).Msg("registration failed")
	}

	// Start agent loop
	go a.Run(ctx)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutting down host agent")
	cancel()
	a.Shutdown()
}
