package admin

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/freecompute/free-compute/apps/gateway/internal/auth"
	"github.com/freecompute/free-compute/apps/gateway/internal/database"
	"github.com/freecompute/free-compute/apps/gateway/internal/security"
)

const (
	AdminEmail = "admin"
	AdminName  = "Administrator"
)

type AdminManager struct {
	logger         *log.Logger
	authManager    *auth.AuthManager
	detector       *security.SecurityDetector
	settings       map[string]string
	systemSettings SystemSettings
	mu             sync.RWMutex
	autoDetectDomain string
}

type SystemSettings struct {
	GatewayAddr           string `json:"gatewayAddr"`
	CDNHostname           string `json:"cdnHostname"`
	EdgeHostname          string `json:"edgeHostname"`
	APIHostname           string `json:"apiHostname"`
	AutoDetectDomain      string `json:"autoDetectDomain"`
	MaxUsers              int    `json:"maxUsers"`
	DefaultStorageQuota   int64  `json:"defaultStorageQuota"`
	ThreatDetection       bool   `json:"threatDetection"`
	AutoPauseOnThreat     bool   `json:"autoPauseOnThreat"`
	RequireAIReview       bool   `json:"requireAiReview"`
	MaxConcurrentSessions int    `json:"maxConcurrentSessions"`
	SessionTimeoutMinutes int    `json:"sessionTimeoutMinutes"`
}

func defaultSystemSettings() SystemSettings {
	return SystemSettings{
		MaxUsers:              1000,
		DefaultStorageQuota:   10 * 1024 * 1024 * 1024,
		ThreatDetection:       true,
		AutoPauseOnThreat:     true,
		RequireAIReview:       true,
		MaxConcurrentSessions: 100,
		SessionTimeoutMinutes: 60,
	}
}

func NewAdminManager(logger *log.Logger, authManager *auth.AuthManager, detector *security.SecurityDetector) *AdminManager {
	if logger == nil {
		logger = log.Default()
	}
	return &AdminManager{
		logger:         logger,
		authManager:    authManager,
		detector:       detector,
		settings:       make(map[string]string),
		systemSettings: defaultSystemSettings(),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func generatePassword(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())[:n]
	}
	for i := range b {
		b[i] = letters[int(b[i]) % len(letters)]
	}
	return string(b)
}

// adminPasswordFile is the location of the auto-generated admin password.
// Kept on disk so restarts don't lock the operator out.
func adminPasswordFile() string {
	if dir := os.Getenv("FREECOMPUTE_STATE_DIR"); dir != "" {
		return dir + string(os.PathSeparator) + ".admin-password"
	}
	return ".freecompute-admin-password"
}

func (m *AdminManager) SeedAdmin() {
	adminEmail := envOr("FREECOMPUTE_ADMIN_EMAIL", AdminEmail)
	adminPassword := envOr("FREECOMPUTE_ADMIN_PASSWORD", "")
	generated := false
	// Prefer a previously persisted generated password so restarts don't
	// re-generate and lock the operator out (Register would then collide
	// with the existing email and the seed would silently fail).
	if adminPassword == "" {
		if b, readErr := os.ReadFile(adminPasswordFile()); readErr == nil {
			adminPassword = strings.TrimSpace(string(b))
		}
	}
	if adminPassword == "" {
		adminPassword = generatePassword(16)
		generated = true
	}
	adminRole := envOr("FREECOMPUTE_ADMIN_ROLE", "admin")
	_, _, err := m.authManager.Login(adminEmail, adminPassword)
	if err == nil {
		m.logger.Printf("admin user already exists")
		return
	}

	user, tokens, err := m.authManager.Register(adminEmail, adminPassword, AdminName)
	if err != nil {
		m.logger.Printf("admin seed error: %v", err)
		return
	}

	m.authManager.UpdateUser(user.ID, func(u *auth.User) {
		u.Verified = true
		u.Role = adminRole
		u.StorageQuota = 1024 * 1024 * 1024 * 1024
		u.TailscaleIP = "100.0.0.1"
		u.TailscaleProxy = "admin"
	})

	_ = tokens

	// Persist the auto-generated password with 0600 permissions so restarts
	// keep working. Only write when we generated it — never overwrite an
	// operator-provided password from the env.
	if generated {
		if writeErr := os.WriteFile(adminPasswordFile(), []byte(adminPassword), 0600); writeErr != nil {
			m.logger.Printf("could not persist admin password to %s: %v (set FREECOMPUTE_ADMIN_PASSWORD to avoid this)", adminPasswordFile(), writeErr)
		}
	}

	// Only log the password when we generated it AND the operator opted in via
	// FREECOMPUTE_LOG_ADMIN_PASSWORD=1. Otherwise print a hint instead so it
	// does not end up in shipped log aggregators / journalctl by default.
	if generated && os.Getenv("FREECOMPUTE_LOG_ADMIN_PASSWORD") == "1" {
		m.logger.Printf("admin user seeded: %s / %s (set FREECOMPUTE_ADMIN_PASSWORD to control)", adminEmail, adminPassword)
	} else if generated {
		m.logger.Printf("admin user seeded: %s (password auto-generated to %s — set FREECOMPUTE_ADMIN_PASSWORD or FREECOMPUTE_LOG_ADMIN_PASSWORD=1 to see it)", adminEmail, adminPasswordFile())
	} else {
		m.logger.Printf("admin user seeded: %s", adminEmail)
	}
}

