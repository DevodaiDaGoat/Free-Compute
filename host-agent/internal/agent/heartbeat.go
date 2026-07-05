package agent

import "time"

// HeartbeatPayload is sent periodically to the scheduler to report host status.
type HeartbeatPayload struct {
	HostID    string    `json:"host_id"`
	Timestamp time.Time `json:"timestamp"`
	Metrics   struct {
		CPUUsage    float64 `json:"cpu_usage"`
		RAMUsedGB   float64 `json:"ram_used_gb"`
		RAMTotalGB  float64 `json:"ram_total_gb"`
		DiskUsedGB  float64 `json:"disk_used_gb"`
		DiskTotalGB float64 `json:"disk_total_gb"`
		ActiveVMs   int     `json:"active_vms"`
	} `json:"metrics"`
	AgentVersion string `json:"agent_version"`
}
