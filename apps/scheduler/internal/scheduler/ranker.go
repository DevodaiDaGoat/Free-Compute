package scheduler

import (
	"math"
	"sort"
	"time"
)

// Host represents a compute host in the cluster.
type Host struct {
	ID            string
	Name          string
	Region        string
	CPUCores      int
	RAMGB         float64
	GPUVramGB     float64
	Online        bool
	LastHeartbeat time.Time
	// Current utilization (0.0 - 1.0)
	CPUUsage float64
	RAMUsage float64
	GPUUsage float64
}

// RankWeights controls the influence of each factor in the Galaxy ranking algorithm.
type RankWeights struct {
	CPU       float64
	RAM       float64
	GPU       float64
	Latency   float64
	Freshness float64
}

// DefaultWeights returns the default weighting for the Galaxy algorithm.
func DefaultWeights() RankWeights {
	return RankWeights{
		CPU:       0.30,
		RAM:       0.25,
		GPU:       0.20,
		Latency:   0.10,
		Freshness: 0.15,
	}
}

// HostScore pairs a host with its computed score.
type HostScore struct {
	Host  Host
	Score float64
}

// cpuScore computes a score based on available CPU capacity.
func cpuScore(h Host) float64 {
	available := 1.0 - h.CPUUsage
	capacity := float64(h.CPUCores)
	return clamp(available*capacity/32.0, 0, 1)
}

// ramScore computes a score based on available RAM.
func ramScore(h Host) float64 {
	available := (1.0 - h.RAMUsage) * h.RAMGB
	return clamp(available/128.0, 0, 1)
}

// gpuScore computes a score based on available GPU VRAM.
func gpuScore(h Host) float64 {
	if h.GPUVramGB == 0 {
		return 0
	}
	available := (1.0 - h.GPUUsage) * h.GPUVramGB
	return clamp(available/24.0, 0, 1)
}

// freshnessScore returns a score based on how recently the host sent a heartbeat.
func freshnessScore(h Host, now time.Time) float64 {
	elapsed := now.Sub(h.LastHeartbeat).Seconds()
	if elapsed < 0 {
		elapsed = 0
	}
	// Full score at 0s, decays to 0 at 300s (5 min).
	return clamp(1.0-elapsed/300.0, 0, 1)
}

// ScoreHost calculates the composite Galaxy score for a single host.
func ScoreHost(h Host, w RankWeights, now time.Time) float64 {
	if !h.Online {
		return 0
	}
	s := w.CPU*cpuScore(h) +
		w.RAM*ramScore(h) +
		w.GPU*gpuScore(h) +
		w.Freshness*freshnessScore(h, now)
	return math.Round(s*1000) / 1000
}

// RankHosts scores and sorts hosts from best to worst.
func RankHosts(hosts []Host, w RankWeights, now time.Time) []HostScore {
	scored := make([]HostScore, 0, len(hosts))
	for _, h := range hosts {
		s := ScoreHost(h, w, now)
		if s > 0 {
			scored = append(scored, HostScore{Host: h, Score: s})
		}
	}
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})
	return scored
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
