package host

// Metrics holds telemetry data reported by a host agent heartbeat.
type Metrics struct {
	HostID       string  `json:"host_id"`
	CPUUsage     float64 `json:"cpu_usage"` // 0-100 %
	RAMUsedGB    float64 `json:"ram_used_gb"`
	RAMTotalGB   float64 `json:"ram_total_gb"`
	GPUUsage     float64 `json:"gpu_usage"` // 0-100 %
	GPUVramUsed  float64 `json:"gpu_vram_used_gb"`
	GPUVramTotal float64 `json:"gpu_vram_total_gb"`
	DiskUsedGB   float64 `json:"disk_used_gb"`
	DiskTotalGB  float64 `json:"disk_total_gb"`
	ActiveVMs    int     `json:"active_vms"`
}

// AvailableCPUPercent returns unused CPU percentage.
func (m *Metrics) AvailableCPUPercent() float64 {
	return 100 - m.CPUUsage
}

// AvailableRAMGB returns unused RAM in GB.
func (m *Metrics) AvailableRAMGB() float64 {
	return m.RAMTotalGB - m.RAMUsedGB
}

// AvailableGPUVramGB returns unused GPU VRAM in GB.
func (m *Metrics) AvailableGPUVramGB() float64 {
	return m.GPUVramTotal - m.GPUVramUsed
}
