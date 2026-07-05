package database

import "time"

type Host struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Region        string    `json:"region"`
	CPUCores      int       `json:"cpu_cores"`
	RAMGB         int       `json:"ram_gb"`
	GPUVramGB     int       `json:"gpu_vram_gb"`
	Online        bool      `json:"online"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	CreatedAt     time.Time `json:"created_at"`
}

type VM struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	HostID    string    `json:"host_id"`
	Name      string    `json:"name"`
	State     string    `json:"state"` // running, paused, stopped
	CPUCores  int       `json:"cpu_cores"`
	RAMGB     int       `json:"ram_gb"`
	StorageGB int       `json:"storage_gb"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type QueueEntry struct {
	ID                   string    `json:"id"`
	UserID               string    `json:"user_id"`
	Position             int       `json:"position"`
	JoinedAt             time.Time `json:"joined_at"`
	EstimatedWaitSeconds int       `json:"estimated_wait_seconds"`
}
