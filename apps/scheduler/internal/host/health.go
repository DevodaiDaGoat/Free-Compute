package host

import (
	"context"
	"log"
	"time"

	"github.com/DevodaiDaGoat/Free-Compute/apps/scheduler/internal/database"
)

// HealthChecker periodically marks hosts as offline when their heartbeat expires.
type HealthChecker struct {
	db           *database.DB
	heartbeatTTL time.Duration
	interval     time.Duration
}

// NewHealthChecker creates a HealthChecker.
func NewHealthChecker(db *database.DB, heartbeatTTL time.Duration) *HealthChecker {
	return &HealthChecker{
		db:           db,
		heartbeatTTL: heartbeatTTL,
		interval:     heartbeatTTL / 2,
	}
}

// Run starts the health check loop, blocking until ctx is cancelled.
func (hc *HealthChecker) Run(ctx context.Context) {
	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hc.check(ctx)
		}
	}
}

func (hc *HealthChecker) check(ctx context.Context) {
	hosts, err := hc.db.ListOnlineHosts(ctx, 0) // all hosts
	if err != nil {
		log.Printf("health check: failed to list hosts: %v", err)
		return
	}

	cutoff := time.Now().UTC().Add(-hc.heartbeatTTL)
	for i := range hosts {
		if hosts[i].LastHeartbeat.Before(cutoff) && hosts[i].Online {
			hosts[i].Online = false
			if err := hc.db.UpsertHost(ctx, &hosts[i]); err != nil {
				log.Printf("health check: failed to mark host %s offline: %v", hosts[i].ID, err)
			} else {
				log.Printf("health check: host %s marked offline (last heartbeat %s)", hosts[i].ID, hosts[i].LastHeartbeat)
			}
		}
	}
}
