package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/DevodaiDaGoat/Free-Compute/apps/gateway/internal/utils"
)

// QueueService defines the queue operations the handler depends on.
type QueueService interface {
	Status(userID string) (any, error)
	Join(userID string) (position int, err error)
	Leave(userID string) error
}

// QueueHandler exposes queue management endpoints.
type QueueHandler struct {
	svc QueueService
}

// NewQueueHandler constructs a QueueHandler with the given service dependency.
func NewQueueHandler(svc QueueService) *QueueHandler {
	return &QueueHandler{svc: svc}
}

// Routes returns a chi router group mounting the queue endpoints.
func (h *QueueHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/status", h.status)
	r.Post("/join", h.join)
	r.Post("/leave", h.leave)
	return r
}

func (h *QueueHandler) status(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"position":               0,
		"estimated_wait_seconds": 0,
		"in_queue":               false,
	})
}

func (h *QueueHandler) join(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"position":               1,
		"estimated_wait_seconds": 30,
	})
}

func (h *QueueHandler) leave(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"in_queue": false,
	})
}
