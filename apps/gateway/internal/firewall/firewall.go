package firewall

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type Direction string

const (
	DirectionInbound  Direction = "inbound"
	DirectionOutbound Direction = "outbound"
)

type Protocol string

const (
	ProtocolTCP  Protocol = "tcp"
	ProtocolUDP  Protocol = "udp"
	ProtocolICMP Protocol = "icmp"
	ProtocolAny  Protocol = "any"
)

type Action string

const (
	ActionAllow Action = "allow"
	ActionDeny  Action = "deny"
)

type Rule struct {
	ID          string    `json:"id"`
	UserID      string    `json:"userId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Direction   Direction `json:"direction"`
	Protocol    Protocol  `json:"protocol"`
	FromPort    int       `json:"fromPort"`
	ToPort      int       `json:"toPort"`
	CIDR        string    `json:"cidr"`
	Action      Action    `json:"action"`
	Priority    int       `json:"priority"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type SecurityGroup struct {
	ID          string    `json:"id"`
	UserID      string    `json:"userId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Rules       []string  `json:"rules"`
	VMIDs       []string  `json:"vmIds"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Manager struct {
	mu       sync.RWMutex
	rules    map[string]*Rule
	groups   map[string]*SecurityGroup
	logger   *log.Logger
	nextID   int
}

func NewManager(logger *log.Logger) *Manager {
	if logger == nil {
		logger = log.Default()
	}
	m := &Manager{
		rules:  make(map[string]*Rule),
		groups: make(map[string]*SecurityGroup),
		logger: logger,
		nextID: 1,
	}
	m.seedDefaults()
	return m
}

func (m *Manager) seedDefaults() {
	defaults := []*Rule{
		{
			ID: "rule_default_allow_ssh", Name: "Allow SSH",
			Description: "Allow SSH access from anywhere", Direction: DirectionInbound,
			Protocol: ProtocolTCP, FromPort: 22, ToPort: 22,
			CIDR: "0.0.0.0/0", Action: ActionAllow, Priority: 100, Enabled: true,
		},
		{
			ID: "rule_default_allow_http", Name: "Allow HTTP",
			Description: "Allow HTTP access from anywhere", Direction: DirectionInbound,
			Protocol: ProtocolTCP, FromPort: 80, ToPort: 80,
			CIDR: "0.0.0.0/0", Action: ActionAllow, Priority: 100, Enabled: true,
		},
		{
			ID: "rule_default_allow_https", Name: "Allow HTTPS",
			Description: "Allow HTTPS access from anywhere", Direction: DirectionInbound,
			Protocol: ProtocolTCP, FromPort: 443, ToPort: 443,
			CIDR: "0.0.0.0/0", Action: ActionAllow, Priority: 100, Enabled: true,
		},
		{
			ID: "rule_default_allow_webrtc", Name: "Allow WebRTC",
			Description: "Allow WebRTC UDP range", Direction: DirectionInbound,
			Protocol: ProtocolUDP, FromPort: 49152, ToPort: 65535,
			CIDR: "0.0.0.0/0", Action: ActionAllow, Priority: 100, Enabled: true,
		},
		{
			ID: "rule_default_deny_all", Name: "Deny All Inbound",
			Description: "Default deny all inbound traffic", Direction: DirectionInbound,
			Protocol: ProtocolAny, FromPort: 0, ToPort: 0,
			CIDR: "0.0.0.0/0", Action: ActionDeny, Priority: 9999, Enabled: true,
		},
	}

	now := time.Now()
	for _, r := range defaults {
		r.CreatedAt = now
		r.UpdatedAt = now
		m.rules[r.ID] = r
	}

	m.groups["sg_default"] = &SecurityGroup{
		ID: "sg_default", Name: "Default",
		Description: "Default security group with basic rules",
		Rules:       []string{"rule_default_allow_ssh", "rule_default_allow_http", "rule_default_allow_https", "rule_default_allow_webrtc", "rule_default_deny_all"},
		CreatedAt:   now, UpdatedAt: now,
	}
}

func (m *Manager) CreateRule(rule *Rule) *Rule {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("rule_%d_%x", m.nextID, time.Now().UnixNano())
	m.nextID++

	rule.ID = id
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	if rule.Action == "" {
		rule.Action = ActionAllow
	}
	if rule.Direction == "" {
		rule.Direction = DirectionInbound
	}
	if rule.Protocol == "" {
		rule.Protocol = ProtocolTCP
	}
	if rule.CIDR == "" {
		rule.CIDR = "0.0.0.0/0"
	}
	if rule.Priority == 0 {
		rule.Priority = 500
	}

	m.rules[id] = rule
	m.logger.Printf("firewall rule created: %s (%s %s)", id[:8], rule.Direction, rule.Protocol)
	return rule
}

func (m *Manager) ListRules(userID string) []*Rule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Rule
	now := time.Now()
	for _, r := range m.rules {
		ruleCopy := *r
		ruleCopy.CreatedAt = now
		ruleCopy.UpdatedAt = now
		_ = ruleCopy
		result = append(result, r)
	}
	return result
}

func (m *Manager) GetRule(id string) *Rule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.rules[id]
}

