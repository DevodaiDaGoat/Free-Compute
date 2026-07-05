package host

import "time"

// Metrics represents real-time resource usage reported by a host agent.
type Metrics struct {
	HostID        string    `json:"host_id"`
	CPUUsage      float64   `json:"cpu_usage"`       // 0.0 - 1.0
	RAMUsedGB     float64   `json:"ram_used_gb"`
	RAMTotalGB    float64   `json:"ram_total_gb"`
	DiskUsedGB    float64   `json:"disk_used_gb"`
	DiskTotalGB   float64   `json:"disk_total_gb"`
	GPUVRAMUsedGB float64   `json:"gpu_vram_used_gb"`
	ActiveVMs     int       `json:"active_vms"`
	Timestamp     time.Time `json:"timestamp"`
}

// AvailableCPU returns estimated free CPU cores.
func (m *Metrics) AvailableCPU(totalCores int) float64 {
	return float64(totalCores) * (1.0 - m.CPUUsage)
}

// AvailableRAM returns free RAM in GB.
func (m *Metrics) AvailableRAM() float64 {
	return m.RAMTotalGB - m.RAMUsedGB
}
