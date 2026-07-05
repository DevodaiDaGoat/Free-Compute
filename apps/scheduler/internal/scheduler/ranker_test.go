package scheduler

import (
	"math"
	"testing"
	"time"
)

func almostEqual(a, b, epsilon float64) bool {
	return math.Abs(a-b) < epsilon
}

func TestClamp(t *testing.T) {
	tests := []struct {
		name     string
		v, lo, hi float64
		want     float64
	}{
		{"within range", 0.5, 0, 1, 0.5},
		{"below lo", -1, 0, 1, 0},
		{"above hi", 2, 0, 1, 1},
		{"at lo", 0, 0, 1, 0},
		{"at hi", 1, 0, 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clamp(tt.v, tt.lo, tt.hi)
			if got != tt.want {
				t.Errorf("clamp(%v, %v, %v) = %v, want %v", tt.v, tt.lo, tt.hi, got, tt.want)
			}
		})
	}
}

func TestCPUScore(t *testing.T) {
	tests := []struct {
		name string
		host Host
		want float64
	}{
		{
			"fully idle 32-core",
			Host{CPUCores: 32, CPUUsage: 0},
			1.0,
		},
		{
			"fully loaded",
			Host{CPUCores: 16, CPUUsage: 1.0},
			0.0,
		},
		{
			"half loaded 16-core",
			Host{CPUCores: 16, CPUUsage: 0.5},
			0.25,
		},
		{
			"idle 64-core clamped",
			Host{CPUCores: 64, CPUUsage: 0},
			1.0, // clamped to 1.0
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cpuScore(tt.host)
			if !almostEqual(got, tt.want, 0.001) {
				t.Errorf("cpuScore() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRAMScore(t *testing.T) {
	tests := []struct {
		name string
		host Host
		want float64
	}{
		{"128GB idle", Host{RAMGB: 128, RAMUsage: 0}, 1.0},
		{"128GB full", Host{RAMGB: 128, RAMUsage: 1.0}, 0.0},
		{"64GB half used", Host{RAMGB: 64, RAMUsage: 0.5}, 0.25},
		{"256GB idle clamped", Host{RAMGB: 256, RAMUsage: 0}, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ramScore(tt.host)
			if !almostEqual(got, tt.want, 0.001) {
				t.Errorf("ramScore() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGPUScore(t *testing.T) {
	tests := []struct {
		name string
		host Host
		want float64
	}{
		{"no GPU", Host{GPUVramGB: 0}, 0.0},
		{"24GB idle", Host{GPUVramGB: 24, GPUUsage: 0}, 1.0},
		{"24GB full", Host{GPUVramGB: 24, GPUUsage: 1.0}, 0.0},
		{"12GB half", Host{GPUVramGB: 12, GPUUsage: 0.5}, 0.25},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gpuScore(tt.host)
			if !almostEqual(got, tt.want, 0.001) {
				t.Errorf("gpuScore() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFreshnessScore(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		host Host
		want float64
	}{
		{"just now", Host{LastHeartbeat: now}, 1.0},
		{"150s ago", Host{LastHeartbeat: now.Add(-150 * time.Second)}, 0.5},
		{"300s ago", Host{LastHeartbeat: now.Add(-300 * time.Second)}, 0.0},
		{"600s ago", Host{LastHeartbeat: now.Add(-600 * time.Second)}, 0.0},
		{"future heartbeat", Host{LastHeartbeat: now.Add(60 * time.Second)}, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := freshnessScore(tt.host, now)
			if !almostEqual(got, tt.want, 0.01) {
				t.Errorf("freshnessScore() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScoreHost_OfflineReturnsZero(t *testing.T) {
	h := Host{Online: false, CPUCores: 32, RAMGB: 128}
	score := ScoreHost(h, DefaultWeights(), time.Now())
	if score != 0 {
		t.Errorf("offline host score = %v, want 0", score)
	}
}

func TestScoreHost_IdealHost(t *testing.T) {
	now := time.Now()
	h := Host{
		Online:        true,
		CPUCores:      32,
		RAMGB:         128,
		GPUVramGB:     24,
		CPUUsage:      0,
		RAMUsage:      0,
		GPUUsage:      0,
		LastHeartbeat: now,
	}
	score := ScoreHost(h, DefaultWeights(), now)
	// cpu=1.0*0.30 + ram=1.0*0.25 + gpu=1.0*0.20 + fresh=1.0*0.15 = 0.90
	if !almostEqual(score, 0.9, 0.01) {
		t.Errorf("ideal host score = %v, want ~0.9", score)
	}
}

func TestDefaultWeights_SumToOne(t *testing.T) {
	w := DefaultWeights()
	sum := w.CPU + w.RAM + w.GPU + w.Latency + w.Freshness
	if !almostEqual(sum, 1.0, 0.001) {
		t.Errorf("weights sum to %v, want 1.0", sum)
	}
}

func TestRankHosts(t *testing.T) {
	now := time.Now()
	hosts := []Host{
		{ID: "worst", Online: true, CPUCores: 4, RAMGB: 8, CPUUsage: 0.9, RAMUsage: 0.9, LastHeartbeat: now.Add(-200 * time.Second)},
		{ID: "best", Online: true, CPUCores: 32, RAMGB: 128, GPUVramGB: 24, LastHeartbeat: now},
		{ID: "offline", Online: false, CPUCores: 64, RAMGB: 256},
		{ID: "mid", Online: true, CPUCores: 16, RAMGB: 64, CPUUsage: 0.3, RAMUsage: 0.2, LastHeartbeat: now},
	}

	ranked := RankHosts(hosts, DefaultWeights(), now)

	if len(ranked) != 3 {
		t.Fatalf("expected 3 ranked hosts (offline excluded), got %d", len(ranked))
	}

	if ranked[0].Host.ID != "best" {
		t.Errorf("first ranked host = %s, want 'best'", ranked[0].Host.ID)
	}
	if ranked[len(ranked)-1].Host.ID != "worst" {
		t.Errorf("last ranked host = %s, want 'worst'", ranked[len(ranked)-1].Host.ID)
	}

	for i := 1; i < len(ranked); i++ {
		if ranked[i].Score > ranked[i-1].Score {
			t.Error("hosts not sorted in descending order")
		}
	}
}

func TestRankHosts_Empty(t *testing.T) {
	ranked := RankHosts(nil, DefaultWeights(), time.Now())
	if len(ranked) != 0 {
		t.Errorf("expected empty result, got %d", len(ranked))
	}
}

func TestRankHosts_AllOffline(t *testing.T) {
	hosts := []Host{
		{ID: "a", Online: false},
		{ID: "b", Online: false},
	}
	ranked := RankHosts(hosts, DefaultWeights(), time.Now())
	if len(ranked) != 0 {
		t.Errorf("expected empty result for all-offline, got %d", len(ranked))
	}
}
