package api

import (
	"net/http"

	"github.com/DevodaiDaGoat/Free-Compute/apps/gateway/internal/middleware"
)

// GetCredits returns the authenticated user's credit balance.
func GetCredits(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// TODO: Query billing service for credit balance
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"credits": 0,
	})
}

// PurchaseCredits initiates a credit purchase for the authenticated user.
func PurchaseCredits(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Amount int `json:"amount"`
	}

	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Amount < 1 || req.Amount > 10000 {
		respondError(w, http.StatusBadRequest, "amount must be between 1 and 10000")
		return
	}

	// TODO: Proxy to billing service with idempotency key
	respondJSON(w, http.StatusOK, map[string]string{"message": "purchase initiated"})
}
