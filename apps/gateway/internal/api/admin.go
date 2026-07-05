package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ListHosts returns all registered hosts (admin only).
func ListHosts(w http.ResponseWriter, r *http.Request) {
	// Auth + admin role already verified by middleware chain
	// TODO: Query scheduler for host list
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"hosts": []interface{}{},
	})
}

// RestartHost sends a restart command to a host (admin only).
func RestartHost(w http.ResponseWriter, r *http.Request) {
	hostID := chi.URLParam(r, "id")

	// SECURITY: Validate UUID to prevent injection
	if _, err := uuid.Parse(hostID); err != nil {
		respondError(w, http.StatusBadRequest, "invalid host ID")
		return
	}

	// TODO: Send restart command to host agent via scheduler
	respondJSON(w, http.StatusOK, map[string]string{"message": "host restart initiated"})
}
