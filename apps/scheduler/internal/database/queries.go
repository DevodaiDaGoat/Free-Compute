package database

import (
	"context"
)

// SECURITY: All queries use parameterized placeholders ($1, $2, ...).
// NEVER interpolate user input into SQL strings.

// GetOnlineHosts returns all hosts that have sent a heartbeat recently.
func (db *DB) GetOnlineHosts(ctx context.Context) ([]Host, error) {
	if db.Pool == nil {
		return nil, nil
	}

	rows, err := db.Pool.Query(ctx,
		`SELECT id, name, region, cpu_cores, ram_gb, gpu_vram_gb, online, last_heartbeat, created_at
		 FROM hosts
		 WHERE online = true AND last_heartbeat > NOW() - INTERVAL '5 minutes'
		 ORDER BY last_heartbeat DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hosts []Host
	for rows.Next() {
		var h Host
		if err := rows.Scan(&h.ID, &h.Name, &h.Region, &h.CPUCores, &h.RAMGB,
			&h.GPUVramGB, &h.Online, &h.LastHeartbeat, &h.CreatedAt); err != nil {
			return nil, err
		}
		hosts = append(hosts, h)
	}
	return hosts, nil
}

// GetHostByID returns a single host by ID.
func (db *DB) GetHostByID(ctx context.Context, hostID string) (*Host, error) {
	if db.Pool == nil {
		return nil, nil
	}

	var h Host
	err := db.Pool.QueryRow(ctx,
		`SELECT id, name, region, cpu_cores, ram_gb, gpu_vram_gb, online, last_heartbeat, created_at
		 FROM hosts WHERE id = $1`, hostID).
		Scan(&h.ID, &h.Name, &h.Region, &h.CPUCores, &h.RAMGB,
			&h.GPUVramGB, &h.Online, &h.LastHeartbeat, &h.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

// GetUserVMs returns all VMs owned by a specific user.
func (db *DB) GetUserVMs(ctx context.Context, userID string) ([]VM, error) {
	if db.Pool == nil {
		return nil, nil
	}

	rows, err := db.Pool.Query(ctx,
		`SELECT id, user_id, host_id, name, state, cpu_cores, ram_gb, storage_gb, created_at, updated_at
		 FROM vms WHERE user_id = $1
		 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vms []VM
	for rows.Next() {
		var v VM
		if err := rows.Scan(&v.ID, &v.UserID, &v.HostID, &v.Name, &v.State,
			&v.CPUCores, &v.RAMGB, &v.StorageGB, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		vms = append(vms, v)
	}
	return vms, nil
}

// GetQueuePosition returns the user's position in the queue.
func (db *DB) GetQueuePosition(ctx context.Context, userID string) (*QueueEntry, error) {
	if db.Pool == nil {
		return nil, nil
	}

	var q QueueEntry
	err := db.Pool.QueryRow(ctx,
		`SELECT id, user_id, position, joined_at, estimated_wait_seconds
		 FROM queue WHERE user_id = $1`, userID).
		Scan(&q.ID, &q.UserID, &q.Position, &q.JoinedAt, &q.EstimatedWaitSeconds)
	if err != nil {
		return nil, err
	}
	return &q, nil
}

// UpdateHostHeartbeat updates a host's last heartbeat timestamp.
func (db *DB) UpdateHostHeartbeat(ctx context.Context, hostID string) error {
	if db.Pool == nil {
		return nil
	}

	_, err := db.Pool.Exec(ctx,
		`UPDATE hosts SET last_heartbeat = NOW(), online = true WHERE id = $1`, hostID)
	return err
}

// MarkHostOffline sets a host as offline.
func (db *DB) MarkHostOffline(ctx context.Context, hostID string) error {
	if db.Pool == nil {
		return nil
	}

	_, err := db.Pool.Exec(ctx,
		`UPDATE hosts SET online = false WHERE id = $1`, hostID)
	return err
}
