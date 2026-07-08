package security

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/freecompute/free-compute/apps/gateway/internal/database"
)

var AIModerationActive atomic.Bool

func SetAIModerationActive(active bool) {
	AIModerationActive.Store(active)
}

func GetAIModerationActive() bool {
	return AIModerationActive.Load()
}

type ThreatLevel string

const (
	ThreatLow       ThreatLevel = "low"
	ThreatMedium    ThreatLevel = "medium"
	ThreatHigh      ThreatLevel = "high"
	ThreatCritical  ThreatLevel = "critical"
)

type VMState string

const (
	VMStateClean     VMState = "clean"
	VMStateFlagged   VMState = "flagged"
	VMStatePaused    VMState = "paused"
	VMStateQuarantine VMState = "quarantine"
)

type ThreatEvent struct {
	ID          string                 `json:"id"`
	VMID        string                 `json:"vmId"`
	UserID      string                 `json:"userId"`
	Type        string                 `json:"type"`
	Level       ThreatLevel            `json:"level"`
	Description string                 `json:"description"`
	Evidence    map[string]interface{} `json:"evidence"`
	ScreenShot  string                 `json:"screenshot,omitempty"`
	CreatedAt   time.Time              `json:"createdAt"`
	ReviewedAt  *time.Time             `json:"reviewedAt,omitempty"`
	ReviewedBy  string                 `json:"reviewedBy,omitempty"`
	Action      string                 `json:"action,omitempty"`
	Resolved    bool                   `json:"resolved"`
}

type SecurityDetector struct {
	logger      *log.Logger
	mu          sync.RWMutex
	threats     map[string]*ThreatEvent
	vmStates    map[string]*VMThreatState
	cryptoHashes map[string]bool
	malwareSignatures map[string]string
}

type VMThreatState struct {
	VMID          string    `json:"vmId"`
	State         VMState   `json:"state"`
	CPUUsage      float64   `json:"cpuUsage"`
	GPUsage       float64   `json:"gpuUsage"`
	NetworkMBps   float64   `json:"networkMbps"`
	ProcessCount  int       `json:"processCount"`
	FlaggedAt     time.Time `json:"flaggedAt"`
	PausedAt      time.Time `json:"pausedAt"`
	ScreenCapture string    `json:"screenCapture"`
	Score         float64   `json:"score"`
}

func NewSecurityDetector(logger *log.Logger) *SecurityDetector {
	if logger == nil {
		logger = log.Default()
	}
	d := &SecurityDetector{
		logger:      logger,
		threats:     make(map[string]*ThreatEvent),
		vmStates:    make(map[string]*VMThreatState),
		cryptoHashes: make(map[string]bool),
		malwareSignatures: make(map[string]string),
	}
	d.initSignatures()
	return d
}

func (d *SecurityDetector) initSignatures() {
	crypto := []string{
		"xmrig", "minerd", "ccminer", "ethminer", "claymore",
		"cpuminer", "sgminer", "bfgminer", "cgminer", "nicehash",
		"t-rex", "phoenixminer", "lolminer", "teamredminer", "nbminer",
		"gminer", "wildrig", "srbminer", "nanominer", "moneroocean",
		"cryptonight", "ethash", "kawpow", "randomx", "equihash",
		"stratum+tcp", "stratum+ssl", "stratum2+tcp",
		"minerd", "xmrstak", "xmrminer", "bfgminer",
	}
	d.mu.Lock()
	for _, s := range crypto {
		d.cryptoHashes[s] = true
	}

	d.malwareSignatures = map[string]string{
		"shellter":        "Legitimate binary injector often used by malware",
		"mimikatz":        "Credential dumping tool",
		"powersploit":     "Post-exploitation framework",
		"meterpreter":     "Metasploit payload",
		"cobaltstrike":    "Commercial pentest framework abused by threat actors",
		"shikata_ga_nai":  "Metasploit encoder",
		"havoc":           "C2 framework",
		"sliver":          "C2 framework",
		"bruteforce":      "Brute force attack pattern",
		"masscan":         "Mass port scanner",
		"hydra":           "Login brute forcer",
		"nmap":            "Network scanner (when used aggressively)",
		"sqlmap":          "SQL injection tool",
		"beef":            "Browser exploitation framework",
		"ettercap":        "MITM attack tool",
		"bettercap":       "MITM framework",
		"wireshark":       "Packet analyzer (when used for packet capture)",
		"tcpdump":         "Packet capture (aggressive usage)",
	}

	d.malwareSignatures["john"] = "Password cracker (illegal-activity heuristic)"
	d.malwareSignatures["johnny"] = "John the Ripper GUI (illegal-activity heuristic)"
	d.mu.Unlock()
}

