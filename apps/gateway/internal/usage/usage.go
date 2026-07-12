package usage

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type ResourceType string

const (
	ResourceCPU      ResourceType = "cpu_cores"
	ResourceRAM      ResourceType = "ram_gb"
	ResourceStorage  ResourceType = "storage_gb"
	ResourceGPU      ResourceType = "gpu_hours"
	ResourceNetwork  ResourceType = "network_gb"
	ResourceSessions ResourceType = "sessions"
)

type UsageRecord struct {
	UserID     string       `json:"userId"`
	Resource   ResourceType `json:"resource"`
	Value      float64      `json:"value"`
	Unit       string       `json:"unit"`
	StartedAt  time.Time    `json:"startedAt"`
	EndedAt    time.Time    `json:"endedAt,omitempty"`
}

type UsageSummary struct {
	UserID          string             `json:"userId"`
	PeriodStart     time.Time          `json:"periodStart"`
	PeriodEnd       time.Time          `json:"periodEnd"`
	Resources       map[string]float64 `json:"resources"`
	SessionCount    int                `json:"sessionCount"`
	TotalUptimeHours float64           `json:"totalUptimeHours"`
}

type Quota struct {
	UserID       string             `json:"userId"`
	Limits       map[string]float64 `json:"limits"`
	Usage        map[string]float64 `json:"usage"`
	Remaining    map[string]float64 `json:"remaining"`
}

type Invoice struct {
	ID          string    `json:"id"`
	UserID      string    `json:"userId"`
	PeriodStart time.Time `json:"periodStart"`
	PeriodEnd   time.Time `json:"periodEnd"`
	Items       []InvoiceItem `json:"items"`
	TotalCents  int64     `json:"totalCents"`
	Paid        bool      `json:"paid"`
	CreatedAt   time.Time `json:"createdAt"`
}