func (m *Manager) UpdateRule(id string, updates map[string]any) (*Rule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, ok := m.rules[id]
	if !ok {
		return nil, fmt.Errorf("rule not found")
	}

	if v, ok := updates["name"].(string); ok {
		rule.Name = v
	}
	if v, ok := updates["description"].(string); ok {
		rule.Description = v
	}
	if v, ok := updates["enabled"].(bool); ok {
		rule.Enabled = v
	}
	if v, ok := updates["action"].(string); ok {
		rule.Action = Action(v)
	}
	if v, ok := updates["cidr"].(string); ok {
		rule.CIDR = v
	}
	rule.UpdatedAt = time.Now()

	return rule, nil
}

func (m *Manager) DeleteRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.rules[id]; !ok {
		return fmt.Errorf("rule not found")
	}
	delete(m.rules, id)

	for _, g := range m.groups {
		for i, rID := range g.Rules {
			if rID == id {
				g.Rules = append(g.Rules[:i], g.Rules[i+1:]...)
				break
			}
		}
	}

	return nil
}

func (m *Manager) CreateGroup(group *SecurityGroup) *SecurityGroup {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("sg_%d_%x", m.nextID, time.Now().UnixNano())
	m.nextID++

	group.ID = id
	group.CreatedAt = time.Now()
	group.UpdatedAt = time.Now()
	if group.Rules == nil {
		group.Rules = []string{}
	}
	if group.VMIDs == nil {
		group.VMIDs = []string{}
	}

	m.groups[id] = group
	return group
}

func (m *Manager) ListGroups(userID string) []*SecurityGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*SecurityGroup
	for _, g := range m.groups {
		result = append(result, g)
	}
	return result
}

func (m *Manager) AssignGroupToVM(groupID, vmID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, ok := m.groups[groupID]
	if !ok {
		return fmt.Errorf("group not found")
	}

	for _, v := range group.VMIDs {
		if v == vmID {
			return nil
		}
	}
	group.VMIDs = append(group.VMIDs, vmID)
	group.UpdatedAt = time.Now()
	return nil
}

func (m *Manager) RemoveGroupFromVM(groupID, vmID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, ok := m.groups[groupID]
	if !ok {
		return fmt.Errorf("group not found")
	}

	for i, v := range group.VMIDs {
		if v == vmID {
			group.VMIDs = append(group.VMIDs[:i], group.VMIDs[i+1:]...)
			group.UpdatedAt = time.Now()
			return nil
		}
	}
	return nil
}

func (m *Manager) Evaluate(vmID string) []*Rule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var rules []*Rule
	for _, g := range m.groups {
		for _, v := range g.VMIDs {
			if v == vmID {
				for _, rID := range g.Rules {
					if r, ok := m.rules[rID]; ok && r.Enabled {
						rules = append(rules, r)
					}
				}
			}
		}
	}

	sortRules(rules)
	return rules
}

func sortRules(rules []*Rule) {
	for i := 0; i < len(rules); i++ {
		for j := i + 1; j < len(rules); j++ {
			if rules[i].Priority > rules[j].Priority {
				rules[i], rules[j] = rules[j], rules[i]
			}
		}
	}
}

func (m *Manager) HandleRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		rules := m.ListRules("")
		writeJSON(w, http.StatusOK, map[string]any{
			"rules": rules,
			"count": len(rules),
		})
	case "POST":
		var rule Rule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		result := m.CreateRule(&rule)
		writeJSON(w, http.StatusCreated, result)
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (m *Manager) HandleRuleOps(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	id := stringsTrimPrefix(path, "/firewall/rules/")
	if idx := stringsIndexByte(id, '/'); idx >= 0 {
		id = id[:idx]
	}

	switch r.Method {
	case "GET":
		rule := m.GetRule(id)
		if rule == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "rule not found"})
			return
		}
		writeJSON(w, http.StatusOK, rule)

	case "PUT":
		var updates map[string]any
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		rule, err := m.UpdateRule(id, updates)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, rule)

	case "DELETE":
		if err := m.DeleteRule(id); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (m *Manager) HandleGroups(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		groups := m.ListGroups("")
		writeJSON(w, http.StatusOK, map[string]any{
			"groups": groups,
			"count":  len(groups),
		})
	case "POST":
		var group SecurityGroup
		if err := json.NewDecoder(r.Body).Decode(&group); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		result := m.CreateGroup(&group)
		writeJSON(w, http.StatusCreated, result)
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (m *Manager) HandleGroupAssign(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		GroupID string `json:"groupId"`
		VMID    string `json:"vmId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if err := m.AssignGroupToVM(req.GroupID, req.VMID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "assigned"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}

func stringsTrimPrefix(s, prefix string) string {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}

func stringsIndexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
