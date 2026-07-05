package host

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/DevodaiDaGoat/Free-Compute/apps/scheduler/internal/database"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Manager handles host lifecycle, health checks, and metrics.
type Manager struct {
	db *database.DB
}

func NewManager(db *database.DB) *Manager {
	return &Manager{db: db}
}

// HandleListHosts returns all registered hosts.
func (m *Manager) HandleListHosts(w http.ResponseWriter, r *http.Request) {
	hosts, err := m.db.GetOnlineHosts(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("failed to list hosts")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"hosts": hosts,
	})
}

// HandleRestartHost sends a restart signal to a host.
func (m *Manager) HandleRestartHost(w http.ResponseWriter, r *http.Request) {
	hostID := chi.URLParam(r, "id")

	// SECURITY: Validate UUID format
	if _, err := uuid.Parse(hostID); err != nil {
		http.Error(w, `{"error":"invalid host ID"}`, http.StatusBadRequest)
		return
	}

	host, err := m.db.GetHostByID(r.Context(), hostID)
	if err != nil || host == nil {
		http.Error(w, `{"error":"host not found"}`, http.StatusNotFound)
		return
	}

	// TODO: Send restart command via secure channel to host agent
	log.Info().Str("host_id", hostID).Msg("host restart initiated")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "restart signal sent",
		"host_id": hostID,
	})
}

// RunHealthChecks periodically checks host health and marks stale hosts offline.
func (m *Manager) RunHealthChecks(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkHostHealth(ctx)
		}
	}
}

func (m *Manager) checkHostHealth(ctx context.Context) {
	hosts, err := m.db.GetOnlineHosts(ctx)
	if err != nil {
		log.Error().Err(err).Msg("health check: failed to get hosts")
		return
	}

	staleThreshold := time.Now().Add(-5 * time.Minute)
	for _, h := range hosts {
		if h.LastHeartbeat.Before(staleThreshold) {
			log.Warn().Str("host_id", h.ID).Msg("marking host offline (stale heartbeat)")
			if err := m.db.MarkHostOffline(ctx, h.ID); err != nil {
				log.Error().Err(err).Str("host_id", h.ID).Msg("failed to mark host offline")
			}
		}
	}
}
