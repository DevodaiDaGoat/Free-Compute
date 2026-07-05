package api

import (
	"net/http"

	"github.com/DevodaiDaGoat/Free-Compute/apps/gateway/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ListVMs returns the authenticated user's VMs.
func ListVMs(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// TODO: Query scheduler/database for user's VMs
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"vms": []interface{}{},
	})
}

// LaunchVM validates the request and starts a VM for the authenticated user.
func LaunchVM(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Name      string `json:"name"`
		CPUCores  int    `json:"cpu_cores"`
		RAMGB     int    `json:"ram_gb"`
		StorageGB int    `json:"storage_gb"`
	}

	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Input validation
	if !isValidVMName(req.Name) {
		respondError(w, http.StatusBadRequest, "name must be 1-64 alphanumeric characters, hyphens, or underscores")
		return
	}
	if req.CPUCores < 1 || req.CPUCores > 16 {
		respondError(w, http.StatusBadRequest, "cpu_cores must be between 1 and 16")
		return
	}
	if req.RAMGB < 1 || req.RAMGB > 64 {
		respondError(w, http.StatusBadRequest, "ram_gb must be between 1 and 64")
		return
	}
	if req.StorageGB < 10 || req.StorageGB > 500 {
		respondError(w, http.StatusBadRequest, "storage_gb must be between 10 and 500")
		return
	}

	// TODO: Check user credits, submit to scheduler
	respondJSON(w, http.StatusAccepted, map[string]string{
		"message": "VM launch queued",
		"vm_id":   uuid.New().String(),
	})
}

// PauseVM pauses a VM owned by the authenticated user.
func PauseVM(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	vmID := chi.URLParam(r, "id")

	if err := validateVMOwnership(userID, vmID); err != nil {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}

	// TODO: Send pause command to host agent
	respondJSON(w, http.StatusOK, map[string]string{"message": "VM paused"})
}

// ResumeVM resumes a paused VM owned by the authenticated user.
func ResumeVM(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	vmID := chi.URLParam(r, "id")

	if err := validateVMOwnership(userID, vmID); err != nil {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}

	// TODO: Send resume command to host agent
	respondJSON(w, http.StatusOK, map[string]string{"message": "VM resumed"})
}

// StopVM stops a VM owned by the authenticated user.
func StopVM(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	vmID := chi.URLParam(r, "id")

	if err := validateVMOwnership(userID, vmID); err != nil {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}

	// TODO: Send stop command to host agent
	respondJSON(w, http.StatusOK, map[string]string{"message": "VM stopped"})
}

// DeleteVM permanently deletes a stopped VM owned by the authenticated user.
func DeleteVM(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	vmID := chi.URLParam(r, "id")

	if err := validateVMOwnership(userID, vmID); err != nil {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}

	// TODO: Verify VM is stopped, then delete
	respondJSON(w, http.StatusOK, map[string]string{"message": "VM deleted"})
}

// --- Helpers ---

func isValidVMName(name string) bool {
	if len(name) < 1 || len(name) > 64 {
		return false
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

func validateVMOwnership(userID, vmID string) error {
	// SECURITY: Validate UUID format to prevent injection
	if _, err := uuid.Parse(vmID); err != nil {
		return err
	}
	// TODO: Query database to verify vm.user_id == userID
	_ = userID
	return nil
}
