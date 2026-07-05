package api

import (
	"net/http"

	"github.com/DevodaiDaGoat/Free-Compute/apps/gateway/internal/middleware"
)

// QueueStatus returns the authenticated user's position in the queue.
func QueueStatus(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// TODO: Query scheduler for queue position
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"position":       0,
		"estimated_wait": 0,
	})
}

// JoinQueue adds the authenticated user to the VM queue.
func JoinQueue(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// TODO: Add to queue via scheduler
	respondJSON(w, http.StatusOK, map[string]string{"message": "joined queue"})
}

// LeaveQueue removes the authenticated user from the VM queue.
func LeaveQueue(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// TODO: Remove from queue via scheduler
	respondJSON(w, http.StatusOK, map[string]string{"message": "left queue"})
}
