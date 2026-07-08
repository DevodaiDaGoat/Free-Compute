package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/freecompute/free-compute/apps/scheduler/internal/config"
	"github.com/freecompute/free-compute/apps/scheduler/internal/host"
)

type SessionType string

const (
	SessionDesktop  SessionType = "desktop"
	SessionGaming   SessionType = "gaming"
	SessionDev      SessionType = "development"
	SessionRemote   SessionType = "remote-support"
)

type Priority int

const (
	PriorityLow    Priority = 1
	PriorityNormal Priority = 2
	PriorityHigh   Priority = 3
	PriorityUrgent Priority = 4
)

type QueueItem struct {
	ID            string      `json:"id"`
	UserID        string      `json:"userId"`
	SessionType   SessionType `json:"sessionType"`
	ResourceClass string      `json:"resourceClass"`
	Priority      Priority    `json:"priority"`
	CPUCores      int         `json:"cpuCores"`
	RAMGB         int         `json:"ramGb"`
	GPURequired   bool        `json:"gpuRequired"`
	GPUVramGB     int         `json:"gpuVramGb"`
	LatencyBudgetMs int       `json:"latencyBudgetMs"`
	Status        string      `json:"status"`
	Position      int         `json:"position"`
	EstimatedWait int         `json:"estimatedWaitSeconds"`
	CreatedAt     time.Time   `json:"createdAt"`
	AssignedHost  string      `json:"assignedHost,omitempty"`
}

