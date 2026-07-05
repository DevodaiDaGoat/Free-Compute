package host

import (
	"context"
	"log"
	"time"

	"github.com/DevodaiDaGoat/Free-Compute/apps/scheduler/internal/database"
)

// Manager handles host lifecycle operations.
type Manager struct {
	db           *database.DB
	heartbeatTTL time.Duration
}

// NewManager creates a Manager with the given database and heartbeat TTL.
func NewManager(db *database.DB, heartbeatTTL time.Duration) *Manager {
	return &Manager{db: db, heartbeatTTL: heartbeatTTL}
}

// OnlineHosts returns all hosts whose last heartbeat is within the TTL.
func (m *Manager) OnlineHosts(ctx context.Context) ([]database.Host, error) {
	return m.db.ListOnlineHosts(ctx, m.heartbeatTTL)
}

// RecordHeartbeat updates a host's last heartbeat timestamp and metrics.
func (m *Manager) RecordHeartbeat(ctx context.Context, host *database.Host) error {
	host.LastHeartbeat = time.Now().UTC()
	host.Online = true
	if err := m.db.UpsertHost(ctx, host); err != nil {
		return err
	}
	log.Printf("heartbeat received from host %s (%s)", host.ID, host.Name)
	return nil
}