func (m *AdminManager) AutoDetectDomain(r *http.Request) string {
	// Fast path — read under RLock. If unset, upgrade to Lock to write.
	m.mu.RLock()
	cur := m.autoDetectDomain
	m.mu.RUnlock()
	if cur != "" {
		return cur
	}
	if r == nil || r.Host == "" {
		return ""
	}
	host := r.Host
	if idx := strings.Index(host, ":"); idx > 0 {
		host = host[:idx]
	}
	if host == "" || host == "localhost" || host == "127.0.0.1" || host == "0.0.0.0" {
		return ""
	}
	m.mu.Lock()
	// Recheck under write lock in case another goroutine wrote it first.
	if m.autoDetectDomain == "" {
		m.autoDetectDomain = host
		m.logger.Printf("auto-detected domain: %s", host)
	}
	out := m.autoDetectDomain
	m.mu.Unlock()
	return out
}

// SystemSettingsInput mirrors SystemSettings but uses pointers for booleans
// so a partial POST can leave them unchanged. Previously bool fields were
// always overwritten with their zero value, so a client sending only
// `{"maxUsers": 200}` silently disabled ThreatDetection, AutoPauseOnThreat
// and RequireAIReview.
type SystemSettingsInput struct {
	GatewayAddr           string `json:"gatewayAddr"`
	CDNHostname           string `json:"cdnHostname"`
	EdgeHostname          string `json:"edgeHostname"`
	APIHostname           string `json:"apiHostname"`
	AutoDetectDomain      string `json:"autoDetectDomain"`
	MaxUsers              int    `json:"maxUsers"`
	DefaultStorageQuota   int64  `json:"defaultStorageQuota"`
	ThreatDetection       *bool  `json:"threatDetection"`
	AutoPauseOnThreat     *bool  `json:"autoPauseOnThreat"`
	RequireAIReview       *bool  `json:"requireAiReview"`
	MaxConcurrentSessions int    `json:"maxConcurrentSessions"`
	SessionTimeoutMinutes int    `json:"sessionTimeoutMinutes"`
}

// UpdateSettings persists a subset of SystemSettings fields. Zero-valued
// numeric fields, empty strings, and nil bool pointers are treated as
// "leave unchanged" so a partial POST (e.g. { threatDetection: false })
// only touches the fields the caller specified.
func (m *AdminManager) UpdateSettings(in *SystemSettingsInput) {
	if in == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if in.GatewayAddr != "" {
		m.systemSettings.GatewayAddr = in.GatewayAddr
	}
	if in.CDNHostname != "" {
		m.systemSettings.CDNHostname = in.CDNHostname
	}
	if in.EdgeHostname != "" {
		m.systemSettings.EdgeHostname = in.EdgeHostname
	}
	if in.APIHostname != "" {
		m.systemSettings.APIHostname = in.APIHostname
	}
	if in.AutoDetectDomain != "" {
		m.autoDetectDomain = in.AutoDetectDomain
	}
	if in.MaxUsers > 0 {
		m.systemSettings.MaxUsers = in.MaxUsers
	}
	if in.DefaultStorageQuota > 0 {
		m.systemSettings.DefaultStorageQuota = in.DefaultStorageQuota
	}
	if in.MaxConcurrentSessions > 0 {
		m.systemSettings.MaxConcurrentSessions = in.MaxConcurrentSessions
	}
	if in.SessionTimeoutMinutes > 0 {
		m.systemSettings.SessionTimeoutMinutes = in.SessionTimeoutMinutes
	}
	if in.ThreatDetection != nil {
		m.systemSettings.ThreatDetection = *in.ThreatDetection
	}
	if in.AutoPauseOnThreat != nil {
		m.systemSettings.AutoPauseOnThreat = *in.AutoPauseOnThreat
	}
	if in.RequireAIReview != nil {
		m.systemSettings.RequireAIReview = *in.RequireAIReview
	}
}

func (m *AdminManager) GetSettings() *SystemSettings {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := m.systemSettings
	out.AutoDetectDomain = m.autoDetectDomain
	return &out
}

type AdminHandler struct {
	manager  *AdminManager
	auth     *auth.AuthManager
	detector *security.SecurityDetector
	db       *database.DB
}

func NewAdminHandler(manager *AdminManager, auth *auth.AuthManager, detector *security.SecurityDetector, db *database.DB) *AdminHandler {
	return &AdminHandler{
		manager:  manager,
		auth:     auth,
		detector: detector,
		db:       db,
	}
}

