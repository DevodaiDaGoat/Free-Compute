package scheduler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/DevodaiDaGoat/Free-Compute/apps/scheduler/internal/database"
	"github.com/DevodaiDaGoat/Free-Compute/apps/scheduler/internal/host"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// Galaxy is the core scheduling algorithm that matches VM requests to hosts.
type Galaxy struct {
	db          *database.DB
	hostManager *host.Manager
	ranker      *Ranker
}

func NewGalaxy(db *database.DB, hostManager *host.Manager) *Galaxy {
	return &Galaxy{
		db:          db,
		hostManager: hostManager,
		ranker:      NewRanker(),
	}
}

type ScheduleRequest struct {
	UserID    string `json:"user_id"`
	CPUCores  int    `json:"cpu_cores"`
	RAMGB     int    `json:"ram_gb"`
	StorageGB int    `json:"storage_gb"`
	GPUVRAM   int    `json:"gpu_vram,omitempty"`
}

// HandleScheduleRequest processes a VM scheduling request from the gateway.
func (g *Galaxy) HandleScheduleRequest(w http.ResponseWriter, r *http.Request) {
	var req ScheduleRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	// Validate resource bounds
	if req.CPUCores < 1 || req.CPUCores > 16 ||
		req.RAMGB < 1 || req.RAMGB > 64 ||
		req.StorageGB < 10 || req.StorageGB > 500 {
		http.Error(w, `{"error":"resource limits exceeded"}`, http.StatusBadRequest)
		return
	}

	// Find best host using Galaxy algorithm
	selectedHost, err := g.selectHost(r.Context(), req)
	if err != nil {
		log.Error().Err(err).Msg("host selection failed")
		http.Error(w, `{"error":"no available hosts"}`, http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"host_id": selectedHost.ID,
		"status":  "scheduled",
	})
}

// HandleQueueStatus returns a user's queue position.
func (g *Galaxy) HandleQueueStatus(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	if userID == "" {
		http.Error(w, `{"error":"user_id required"}`, http.StatusBadRequest)
		return
	}

	entry, err := g.db.GetQueuePosition(r.Context(), userID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"in_queue": false,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"in_queue":       true,
		"position":       entry.Position,
		"estimated_wait": entry.EstimatedWaitSeconds,
	})
}

// ProcessQueue is a background worker that processes the scheduling queue.
func (g *Galaxy) ProcessQueue(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			g.processNextInQueue(ctx)
		}
	}
}

func (g *Galaxy) selectHost(ctx context.Context, req ScheduleRequest) (*database.Host, error) {
	hosts, err := g.db.GetOnlineHosts(ctx)
	if err != nil {
		return nil, err
	}

	// Score and rank hosts based on available resources and proximity
	ranked := g.ranker.RankHosts(hosts, req)
	if len(ranked) == 0 {
		return nil, ErrNoAvailableHosts
	}

	return &ranked[0], nil
}

func (g *Galaxy) processNextInQueue(ctx context.Context) {
	// TODO: Dequeue next entry and attempt scheduling
	_ = ctx
}

var ErrNoAvailableHosts = &SchedulerError{Message: "no available hosts match the resource requirements"}

type SchedulerError struct {
	Message string
}

func (e *SchedulerError) Error() string {
	return e.Message
}