type Allocation struct {
	ID        string    `json:"id"`
	QueueID   string    `json:"queueId"`
	UserID    string    `json:"userId"`
	HostID    string    `json:"hostId"`
	CPUCores  int       `json:"cpuCores"`
	RAMGB     int       `json:"ramGb"`
	AllocatedAt time.Time `json:"allocatedAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type Scheduler struct {
	cfg      *config.Config
	hostMgr  *host.Manager
	logger   *log.Logger
	mu       sync.RWMutex
	queue    []*QueueItem
	nextID   int
	allocations map[string]*Allocation
}

func New(cfg *config.Config, hostMgr *host.Manager, logger *log.Logger) *Scheduler {
	if logger == nil {
		logger = log.Default()
	}
	return &Scheduler{
		cfg:         cfg,
		hostMgr:     hostMgr,
		logger:      logger,
		queue:       make([]*QueueItem, 0),
		nextID:      1,
		allocations: make(map[string]*Allocation),
	}
}

func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.ScheduleInterval)
	defer ticker.Stop()

	s.logger.Printf("scheduler loop started (interval: %v)", s.cfg.ScheduleInterval)
	for {
		select {
		case <-ctx.Done():
			s.logger.Print("scheduler loop stopped")
			return
		case <-ticker.C:
			s.scheduleCycle()
		}
	}
}

func (s *Scheduler) scheduleCycle() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.queue) == 0 {
		return
	}

	hosts := s.hostMgr.GetAvailable()
	if len(hosts) == 0 {
		s.logger.Print("no available hosts for scheduling")
		return
	}

	for _, item := range s.queue {
		if item.Status != "queued" {
			continue
		}

		ranked := s.rankHosts(item, hosts)
		if len(ranked) == 0 {
			continue
		}

		best := ranked[0]
		alloc := &Allocation{
			ID:          item.ID,
			QueueID:     item.ID,
			UserID:      item.UserID,
			HostID:      best.ID,
			CPUCores:    item.CPUCores,
			RAMGB:       item.RAMGB,
			AllocatedAt: time.Now(),
			ExpiresAt:   time.Now().Add(s.cfg.DefaultTTL),
		}

		s.allocations[item.ID] = alloc
		item.Status = "allocated"
		item.AssignedHost = best.ID
		s.hostMgr.Allocate(best.ID, item.CPUCores, item.RAMGB)

		s.logger.Printf("allocated %s (user=%s) to host %s", item.ID[:8], item.UserID, best.ID[:8])
	}

	s.cleanupExpired()
}

func (s *Scheduler) cleanupExpired() {
	now := time.Now()
	for id, alloc := range s.allocations {
		if now.After(alloc.ExpiresAt) {
			s.hostMgr.Release(alloc.HostID, alloc.CPUCores, alloc.RAMGB)
			delete(s.allocations, id)
			s.logger.Printf("expired allocation %s on host %s", id[:8], alloc.HostID[:8])
		}
	}
}

func (s *Scheduler) HandleQueue(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		s.enqueue(w, r)
	case "GET":
		s.listQueue(w, r)
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (s *Scheduler) HandleQueueItem(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/queue/"):]
	if id == "" {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}

	switch r.Method {
	case "GET":
		s.getQueueItem(w, id)
	case "DELETE":
		s.dequeue(w, id)
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (s *Scheduler) HandleAllocations(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	allocs := make([]*Allocation, 0, len(s.allocations))
	for _, a := range s.allocations {
		allocs = append(allocs, a)
	}
	s.mu.RUnlock()

	writeJSON(w, http.StatusOK, map[string]any{"allocations": allocs})
}

func (s *Scheduler) HandleSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	s.scheduleCycle()
	writeJSON(w, http.StatusOK, map[string]string{"status": "scheduled"})
}

func (s *Scheduler) enqueue(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID        string      `json:"userId"`
		SessionType   SessionType `json:"sessionType"`
		ResourceClass string      `json:"resourceClass"`
		Priority      Priority    `json:"priority"`
		CPUCores      int         `json:"cpuCores"`
		RAMGB         int         `json:"ramGb"`
		GPURequired   bool        `json:"gpuRequired"`
		GPUVramGB     int         `json:"gpuVramGb"`
		LatencyBudgetMs int       `json:"latencyBudgetMs"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if req.UserID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "userId required"})
		return
	}
	if req.CPUCores < 1 {
		req.CPUCores = 2
	}
	if req.RAMGB < 1 {
		req.RAMGB = 4
	}
	if req.Priority < 1 {
		req.Priority = PriorityNormal
	}

	s.mu.Lock()
	if len(s.queue) >= s.cfg.MaxQueueSize {
		s.mu.Unlock()
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "queue full"})
		return
	}

	id := fmt.Sprintf("req_%d_%x", s.nextID, time.Now().UnixNano())
	s.nextID++

	item := &QueueItem{
		ID:             id,
		UserID:         req.UserID,
		SessionType:    req.SessionType,
		ResourceClass:  req.ResourceClass,
		Priority:       req.Priority,
		CPUCores:       req.CPUCores,
		RAMGB:          req.RAMGB,
		GPURequired:    req.GPURequired,
		GPUVramGB:      req.GPUVramGB,
		LatencyBudgetMs: req.LatencyBudgetMs,
		Status:         "queued",
		Position:       len(s.queue) + 1,
		CreatedAt:      time.Now(),
	}

	s.queue = append(s.queue, item)
	s.mu.Unlock()

	writeJSON(w, http.StatusCreated, item)
}

func (s *Scheduler) listQueue(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	items := make([]*QueueItem, len(s.queue))
	copy(items, s.queue)
	s.mu.RUnlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"queue": items,
		"count": len(items),
	})
}

func (s *Scheduler) getQueueItem(w http.ResponseWriter, id string) {
	s.mu.RLock()
	for _, item := range s.queue {
		if item.ID == id {
			s.mu.RUnlock()
			writeJSON(w, http.StatusOK, item)
			return
		}
	}
	s.mu.RUnlock()
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}

func (s *Scheduler) dequeue(w http.ResponseWriter, id string) {
	s.mu.Lock()
	for i, item := range s.queue {
		if item.ID == id {
			s.queue = append(s.queue[:i], s.queue[i+1:]...)
			if alloc, ok := s.allocations[id]; ok {
				s.hostMgr.Release(alloc.HostID, alloc.CPUCores, alloc.RAMGB)
				delete(s.allocations, id)
			}
			s.mu.Unlock()
			writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
			return
		}
	}
	s.mu.Unlock()
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
