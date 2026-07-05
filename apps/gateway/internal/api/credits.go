package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/DevodaiDaGoat/Free-Compute/apps/gateway/internal/utils"
)

// CreditsService defines the credit operations the handler depends on.
type CreditsService interface {
	Balance(userID string) (int, error)
	Purchase(userID string, amount int) (newBalance int, err error)
}

// CreditsHandler exposes credit balance and purchase endpoints.
type CreditsHandler struct {
	svc CreditsService
}

// NewCreditsHandler constructs a CreditsHandler with the given service dependency.
func NewCreditsHandler(svc CreditsService) *CreditsHandler {
	return &CreditsHandler{svc: svc}
}

// Routes returns a chi router group mounting the credits endpoints.
func (h *CreditsHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.balance)
	r.Post("/purchase", h.purchase)
	return r
}

func (h *CreditsHandler) balance(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"balance": 0,
	})
}

func (h *CreditsHandler) purchase(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, http.StatusAccepted, map[string]any{
		"balance": 100,
		"status":  "pending",
	})
}
