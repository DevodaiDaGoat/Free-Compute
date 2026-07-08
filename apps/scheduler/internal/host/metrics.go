package host

import (
	"sync"
	"time"
)

type HostMetrics struct {
	HostID         string    `json:"hostId"`
	CPULoad        float64   `json:"cpuLoad"`
	RAMUsed        int       `json:"ramUsed"`
	RAMTotal       int       `json:"ramTotal"`
	DiskUsed       int64     `json:"diskUsed"`
	DiskTotal      int64     `json:"diskTotal"`
	NetworkRx      int64     `json:"networkRx"`
	NetworkTx      int64     `json:"networkTx"`
	ActiveVMs      int       `json:"activeVms"`
	ActiveSessions int       `json:"activeSessions"`
	CollectedAt    time.Time `json:"collectedAt"`
}

type MetricsCollector struct {
	mu      sync.RWMutex
	history map[string][]*HostMetrics
	maxHist int
}

func NewMetricsCollector(maxHistory int) *MetricsCollector {
	if maxHistory < 1 {
		maxHistory = 100
	}
	return &MetricsCollector{
		history: make(map[string][]*HostMetrics),
		maxHist: maxHistory,
	}
}

func (mc *MetricsCollector) Record(m *HostMetrics) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	m.CollectedAt = time.Now()
	mc.history[m.HostID] = append(mc.history[m.HostID], m)

	if len(mc.history[m.HostID]) > mc.maxHist {
		mc.history[m.HostID] = mc.history[m.HostID][len(mc.history[m.HostID])-mc.maxHist:]
	}
}

func (mc *MetricsCollector) Latest(hostID string) *HostMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	hist := mc.history[hostID]
	if len(hist) == 0 {
		return nil
	}
	return hist[len(hist)-1]
}

func (mc *MetricsCollector) History(hostID string) []*HostMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	hist := mc.history[hostID]
	result := make([]*HostMetrics, len(hist))
	copy(result, hist)
	return result
}

func (mc *MetricsCollector) AverageLoad(hostID string, window time.Duration) float64 {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	hist := mc.history[hostID]
	if len(hist) == 0 {
		return 0
	}

	cutoff := time.Now().Add(-window)
	var total float64
	var count int

	for i := len(hist) - 1; i >= 0; i-- {
		if hist[i].CollectedAt.Before(cutoff) {
			break
		}
		total += hist[i].CPULoad
		count++
	}

	if count == 0 {
		return 0
	}
	return total / float64(count)
}