type InvoiceItem struct {
	Description string `json:"description"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   int    `json:"unitPriceCents"`
	TotalCents  int64  `json:"totalCents"`
}

type Tracker struct {
	mu        sync.RWMutex
	records   map[string][]*UsageRecord
	quotas    map[string]map[string]float64
	logger    *log.Logger
}

func NewTracker(logger *log.Logger) *Tracker {
	if logger == nil {
		logger = log.Default()
	}
	t := &Tracker{
		records: make(map[string][]*UsageRecord),
		quotas:  make(map[string]map[string]float64),
		logger:  logger,
	}
	t.seedDefaultQuotas()
	return t
}

func (t *Tracker) seedDefaultQuotas() {
	t.quotas["default"] = map[string]float64{
		string(ResourceCPU):      4,
		string(ResourceRAM):      8,
		string(ResourceStorage):  50,
		string(ResourceGPU):      0,
		string(ResourceNetwork):  100,
		string(ResourceSessions): 5,
	}
}

func (t *Tracker) Track(userID string, resource ResourceType, value float64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.records[userID] = append(t.records[userID], &UsageRecord{
		UserID:    userID,
		Resource:  resource,
		Value:     value,
		Unit:      unitFor(resource),
		StartedAt: time.Now(),
	})

	// Opportunistically prune records older than 30 days when the slice grows
	// past 512 entries. Older records are never read (calculateUsage / GetUsage
	// filter by 30-day window) so keeping them is a slow memory leak.
	records := t.records[userID]
	if len(records) > 512 {
		cutoff := time.Now().Add(-30 * 24 * time.Hour)
		kept := records[:0]
		for _, r := range records {
			if r.StartedAt.After(cutoff) {
				kept = append(kept, r)
			}
		}
		t.records[userID] = kept
	}
}

func (t *Tracker) GetUsage(userID string, since time.Time) *UsageSummary {
	t.mu.RLock()
	defer t.mu.RUnlock()

	summary := &UsageSummary{
		UserID:      userID,
		PeriodStart: since,
		PeriodEnd:   time.Now(),
		Resources:   make(map[string]float64),
	}

	records := t.records[userID]
	for _, r := range records {
		if r.StartedAt.Before(since) {
			continue
		}
		summary.Resources[string(r.Resource)] += r.Value
		if r.Resource == ResourceSessions {
			summary.SessionCount++
		}
	}

	summary.TotalUptimeHours = summary.Resources[string(ResourceCPU)] / 3600
	return summary
}

func (t *Tracker) GetQuota(userID string) *Quota {
	t.mu.RLock()
	defer t.mu.RUnlock()

	limits, ok := t.quotas[userID]
	if !ok {
		limits = t.quotas["default"]
	}

	usage := t.calculateUsage(userID)

	quota := &Quota{
		UserID:    userID,
		Limits:    make(map[string]float64),
		Usage:     make(map[string]float64),
		Remaining: make(map[string]float64),
	}

	for k, v := range limits {
		quota.Limits[k] = v
		quota.Usage[k] = usage[k]
		quota.Remaining[k] = v - usage[k]
	}

	return quota
}

func (t *Tracker) SetQuota(userID string, resource ResourceType, limit float64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.quotas[userID] == nil {
		t.quotas[userID] = make(map[string]float64)
		defaults := t.quotas["default"]
		for k, v := range defaults {
			t.quotas[userID][k] = v
		}
	}
	t.quotas[userID][string(resource)] = limit
}

func (t *Tracker) calculateUsage(userID string) map[string]float64 {
	usage := make(map[string]float64)
	since := time.Now().Add(-30 * 24 * time.Hour)

	for _, r := range t.records[userID] {
		if r.StartedAt.Before(since) {
			continue
		}
		usage[string(r.Resource)] += r.Value
	}

	return usage
}

func (t *Tracker) CheckQuota(userID string, resource ResourceType, requested float64) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	limits, ok := t.quotas[userID]
	if !ok {
		limits = t.quotas["default"]
	}

	usage := t.calculateUsage(userID)
	limit, hasLimit := limits[string(resource)]
	if !hasLimit {
		return true
	}

	return usage[string(resource)]+requested <= limit
}

func (t *Tracker) GenerateInvoice(userID string, since, until time.Time) *Invoice {
	usage := t.GetUsage(userID, since)

	prices := map[string]int{
		string(ResourceCPU):      50,
		string(ResourceRAM):      30,
		string(ResourceStorage):  5,
		string(ResourceGPU):      200,
		string(ResourceNetwork):  10,
	}

	var items []InvoiceItem
	var total int64

	for resource, value := range usage.Resources {
		unitPrice := prices[resource]
		itemTotal := int64(value * float64(unitPrice))
		items = append(items, InvoiceItem{
			Description: fmt.Sprintf("%s usage", resource),
			Quantity:    value,
			UnitPrice:   unitPrice,
			TotalCents:  itemTotal,
		})
		total += itemTotal
	}

	shortID := userID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	return &Invoice{
		ID:          fmt.Sprintf("inv_%s_%d", shortID, since.Unix()),
		UserID:      userID,
		PeriodStart: since,
		PeriodEnd:   until,
		Items:       items,
		TotalCents:  total,
		CreatedAt:   time.Now(),
	}
}

func (t *Tracker) HandleUsage(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userId")
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "userId required"})
		return
	}

	days := 7
	if d := r.URL.Query().Get("days"); d != "" {
		var parsed int
		if _, err := fmt.Sscanf(d, "%d", &parsed); err == nil {
			// Clamp to a sane range. Negative days would make `since` land in
			// the future so every query returned nothing; huge values would
			// scan the entire recording set uselessly.
			if parsed < 1 {
				parsed = 1
			} else if parsed > 365 {
				parsed = 365
			}
			days = parsed
		}
	}

	since := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	usage := t.GetUsage(userID, since)
	writeJSON(w, http.StatusOK, usage)
}

func (t *Tracker) HandleQuota(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userId")
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "userId required"})
		return
	}

	switch r.Method {
	case "GET":
		quota := t.GetQuota(userID)
		writeJSON(w, http.StatusOK, quota)

	case "PUT":
		var req struct {
			Resource string  `json:"resource"`
			Limit    float64 `json:"limit"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		t.SetQuota(userID, ResourceType(req.Resource), req.Limit)
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})

	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (t *Tracker) HandleInvoice(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userId")
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "userId required"})
		return
	}

	since := time.Now().Add(-30 * 24 * time.Hour)
	invoice := t.GenerateInvoice(userID, since, time.Now())
	writeJSON(w, http.StatusOK, invoice)
}

func unitFor(resource ResourceType) string {
	switch resource {
	case ResourceCPU:
		return "core-seconds"
	case ResourceRAM:
		return "GB-seconds"
	case ResourceStorage:
		return "GB-months"
	case ResourceGPU:
		return "GPU-seconds"
	case ResourceNetwork:
		// tunnel/server.handleHostMetrics feeds float64(bytesOut) into Track,
		// so the raw values are bytes; the human label is bytes, not GB.
		return "bytes"
	case ResourceSessions:
		return "count"
	default:
		return "units"
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