func (h *AdminHandler) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := auth.UserFromContext(r)
		// Gate by role, not email. Two callers with role="admin" pass; the
		// literal-email check locked the panel to a single seeded account and
		// would grant privileges to anyone who registered with email="admin"
		// before seeding ran (or with a case-variation).
		if user == nil || auth.RoleLevelOf(user.Role) < auth.RoleAdmin {
			http.Error(w, `{"error":"admin access required"}`, http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func (h *AdminHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	threats := h.detector.ListThreats(false)
	states := h.detector.ListVMStates()

	pausedCount := 0
	flaggedCount := 0
	for _, s := range states {
		switch s.State {
		case "paused":
			pausedCount++
		case "flagged":
			flaggedCount++
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"totalThreats":  len(threats),
		"pausedVMs":     pausedCount,
		"flaggedVMs":    flaggedCount,
		"activeThreats": h.detector.ThreatCount(),
		"settings":      h.manager.GetSettings(),
	})
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	type userInfo struct {
		ID           string `json:"id"`
		Email        string `json:"email"`
		DisplayName  string `json:"displayName"`
		StorageUsed  int64  `json:"storageUsed"`
		StorageQuota int64  `json:"storageQuota"`
		TailscaleIP  string `json:"tailscaleIp,omitempty"`
		CreatedAt    string `json:"createdAt"`
	}
	var users []userInfo
	h.auth.ListAllUsers(func(u *auth.User) {
		users = append(users, userInfo{
			ID:           u.ID,
			Email:        u.Email,
			DisplayName:  u.DisplayName,
			StorageUsed:  u.StorageUsed,
			StorageQuota: u.StorageQuota,
			TailscaleIP:  u.TailscaleIP,
			CreatedAt:    u.CreatedAt.Format(time.RFC3339),
		})
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{"users": users})
}

func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userId")
	if userID == "" {
		http.Error(w, `{"error":"userId required"}`, http.StatusBadRequest)
		return
	}
	if err := h.auth.DeleteUser(userID); err != nil {
		http.Error(w, `{"error":"could not delete user"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *AdminHandler) ListThreats(w http.ResponseWriter, r *http.Request) {
	resolved := r.URL.Query().Get("resolved") == "true"
	threats := h.detector.ListThreats(resolved)
	writeJSON(w, http.StatusOK, map[string]interface{}{"threats": threats})
}

func (h *AdminHandler) ReviewThreat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ThreatID string `json:"threatId"`
		Action   string `json:"action"`
		Resolved bool   `json:"resolved"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	user := auth.UserFromContext(r)
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	h.detector.ReviewThreat(req.ThreatID, user.ID, req.Resolved, req.Action)

	threat := h.detector.GetThreat(req.ThreatID)
	if threat != nil && req.Action == "resume" {
		h.detector.ResumeVM(threat.VMID)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "reviewed"})
}

func (h *AdminHandler) PauseVM(w http.ResponseWriter, r *http.Request) {
	vmID := r.URL.Query().Get("vmId")
	if vmID == "" {
		http.Error(w, `{"error":"vmId required"}`, http.StatusBadRequest)
		return
	}
	h.detector.PauseVM(vmID, "admin-requested")
	writeJSON(w, http.StatusOK, map[string]string{"status": "paused"})
}

func (h *AdminHandler) ResumeVM(w http.ResponseWriter, r *http.Request) {
	vmID := r.URL.Query().Get("vmId")
	if vmID == "" {
		http.Error(w, `{"error":"vmId required"}`, http.StatusBadRequest)
		return
	}
	h.detector.ResumeVM(vmID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "resumed"})
}

func (h *AdminHandler) Settings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, h.manager.GetSettings())
		return
	}
	if r.Method == http.MethodPost {
		var settings SystemSettingsInput
		if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
			http.Error(w, `{"error":"invalid settings"}`, http.StatusBadRequest)
			return
		}
		h.manager.UpdateSettings(&settings)
		writeJSON(w, http.StatusOK, h.manager.GetSettings())
		return
	}
	http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
}

func (h *AdminHandler) AutoDetect(w http.ResponseWriter, r *http.Request) {
	domain := h.manager.AutoDetectDomain(r)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"domain":      domain,
		"host":        r.Host,
		"remoteAddr":  r.RemoteAddr,
		"autoDetected": domain != "",
	})
}

func (h *AdminHandler) SetRole(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"userId"`
		Role   string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	if req.UserID == "" || req.Role == "" {
		http.Error(w, `{"error":"userId and role required"}`, http.StatusBadRequest)
		return
	}
	if err := h.auth.SetRole(req.UserID, req.Role); err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, auth.ErrUserNotFound):
			status = http.StatusNotFound
		case strings.Contains(err.Error(), "invalid role"):
			status = http.StatusBadRequest
		}
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), status)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *AdminHandler) ApprovePersonalizationSync(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RequestID string `json:"requestId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	if req.RequestID == "" {
		http.Error(w, `{"error":"requestId required"}`, http.StatusBadRequest)
		return
	}
	user := auth.UserFromContext(r)
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if h.db == nil {
		http.Error(w, `{"error":"database unavailable"}`, http.StatusInternalServerError)
		return
	}
	if err := h.db.ApproveSyncRequest(req.RequestID, user.ID); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
