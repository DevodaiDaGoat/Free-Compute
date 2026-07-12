package monitoring

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type HealthStatus string

const (
	HealthOK      HealthStatus = "ok"
	HealthDegraded HealthStatus = "degraded"
	HealthDown    HealthStatus = "down"
)

type ComponentHealth struct {
	Name      string       `json:"name"`
	Status    HealthStatus `json:"status"`
	Message   string       `json:"message,omitempty"`
	LastCheck time.Time    `json:"lastCheck"`
	LatencyMs int64        `json:"latencyMs"`
}

type HealthReport struct {
	Status     HealthStatus       `json:"status"`
	Uptime     string             `json:"uptime"`
	Components []*ComponentHealth `json:"components"`
	Timestamp  time.Time          `json:"timestamp"`
}

type HealthChecker struct {
	mu         sync.RWMutex
	components map[string]*ComponentHealth
	startTime  time.Time
}

func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		components: make(map[string]*ComponentHealth),
		startTime:  time.Now(),
	}
}

func (hc *HealthChecker) RegisterComponent(name string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	hc.components[name] = &ComponentHealth{
		Name:      name,
		Status:    HealthDown,
		LastCheck: time.Now(),
	}
}

func (hc *HealthChecker) ReportHealth(name string, status HealthStatus, message string, latency time.Duration) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if comp, ok := hc.components[name]; ok {
		comp.Status = status
		comp.Message = message
		comp.LastCheck = time.Now()
		comp.LatencyMs = latency.Milliseconds()
	} else {
		hc.components[name] = &ComponentHealth{
			Name:      name,
			Status:    status,
			Message:   message,
			LastCheck: time.Now(),
			LatencyMs: latency.Milliseconds(),
		}
	}
}

func (hc *HealthChecker) Report() *HealthReport {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	overall := HealthOK
	// Snapshot each ComponentHealth into a fresh value so the JSON encoder
	// (which runs after we release the RLock) can't race with a concurrent
	// ReportHealth mutating the underlying struct.
	components := make([]*ComponentHealth, 0, len(hc.components))

	for _, comp := range hc.components {
		snap := *comp
		components = append(components, &snap)
		if comp.Status == HealthDown {
			overall = HealthDown
		} else if comp.Status == HealthDegraded && overall != HealthDown {
			overall = HealthDegraded
		}
	}

	uptime := time.Since(hc.startTime).Round(time.Second).String()

	return &HealthReport{
		Status:     overall,
		Uptime:     uptime,
		Components: components,
		Timestamp:  time.Now(),
	}
}

func (hc *HealthChecker) HandleHealthDetailed(w http.ResponseWriter, r *http.Request) {
	report := hc.Report()
	w.Header().Set("Content-Type", "application/json")

	statusCode := http.StatusOK
	if report.Status == HealthDown {
		statusCode = http.StatusServiceUnavailable
	} else if report.Status == HealthDegraded {
		statusCode = http.StatusOK
	}

	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(report)
}

func (hc *HealthChecker) HandleHealthz(w http.ResponseWriter, r *http.Request) {
	report := hc.Report()
	w.Header().Set("Content-Type", "application/json")

	if report.Status == HealthDown {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, `{"status":"down","uptime":"%s"}`, report.Uptime)
	} else {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","uptime":"%s"}`, report.Uptime)
	}
}
