package middleware

import (
	"net/http"

	"github.com/DevodaiDaGoat/Free-Compute/apps/gateway/internal/config"
	"github.com/go-chi/cors"
)

// CORS returns a strict CORS middleware.
// SECURITY: Never use wildcard "*" with credentials.
func CORS(cfg *config.Config) func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           3600,
	})
}
