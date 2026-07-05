package scheduler

import (
	"github.com/DevodaiDaGoat/Free-Compute/apps/scheduler/internal/database"
	"github.com/DevodaiDaGoat/Free-Compute/apps/scheduler/internal/host"
)

// ResourceRequest describes the resources a VM needs.
type ResourceRequest struct {
	CPUCores  int     `json:"cpu_cores"`
	RAMGB     float64 `json:"ram_gb"`
	StorageGB float64 `json:"storage_gb"`
	GPUVramGB float64 `json:"gpu_vram_gb"` // 0 = no GPU needed
}

// HostScore pairs a host with its computed suitability score.
type HostScore struct {
	Host    database.Host
	Metrics host.Metrics
	Score   float64
}

// Ranker scores and ranks hosts for a given resource request.
type Ranker struct{}

// NewRanker creates a Ranker.
func NewRanker() *Ranker {
	return &Ranker{}
}

// Rank orders candidate hosts by suitability for the request, highest score first.
func (r *Ranker) Rank(candidates []HostScore, req ResourceRequest) []HostScore {
	for i := range candidates {
		candidates[i].Score = r.score(candidates[i], req)
	}

	// Simple insertion sort — candidate lists are small.
	for i := 1; i < len(candidates); i++ {
		key := candidates[i]
		j := i - 1
		for j >= 0 && candidates[j].Score < key.Score {
			candidates[j+1] = candidates[j]
			j--
		}
		candidates[j+1] = key
	}
	return candidates
}

// score computes a suitability score for a single host.
// Higher is better. Returns 0 if the host cannot satisfy the request.
func (r *Ranker) score(hs HostScore, req ResourceRequest) float64 {
	availRAM := hs.Metrics.AvailableRAMGB()
	availVRAM := hs.Metrics.AvailableGPUVramGB()
	availCPU := hs.Metrics.AvailableCPUPercent()

	// Hard constraints — host must have enough resources.
	if availRAM < float64(req.RAMGB) {
		return 0
	}
	if req.GPUVramGB > 0 && availVRAM < req.GPUVramGB {
		return 0
	}

	// Weighted score: prefer hosts with more headroom.
	cpuScore := availCPU / 100.0
	ramScore := availRAM / hs.Metrics.RAMTotalGB
	vmPenalty := 1.0 / float64(1+hs.Metrics.ActiveVMs)

	score := 0.4*cpuScore + 0.4*ramScore + 0.2*vmPenalty

	// Bonus for GPU availability when requested.
	if req.GPUVramGB > 0 && hs.Metrics.GPUVramTotal > 0 {
		gpuScore := availVRAM / hs.Metrics.GPUVramTotal
		score = 0.3*cpuScore + 0.3*ramScore + 0.2*gpuScore + 0.2*vmPenalty
	}

	return score
}
