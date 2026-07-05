package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/DevodaiDaGoat/Free-Compute/apps/gateway/internal/utils"
)

// VMService defines the VM lifecycle operations the handler depends on.
type VMService interface {
	List(userID string) ([]any, error)
	Launch(userID string, spec any) (vmID string, err error)
	Pause(vmID string) error
	Resume(vmID string) error
	Stop(vmID string) error
	Delete(vmID string) error
}

// VMHandler exposes VM management endpoints.
type VMHandler struct {
	svc VMService
}

// NewVMHandler constructs a VMHandler with the given service dependency.
func NewVMHandler(svc VMService) *VMHandler {
	return &VMHandler{svc: svc}
}

// Routes returns a chi router group mounting the VM endpoints.
func (h *VMHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Post("/launch", h.launch)
	r.Post("/{id}/pause", h.pause)
	r.Post("/{id}/resume", h.resume)
	r.Post("/{id}/stop", h.stop)
	r.Delete("/{id}", h.delete)
	return r
}

func (h *VMHandler) list(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"vms": []any{},
	})
}

func (h *VMHandler) launch(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, http.StatusAccepted, map[string]any{
		"vm_id": "vm_placeholder",
		"state": "provisioning",
	})
}

func (h *VMHandler) pause(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	utils.WriteJSON(w, http.StatusOK, map[string]any{"vm_id": id, "state": "paused"})
}

func (h *VMHandler) resume(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	utils.WriteJSON(w, http.StatusOK, map[string]any{"vm_id": id, "state": "running"})
}

func (h *VMHandler) stop(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	utils.WriteJSON(w, http.StatusOK, map[string]any{"vm_id": id, "state": "stopped"})
}

func (h *VMHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	utils.WriteJSON(w, http.StatusOK, map[string]any{"vm_id": id, "deleted": true})
}
