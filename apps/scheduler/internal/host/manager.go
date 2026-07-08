package host

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

type Host struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Region           string    `json:"region"`
	CPUCores         int       `json:"cpuCores"`
	RAMGB            int       `json:"ramGb"`
	GPUVramGB        int       `json:"gpuVramGb"`
	GPUModel         string    `json:"gpuModel"`
	DriverVersion    string    `json:"driverVersion"`
	EncoderSupport   []string  `json:"encoderSupport"`
	UplinkMbps       int       `json:"uplinkMbps"`
	LatencyMs        int       `json:"latencyMs"`
	UptimeHours      int       `json:"uptimeHours"`
	Online           bool      `json:"online"`
	AllocatedCPUCores int     `json:"allocatedCpuCores"`
	AllocatedRAMGB   int       `json:"allocatedRamGb"`
	AllocatedGPUVram int       `json:"allocatedGpuVram"`
	LastHeartbeat    time.Time `json:"lastHeartbeat"`
	Capabilities     []string  `json:"capabilities"`
	CreatedAt        time.Time `json:"createdAt"`
}

type Manager struct {
	mu    sync.RWMutex
	hosts map[string]*Host
	logger *log.Logger
}

func NewManager(logger *log.Logger) *Manager {
	if logger == nil {
		logger = log.Default()
	}
	return &Manager{
		hosts:  make(map[string]*Host),
		logger: logger,
	}
}

func (m *Manager) Register(h *Host) {
	m.mu.Lock()
	h.Online = true
	h.LastHeartbeat = time.Now()
	h.CreatedAt = time.Now()
	m.hosts[h.ID] = h
	m.mu.Unlock()
	m.logger.Printf("host registered: %s (%s, %d CPU, %d GB RAM)", h.ID[:8], h.Name, h.CPUCores, h.RAMGB)
}

func (m *Manager) Heartbeat(id string, metrics map[string]any) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	h, ok := m.hosts[id]
	if !ok {
		return false
	}

	h.Online = true
	h.LastHeartbeat = time.Now()

	if cpu, ok := metrics["cpuCores"].(float64); ok {
		h.CPUCores = int(cpu)
	}
	if ram, ok := metrics["ramGb"].(float64); ok {
		h.RAMGB = int(ram)
	}
	if gpuVram, ok := metrics["gpuVramGb"].(float64); ok {
		h.GPUVramGB = int(gpuVram)
	}
	if uptime, ok := metrics["uptimeHours"].(float64); ok {
		h.UptimeHours = int(uptime)
	}
	if latency, ok := metrics["latencyMs"].(float64); ok {
		h.LatencyMs = int(latency)
	}

	return true
}

func (m *Manager) Get(id string) *Host {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.hosts[id]
}

func (m *Manager) GetAvailable() []*Host {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Host
	now := time.Now()

	for _, h := range m.hosts {
		if !h.Online {
			continue
		}
		if now.Sub(h.LastHeartbeat) > 30*time.Second {
			continue
		}
		if h.AllocatedCPUCores >= h.CPUCores {
			continue
		}
		if h.AllocatedRAMGB >= h.RAMGB {
			continue
		}
		result = append(result, h)
	}
	return result
}

func (m *Manager) Allocate(id string, cpu, ram int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	h, ok := m.hosts[id]
	if !ok {
		return false
	}
	if h.AllocatedCPUCores+cpu > h.CPUCores {
		return false
	}
	if h.AllocatedRAMGB+ram > h.RAMGB {
		return false
	}

	h.AllocatedCPUCores += cpu
	h.AllocatedRAMGB += ram
	return true
}

func (m *Manager) Release(id string, cpu, ram int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	h, ok := m.hosts[id]
	if !ok {
		return
	}
	h.AllocatedCPUCores = max(0, h.AllocatedCPUCores-cpu)
	h.AllocatedRAMGB = max(0, h.AllocatedRAMGB-ram)
}

func (m *Manager) MarkOffline(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if h, ok := m.hosts[id]; ok {
		h.Online = false
		m.logger.Printf("host marked offline: %s", id[:8])
	}
}

func (m *Manager) List() []*Host {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Host, 0, len(m.hosts))
	for _, h := range m.hosts {
		result = append(result, h)
	}
	return result
}

func (m *Manager) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var h Host
	if err := json.NewDecoder(r.Body).Decode(&h); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if h.ID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	m.Register(&h)
	writeJSON(w, http.StatusCreated, h)
}

func (m *Manager) HandleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID      string         `json:"id"`
		Metrics map[string]any `json:"metrics"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if ok := m.Heartbeat(req.ID, req.Metrics); !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "host not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (m *Manager) HandleListHosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	hosts := m.List()
	writeJSON(w, http.StatusOK, map[string]any{
		"hosts": hosts,
		"count": len(hosts),
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
