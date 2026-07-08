package scheduler

import (
	"math"
	"sort"

	"github.com/freecompute/free-compute/apps/scheduler/internal/host"
)

type rankedHost struct {
	host  *host.Host
	score float64
}

func (s *Scheduler) rankHosts(item *QueueItem, hosts []*host.Host) []*host.Host {
	ranked := make([]rankedHost, 0, len(hosts))

	for _, h := range hosts {
		score := s.scoreHost(item, h)
		if score > 0 {
			ranked = append(ranked, rankedHost{host: h, score: score})
		}
	}

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})

	result := make([]*host.Host, len(ranked))
	for i, r := range ranked {
		result[i] = r.host
	}
	return result
}

func (s *Scheduler) scoreHost(item *QueueItem, h *host.Host) float64 {
	if h.CPUCores-h.AllocatedCPUCores < item.CPUCores {
		return 0
	}
	if h.RAMGB-h.AllocatedRAMGB < item.RAMGB {
		return 0
	}
	if item.GPURequired && h.GPUVramGB == 0 {
		return 0
	}
	if item.GPURequired && h.GPUVramGB < item.GPUVramGB {
		return 0
	}

	score := 100.0

	cpuAvail := float64(h.CPUCores - h.AllocatedCPUCores)
	cpuRatio := float64(item.CPUCores) / cpuAvail
	score -= cpuRatio * 20

	ramAvail := float64(h.RAMGB - h.AllocatedRAMGB)
	ramRatio := float64(item.RAMGB) / ramAvail
	score -= ramRatio * 20

	if h.UplinkMbps > 0 && item.LatencyBudgetMs > 0 {
		latencyScore := float64(100-h.LatencyMs) / 100.0
		score += latencyScore * 15
		if h.LatencyMs > item.LatencyBudgetMs {
			score -= 30
		}
	}

	if h.UptimeHours > 0 {
		uptimeScore := math.Min(float64(h.UptimeHours)/720, 1.0)
		score += uptimeScore * 10
	}

	if h.GPUVramGB > 0 {
		score += 15
	}

	loadRatio := float64(h.AllocatedCPUCores) / float64(h.CPUCores)
	score -= loadRatio * 20

	if score < 0 {
		score = 0
	}

	return score
}
