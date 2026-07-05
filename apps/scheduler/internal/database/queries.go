package database

import (
	"context"
	"time"
)

// ListOnlineHosts returns all hosts that have sent a heartbeat within the TTL.
func (db *DB) ListOnlineHosts(ctx context.Context, heartbeatTTL time.Duration) ([]Host, error) {
	// TODO: query database
	_ = heartbeatTTL
	return nil, nil
}

// GetHost returns a single host by ID.
func (db *DB) GetHost(ctx context.Context, id string) (*Host, error) {
	// TODO: query database
	_ = id
	return nil, nil
}

// UpsertHost inserts or updates a host record (used during heartbeat).
func (db *DB) UpsertHost(ctx context.Context, host *Host) error {
	// TODO: upsert into database
	_ = host
	return nil
}

// ListVMsByHost returns all VMs running on a given host.
func (db *DB) ListVMsByHost(ctx context.Context, hostID string) ([]VM, error) {
	// TODO: query database
	_ = hostID
	return nil, nil
}

// GetQueueEntries returns queue entries ordered by position.
func (db *DB) GetQueueEntries(ctx context.Context) ([]QueueEntry, error) {
	// TODO: query database
	return nil, nil
}

// DequeueNext removes and returns the next queue entry.
func (db *DB) DequeueNext(ctx context.Context) (*QueueEntry, error) {
	// TODO: query database
	return nil, nil
}
