package main

import (
	"net/http"
	"os"
	"time"

	"github.com/DevodaiDaGoat/Free-Compute/apps/file-service/internal/config"
	"github.com/DevodaiDaGoat/Free-Compute/apps/file-service/internal/handler"
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

	uploadHandler := handler.NewUploadHandler(cfg)
	downloadHandler := handler.NewDownloadHandler(cfg)
	deleteHandler := handler.NewDeleteHandler(cfg)

	r := chi.NewRouter()

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Internal API — called by gateway
	r.Post("/upload", uploadHandler.Handle)
	r.Get("/download/{id}", downloadHandler.Handle)
	r.Delete("/delete/{id}", deleteHandler.Handle)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  60 * time.Second, // Longer for file uploads
		WriteTimeout: 60 * time.Second,
	}

	log.Info().Str("port", cfg.Port).Msg("file-service starting")
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal().Err(err).Msg("server failed")
	}
}