func (d *SecurityDetector) AnalyzeMetrics(vmID string, cpu, gpu, network float64, processes int) *ThreatEvent {
	d.mu.Lock()
	defer d.mu.Unlock()

	state, exists := d.vmStates[vmID]
	if !exists {
		state = &VMThreatState{VMID: vmID, State: VMStateClean}
		d.vmStates[vmID] = state
	}

	state.CPUUsage = cpu
	state.GPUsage = gpu
	state.NetworkMBps = network
	state.ProcessCount = processes

	score := 0.0

	if cpu > 90 && network > 50 {
		score += 30
	}
	if gpu > 85 && network > 100 {
		score += 25
	}
	if cpu > 70 && network > 200 {
		score += 35
	}
	if processes > 500 {
		score += 10
	}
	persistentHighCPU := cpu > 80
	_ = persistentHighCPU

	state.Score = score

	if score >= 50 {
		event := &ThreatEvent{
			ID:          fmt.Sprintf("threat_%d", time.Now().UnixNano()),
			VMID:        vmID,
			Type:        "crypto-mining",
			Level:       ThreatHigh,
			Description: "Possible crypto mining detected: high CPU/GPU + network",
			Evidence: map[string]interface{}{
				"cpu":     cpu,
				"gpu":     gpu,
				"network": network,
				"score":   score,
				"processes": processes,
			},
			CreatedAt: time.Now(),
		}
		d.threats[event.ID] = event
		state.State = VMStateFlagged
		SetAIModerationActive(true)

		if score >= 70 {
			state.State = VMStatePaused
			event.Level = ThreatCritical
			event.Description = "CRITICAL: Crypto mining with high resource usage - connection paused"
			d.logger.Printf("CRITICAL threat on VM %s: score=%.0f cpu=%.0f%% network=%.0fMbps - CONNECTION PAUSED",
				vmID, score, cpu, network)
		} else {
			d.logger.Printf("threat detected on VM %s: type=%s score=%.0f level=%s",
				vmID, event.Type, score, event.Level)
		}
		return event
	}

	return nil
}

func (d *SecurityDetector) AnalyzeProcessList(vmID string, processes []string) *ThreatEvent {
	d.mu.Lock()
	defer d.mu.Unlock()

	state, exists := d.vmStates[vmID]
	if !exists {
		return nil
	}

	for _, proc := range processes {
		lower := strings.ToLower(proc)

		if d.cryptoHashes[lower] {
			event := &ThreatEvent{
				ID:          fmt.Sprintf("threat_%d", time.Now().UnixNano()),
				VMID:        vmID,
				Type:        "crypto-mining",
				Level:       ThreatCritical,
				Description: fmt.Sprintf("Crypto mining process detected: %s", proc),
				Evidence:   map[string]interface{}{"process": proc},
				CreatedAt:   time.Now(),
			}
			d.threats[event.ID] = event
			state.State = VMStatePaused
			SetAIModerationActive(true)
			d.logger.Printf("CRITICAL: Crypto miner %s on VM %s - CONNECTION PAUSED", proc, vmID)
			return event
		}

		if desc, ok := d.malwareSignatures[lower]; ok {
			event := &ThreatEvent{
				ID:          fmt.Sprintf("threat_%d", time.Now().UnixNano()),
				VMID:        vmID,
				Type:        "malware-tool",
				Level:       ThreatHigh,
				Description: fmt.Sprintf("Suspicious tool detected: %s - %s", proc, desc),
				Evidence:   map[string]interface{}{"process": proc, "description": desc},
				CreatedAt:   time.Now(),
			}
			d.threats[event.ID] = event
			state.State = VMStateFlagged
			SetAIModerationActive(true)
			d.logger.Printf("FLAGGED: %s on VM %s - %s", proc, vmID, desc)
			return event
		}
	}

	return nil
}

func (d *SecurityDetector) AnalyzeTraffic(userID string, bytesIn, bytesOut int64, conns int, dur time.Duration) *ThreatEvent {
	seconds := dur.Seconds()
	if seconds < 1 {
		seconds = 1
	}
	outPerSec := float64(bytesOut) / seconds

	if outPerSec > 50_000_000 || conns > 500 {
		event := &ThreatEvent{
			ID:          fmt.Sprintf("threat_%d", time.Now().UnixNano()),
			UserID:      userID,
			Type:        "traffic-anomaly",
			Level:       ThreatMedium,
			Description: fmt.Sprintf("Traffic anomaly: %.0f bytes/s out, %d conns", outPerSec, conns),
			Evidence: map[string]interface{}{
				"bytesIn":        bytesIn,
				"bytesOut":       bytesOut,
				"conns":          conns,
				"duration":       dur.String(),
				"outBytesPerSec": outPerSec,
			},
			CreatedAt: time.Now(),
		}
		d.mu.Lock()
		d.threats[event.ID] = event
		d.mu.Unlock()
		SetAIModerationActive(true)
		return event
	}

	return nil
}

