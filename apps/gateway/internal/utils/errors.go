package utils

import (
	"net/http"

	"github.com/rs/zerolog/log"
)

// HandleError logs the full error internally and returns a generic message to the client.
// SECURITY: Never expose internal error details (DB errors, stack traces) to clients.
func HandleError(w http.ResponseWriter, err error, statusCode int) {
	log.Error().Err(err).Int("status", statusCode).Msg("request error")
	http.Error(w, http.StatusText(statusCode), statusCode)
}
