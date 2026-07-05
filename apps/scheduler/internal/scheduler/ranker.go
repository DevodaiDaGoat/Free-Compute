package scheduler

import (
	"sort"

	"github.com/DevodaiDaGoat/Free-Compute/apps/scheduler/internal/database"
)

// Ranker scores hosts based on available resources and suitability.
type Ranker struct{}

func NewRanker() *Ranker {
	return &Ranker{}
}

type scoredHost struct {
	database.Host
	score float64
}

// RankHosts scores and sorts hosts by suitability for the given resource request.
// The Galaxy algorithm prioritizes:
// 1. Sufficient resources (hard requirement)
// 2. Least loaded (best fit — avoid fragmentation)
// 3. Freshest heartbeat (most reliable)
func (r *Ranker) RankHosts(hosts []database.Host, req ScheduleRequest) []database.Host {
	var scored []scoredHost

	for _, h := range hosts {
		// Hard requirement: host must have enough resources
		if h.CPUCores < req.CPUCores || h.RAMGB < req.RAMGB {
			continue
		}

		// GPU requirement (if specified)
		if req.GPUVRAM > 0 && h.GPUVramGB < req.GPUVRAM {
			continue
		}

		// Score: lower residual = better fit (bin-packing heuristic)
		cpuResidual := float64(h.CPUCores-req.CPUCores) / float64(h.CPUCores)
		ramResidual := float64(h.RAMGB-req.RAMGB) / float64(h.RAMGB)
		score := 1.0 - (cpuResidual+ramResidual)/2.0

		scored = append(scored, scoredHost{Host: h, score: score})
	}

	// Sort by score descending (best fit first)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	result := make([]database.Host, len(scored))
	for i, s := range scored {
		result[i] = s.Host
	}
	return result
}