func (d *SecurityDetector) PauseVM(vmID string, reason string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	state, exists := d.vmStates[vmID]
	if !exists {
		state = &VMThreatState{VMID: vmID, State: VMStatePaused}
		d.vmStates[vmID] = state
	}
	state.State = VMStatePaused
	state.PausedAt = time.Now()
	d.logger.Printf("VM %s paused: %s", vmID, reason)
}

func (d *SecurityDetector) ResumeVM(vmID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	state, exists := d.vmStates[vmID]
	if !exists {
		return
	}
	state.State = VMStateClean
	d.logger.Printf("VM %s resumed", vmID)
}

func (d *SecurityDetector) GetVMState(vmID string) *VMThreatState {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.vmStates[vmID]
}

func (d *SecurityDetector) ListThreats(resolved bool) []*ThreatEvent {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var result []*ThreatEvent
	for _, t := range d.threats {
		if t.Resolved == resolved {
			result = append(result, t)
		}
	}
	return result
}

func (d *SecurityDetector) GetThreat(id string) *ThreatEvent {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.threats[id]
}

func (d *SecurityDetector) ReviewThreat(id, reviewer string, resolved bool, action string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	t, exists := d.threats[id]
	if !exists {
		return
	}
	now := time.Now()
	t.ReviewedAt = &now
	t.ReviewedBy = reviewer
	t.Resolved = resolved
	t.Action = action
	d.logger.Printf("threat %s reviewed by %s: resolved=%v action=%s", id, reviewer, resolved, action)
}

func (d *SecurityDetector) ReloadSignatures(db *database.DB) error {
	if db == nil {
		return nil
	}
	rows, err := db.Query(`SELECT key, value FROM settings WHERE key LIKE 'security_%'`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		switch {
		case key == "security_crypto":
			d.mu.Lock()
			for _, s := range strings.Split(value, ",") {
				s = strings.TrimSpace(s)
				if s != "" {
					d.cryptoHashes[s] = true
				}
			}
			d.mu.Unlock()
		case key == "security_malware":
			var m map[string]string
			if err := json.Unmarshal([]byte(value), &m); err == nil {
				d.mu.Lock()
				for k, v := range m {
					d.malwareSignatures[k] = v
				}
				d.mu.Unlock()
			}
		}
	}
	return nil
}

func (d *SecurityDetector) SetScreenCapture(vmID, capture string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	state, exists := d.vmStates[vmID]
	if !exists {
		return
	}
	state.ScreenCapture = capture
}

func (d *SecurityDetector) ListVMStates() []*VMThreatState {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var states []*VMThreatState
	for _, s := range d.vmStates {
		states = append(states, s)
	}
	return states
}

func (d *SecurityDetector) ThreatCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	count := 0
	for _, t := range d.threats {
		if !t.Resolved {
			count++
		}
	}
	return count
}

type SecurityHandler struct {
	detector *SecurityDetector
}

func NewSecurityHandler(detector *SecurityDetector) *SecurityHandler {
	return &SecurityHandler{detector: detector}
}

func (h *SecurityHandler) ReportMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		VMID      string  `json:"vmId"`
		CPU       float64 `json:"cpu"`
		GPU       float64 `json:"gpu"`
		Network   float64 `json:"network"`
		Processes int     `json:"processes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	threat := h.detector.AnalyzeMetrics(req.VMID, req.CPU, req.GPU, req.Network, req.Processes)
	if threat != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"threat":  threat,
			"paused":  threat.Level == ThreatCritical,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "clean"})
}

func (h *SecurityHandler) ReportProcess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		VMID      string   `json:"vmId"`
		Processes []string `json:"processes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	threat := h.detector.AnalyzeProcessList(req.VMID, req.Processes)
	if threat != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"threat":  threat,
			"paused":  threat.Level == ThreatCritical,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "clean"})
}

type ThreatCountResponse struct {
	Total   int `json:"total"`
	Unresolved int `json:"unresolved"`
	Critical int `json:"critical"`
	High    int `json:"high"`
	Medium  int `json:"medium"`
	Low     int `json:"low"`
}

func (h *SecurityHandler) Stats(w http.ResponseWriter, r *http.Request) {
	threats := h.detector.ListThreats(false)
	counts := ThreatCountResponse{Total: len(threats)}
	for _, t := range threats {
		switch t.Level {
		case ThreatCritical:
			counts.Critical++
		case ThreatHigh:
			counts.High++
		case ThreatMedium:
			counts.Medium++
		case ThreatLow:
			counts.Low++
		}
	}
	counts.Unresolved = counts.Total
	writeJSON(w, http.StatusOK, counts)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}

var _ = math.Max
