package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/DevodaiDaGoat/Free-Compute/apps/scheduler/internal/database"
)

// QueueManager processes the user queue, dequeuing entries and attempting allocation.
type QueueManager struct {
	db       *database.DB
	galaxy   *Galaxy
	interval time.Duration
}

// NewQueueManager creates a QueueManager.
func NewQueueManager(db *database.DB, galaxy *Galaxy, pollInterval time.Duration) *QueueManager {
	return &QueueManager{
		db:       db,
		galaxy:   galaxy,
		interval: pollInterval,
	}
}

// Run starts the queue processing loop, blocking until ctx is cancelled.
func (qm *QueueManager) Run(ctx context.Context) {
	ticker := time.NewTicker(qm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			qm.processNext(ctx)
		}
	}
}

func (qm *QueueManager) processNext(ctx context.Context) {
	entry, err := qm.db.DequeueNext(ctx)
	if err != nil {
		log.Printf("queue: dequeue error: %v", err)
		return
	}
	if entry == nil {
		return // queue is empty
	}

	// Default resource request for queued users.
	req := ResourceRequest{
		CPUCores:  2,
		RAMGB:     4,
		StorageGB: 20,
	}

	allocation, err := qm.galaxy.Schedule(ctx, req, entry.UserID)
	if err != nil {
		log.Printf("queue: allocation failed for user %s: %v", entry.UserID, err)
		// TODO: re-enqueue or notify the user.
		return
	}

	log.Printf("queue: allocated VM %s on host %s for user %s",
		allocation.VM.ID, allocation.HostID, entry.UserID)
}
