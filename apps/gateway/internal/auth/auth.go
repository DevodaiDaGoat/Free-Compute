package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/freecompute/free-compute/apps/gateway/internal/database"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserExists       = errors.New("user already exists")
	ErrUserNotFound     = errors.New("user not found")
	ErrInvalidPassword  = errors.New("invalid password")
	ErrInvalidToken     = errors.New("invalid or expired token")
	ErrQuotaExceeded    = errors.New("storage quota exceeded")
)

type User struct {
	ID             string    `json:"id"`
	Email          string    `json:"email"`
	PasswordHash   string    `json:"-"`
	DisplayName    string    `json:"displayName"`
	Verified       bool      `json:"verified"`
	Credits        int64     `json:"credits"`
	StorageUsed    int64     `json:"storageUsed"`
	StorageQuota   int64     `json:"storageQuota"`
	TailscaleIP    string    `json:"tailscaleIp,omitempty"`
	TailscaleProxy string    `json:"tailscaleProxy,omitempty"`
	Preferences    json.RawMessage `json:"preferences"`
	Role           string    `json:"role"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

const (
	RoleUser      = 0
	RoleModerator = 1
	RoleAdmin     = 2

	tokenCleanupThreshold = 10000
)

func roleLevel(role string) int {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "admin":
		return RoleAdmin
	case "moderator":
		return RoleModerator
	default:
		return RoleUser
	}
}

// RoleLevelOf is the exported form of roleLevel for use by other packages.
func RoleLevelOf(role string) int { return roleLevel(role) }

type TokenPair struct {
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken,omitempty"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

type AuthManager struct {
	logger         *log.Logger
	mu             sync.RWMutex
	users          map[string]*User
	emails         map[string]string
	tokens         map[string]string
	refreshTokens  map[string]string
	tokenExpiry    map[string]time.Time
	validationCount atomic.Int64
	jwtSecret      []byte
	db             *database.DB
}

func NewAuthManager(logger *log.Logger) *AuthManager {
	return newAuthManager(logger, nil)
}

func NewAuthManagerWithDB(logger *log.Logger, db *database.DB) *AuthManager {
	m := newAuthManager(logger, db)
	m.loadFromDB()
	return m
}

func newAuthManager(logger *log.Logger, db *database.DB) *AuthManager {
	if logger == nil {
		logger = log.Default()
	}
	secret := make([]byte, 64)
	if _, err := rand.Read(secret); err != nil {
		logger.Printf("warning: failed to generate random jwt secret: %v", err)
	}
	return &AuthManager{
		logger:         logger,
		users:          make(map[string]*User),
		emails:         make(map[string]string),
		tokens:         make(map[string]string),
		refreshTokens:  make(map[string]string),
		tokenExpiry:    make(map[string]time.Time),
		jwtSecret:      secret,
		db:             db,
	}
}

func (m *AuthManager) loadFromDB() {
	if m.db == nil {
		return
	}
	rows, err := m.db.ListUsers()
	if err != nil {
		m.logger.Printf("load users from db: %v", err)
		return
	}
	for _, row := range rows {
		u := &User{
			ID:           row.ID,
			Email:        row.Email,
			PasswordHash: row.PasswordHash,
			DisplayName:  row.DisplayName,
			Verified:     row.Verified,
			Credits:      row.Credits,
			StorageUsed:  row.StorageUsed,
			StorageQuota: row.StorageQuota,
			TailscaleIP:  row.TailscaleIP,
			Preferences:  json.RawMessage(row.Preferences),
			Role:         row.Role,
			CreatedAt:    row.CreatedAt,
			UpdatedAt:    row.UpdatedAt,
		}
		m.users[u.ID] = u
		m.emails[u.Email] = u.ID
	}
	m.logger.Printf("loaded %d users from database", len(rows))
}

func (m *AuthManager) Register(email, password, displayName string) (*User, *TokenPair, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	email = strings.ToLower(strings.TrimSpace(email))
	if _, exists := m.emails[email]; exists {
		return nil, nil, ErrUserExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, fmt.Errorf("hash password: %w", err)
	}

	userID := generateID("user_")
	now := time.Now()
	user := &User{
		ID:           userID,
		Email:        email,
		PasswordHash: string(hash),
		DisplayName:  displayName,
		Verified:     true,
		StorageQuota: 10 * 1024 * 1024 * 1024,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// Persist to DB FIRST — if DB write fails we must not leave a phantom
	// entry in the in-memory maps, since that would (a) be lost on restart
	// and (b) block subsequent Register attempts with the same email because
	// m.emails still holds the reservation. Reserve the maps only after the
	// row is durable.
	if m.db != nil {
		if err := m.db.CreateUser(&database.UserRow{
			ID:           userID,
			Email:        email,
			PasswordHash: string(hash),
			DisplayName:  displayName,
			Verified:     true,
			StorageQuota: 10 * 1024 * 1024 * 1024,
			Role:         user.Role,
			CreatedAt:    now,
			UpdatedAt:    now,
		}); err != nil {
			m.logger.Printf("persist user to db: %v", err)
			return nil, nil, fmt.Errorf("persist user: %w", err)
		}
	}

	m.users[userID] = user
	m.emails[email] = userID

	m.cleanup()
	tokens := m.generateTokens(userID)
	m.logger.Printf("registered user %s (%s)", userID, email)
	return user, tokens, nil
}

func (m *AuthManager) Login(email, password string) (*User, *TokenPair, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	email = strings.ToLower(strings.TrimSpace(email))
	userID, exists := m.emails[email]
	if !exists {
		return nil, nil, ErrUserNotFound
	}
	user, userOK := m.users[userID]
	if !userOK || user == nil {
		// emails map holds a stale reservation for a user that no longer
		// exists in the users map (e.g. DB load raced with an eviction).
		// Previously the next line dereferenced user.PasswordHash → panic.
		delete(m.emails, email)
		return nil, nil, ErrUserNotFound
	}
	m.cleanup()

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, nil, ErrInvalidPassword
	}

	tokens := m.generateTokens(userID)
	return user, tokens, nil
}

func (m *AuthManager) ValidateToken(token string) (*User, error) {
	m.mu.RLock()
	userID, exists := m.tokens[token]
	exp, hasExp := m.tokenExpiry[token]
	user, userOK := m.users[userID]
	m.mu.RUnlock()

	if !exists {
		return nil, ErrInvalidToken
	}

	if hasExp && time.Now().After(exp) {
		m.mu.Lock()
		delete(m.tokens, token)
		delete(m.refreshTokens, token)
		delete(m.tokenExpiry, token)
		m.mu.Unlock()
		return nil, ErrInvalidToken
	}

	if !userOK {
		return nil, ErrUserNotFound
	}

	if m.validationCount.Add(1)%1000 == 0 {
		m.mu.Lock()
		m.cleanup()
		m.mu.Unlock()
	}

	return user, nil
}

func (m *AuthManager) GetUser(userID string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	user, exists := m.users[userID]
	if !exists {
		return nil, ErrUserNotFound
	}
	return user, nil
}

func (m *AuthManager) UpdateUser(userID string, update func(*User)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	user, exists := m.users[userID]
	if !exists {
		return ErrUserNotFound
	}
	update(user)
	user.UpdatedAt = time.Now()
	m.persistUser(user)
	return nil
}

func (m *AuthManager) persistUser(user *User) {
	if m.db == nil {
		return
	}
	if err := m.db.UpdateUser(&database.UserRow{
		ID:           user.ID,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		DisplayName:  user.DisplayName,
		AvatarURL:    "",
		TailscaleIP:  user.TailscaleIP,
		TailscaleKey: "",
		StorageUsed:  user.StorageUsed,
		StorageQuota: user.StorageQuota,
		Credits:      user.Credits,
		Role:         user.Role,
		Verified:     user.Verified,
		Banned:       false,
		UpdatedAt:    user.UpdatedAt,
	}); err != nil {
		m.logger.Printf("update user in db: %v", err)
	}
}

func (m *AuthManager) AllocateTailscaleIP(userID string) string {
	m.mu.Lock()
	user, exists := m.users[userID]
	if !exists {
		m.mu.Unlock()
		return ""
	}
	if user.TailscaleIP != "" {
		out := user.TailscaleIP
		m.mu.Unlock()
		return out
	}
	// Use crypto/rand for octet uniqueness — three UnixNano%255 calls in the
	// same nanosecond collapse to the same digit, so different users could get
	// identical IPs. Also persist the allocation so a restart doesn't lose it.
	var octets [3]byte
	if _, err := rand.Read(octets[:]); err != nil {
		// Fallback that at least varies across users in the same instant.
		now := time.Now().UnixNano()
		octets[0] = byte(now)
		octets[1] = byte(now >> 8)
		octets[2] = byte(now >> 16)
	}
	ip := fmt.Sprintf("100.%d.%d.%d", int(octets[0])%255, int(octets[1])%255, int(octets[2])%255)
	user.TailscaleIP = ip
	user.TailscaleProxy = "relay"
	user.UpdatedAt = time.Now()
	m.persistUser(user)
	m.mu.Unlock()
	return ip
}

func (m *AuthManager) CheckStorageQuota(userID string, additionalBytes int64) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	user, exists := m.users[userID]
	if !exists {
		return ErrUserNotFound
	}
	if user.StorageUsed+additionalBytes > user.StorageQuota {
		return ErrQuotaExceeded
	}
	return nil
}

func (m *AuthManager) AddStorageUsed(userID string, bytes int64) {
	m.mu.Lock()
	user, exists := m.users[userID]
	if !exists {
		m.mu.Unlock()
		return
	}
	user.StorageUsed += bytes
	if user.StorageUsed < 0 {
		user.StorageUsed = 0
	}
	user.UpdatedAt = time.Now()
	m.mu.Unlock()
	// Persist outside the lock — otherwise a DB stall would block every reader
	// of AuthManager.users. persistUser is a no-op when m.db is nil.
	m.persistUser(user)
}

func (m *AuthManager) ListAllUsers(fn func(*User)) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, user := range m.users {
		fn(user)
	}
}

func (m *AuthManager) DeleteUser(userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	user, exists := m.users[userID]
	if !exists {
		return ErrUserNotFound
	}
	delete(m.emails, user.Email)
	delete(m.users, userID)
	m.logger.Printf("deleted user %s (%s)", userID, user.Email)
	return nil
}

func (m *AuthManager) cleanup() {
	now := time.Now()
	for token, exp := range m.tokenExpiry {
		if now.After(exp) {
			delete(m.tokens, token)
			delete(m.refreshTokens, token)
			delete(m.tokenExpiry, token)
		}
	}
}

func (m *AuthManager) generateTokens(userID string) *TokenPair {
	expiresAt := time.Now().Add(24 * time.Hour)
	accessToken := generateJWT(userID, m.jwtSecret, expiresAt)
	refreshToken := generateID("ref_")
	m.tokens[accessToken] = userID
	m.refreshTokens[refreshToken] = userID
	m.tokenExpiry[accessToken] = expiresAt
	m.tokenExpiry[refreshToken] = expiresAt
	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
	}
}

func generateJWT(userID string, secret []byte, expiresAt time.Time) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(
		fmt.Sprintf(`{"sub":"%s","exp":%d,"iat":%d}`, userID, expiresAt.Unix(), time.Now().Unix()),
	))
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(header + "." + payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return header + "." + payload + "." + sig
}

func generateID(prefix string) string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return prefix + fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return prefix + hex.EncodeToString(b)
}

type AuthHandler struct {
	auth *AuthManager
	db   *database.DB
}

func NewAuthHandler(auth *AuthManager, db *database.DB) *AuthHandler {
	return &AuthHandler{auth: auth, db: db}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"displayName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	if req.Email == "" || req.Password == "" {
		http.Error(w, `{"error":"email and password required"}`, http.StatusBadRequest)
		return
	}
	if len(req.Password) < 8 {
		http.Error(w, `{"error":"password must be at least 8 characters"}`, http.StatusBadRequest)
		return
	}
	if len(req.Email) > 254 || !strings.Contains(req.Email, "@") {
		http.Error(w, `{"error":"invalid email"}`, http.StatusBadRequest)
		return
	}

	user, tokens, err := h.auth.Register(req.Email, req.Password, req.DisplayName)
	if err != nil {
		if errors.Is(err, ErrUserExists) {
			http.Error(w, `{"error":"email already registered"}`, http.StatusConflict)
		} else {
			http.Error(w, `{"error":"registration failed"}`, http.StatusInternalServerError)
		}
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"user":   user,
		"tokens": tokens,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	user, tokens, err := h.auth.Login(req.Email, req.Password)
	if err != nil {
		// Return the same status/message for both wrong email and wrong password
		// to prevent user enumeration.
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user":   user,
		"tokens": tokens,
	})
}

func (h *AuthHandler) Profile(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *AuthHandler) PublicProfile(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"gateway": "freecompute-universal-proxy",
		"version": "0.1.0",
	})
}

func (h *AuthHandler) AllocateIP(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	ip := h.auth.AllocateTailscaleIP(user.ID)
	writeJSON(w, http.StatusOK, map[string]string{"tailscaleIp": ip})
}

func (h *AuthHandler) Preferences(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodGet:
		prefs, err := h.auth.GetPreferences(user.ID)
		if err != nil {
			http.Error(w, `{"error":"could not load preferences"}`, http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"preferences": prefs})
	case http.MethodPut:
		var req struct {
			Preferences json.RawMessage `json:"preferences"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
			return
		}
		if err := h.auth.SavePreferences(user.ID, req.Preferences); err != nil {
			if errors.Is(err, ErrUserNotFound) {
				http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
			} else {
				http.Error(w, `{"error":"could not save preferences"}`, http.StatusInternalServerError)
			}
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (h *AuthHandler) RequestPersonalizationSync(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if h.db == nil {
		http.Error(w, `{"error":"database unavailable"}`, http.StatusInternalServerError)
		return
	}
	id, err := h.db.CreateSyncRequest(user.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":     id,
		"status": "pending",
	})
}

func AuthMiddleware(auth *AuthManager, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r.Header.Get("Authorization"))
		if token != "" {
			user, err := auth.ValidateToken(token)
			if err == nil {
				r = r.WithContext(newContextWithUser(r.Context(), user))
			}
		}
		next(w, r)
	}
}

func RequireAuth(auth *AuthManager, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r.Header.Get("Authorization"))
		if token == "" {
			http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
			return
		}
		user, err := auth.ValidateToken(token)
		if err != nil {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}
		r = r.WithContext(newContextWithUser(r.Context(), user))
		next(w, r)
	}
}

func UserFromContext(r *http.Request) *User {
	if user, ok := r.Context().Value(ctxKeyUser).(*User); ok {
		return user
	}
	return nil
}

func userFromContext(r *http.Request) *User {
	return UserFromContext(r)
}

// GetPreferences returns the raw preferences JSON blob for a user. The blob is an
// opaque JSON object; it may contain a "browsingMode" key with value
// "speed" | "privacy" | "casual" which the gateway reverse proxy reads to select
// a BrowsingPolicy (see internal/browsing). Unknown keys are ignored.
func (m *AuthManager) GetPreferences(userID string) (json.RawMessage, error) {
	user, err := m.GetUser(userID)
	if err != nil {
		return nil, err
	}
	if user.Preferences == nil {
		return json.RawMessage("{}"), nil
	}
	return user.Preferences, nil
}

// SavePreferences persists the raw preferences JSON blob for a user. The blob is
// validated for correctness/limits only; the proxy interprets keys such as
// "browsingMode" ("speed" | "privacy" | "casual") at request time.
func (m *AuthManager) SavePreferences(userID string, blob json.RawMessage) error {
	if !json.Valid(blob) {
		return fmt.Errorf("invalid preferences json")
	}
	if len(blob) > 64*1024 {
		return fmt.Errorf("preferences exceed 64 KB")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	user, exists := m.users[userID]
	if !exists {
		return ErrUserNotFound
	}
	user.Preferences = blob
	user.UpdatedAt = time.Now()
	if m.db != nil {
		if err := m.db.SetUserPreferences(userID, []byte(blob)); err != nil {
			return err
		}
	}
	return nil
}

func (m *AuthManager) SetRole(userID, role string) error {
	role = strings.ToLower(strings.TrimSpace(role))
	switch role {
	case "user", "moderator", "admin":
	default:
		return fmt.Errorf("invalid role: %s", role)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	user, exists := m.users[userID]
	if !exists {
		return ErrUserNotFound
	}
	user.Role = role
	user.UpdatedAt = time.Now()
	if m.db != nil {
		if err := m.db.SetUserRole(userID, role); err != nil {
			return err
		}
	}
	return nil
}

func RequireRole(min int, auth *AuthManager, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r.Header.Get("Authorization"))
		if token == "" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		user, err := auth.ValidateToken(token)
		if err != nil {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}
		if roleLevel(user.Role) < min {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		r = r.WithContext(newContextWithUser(r.Context(), user))
		next(w, r)
	}
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
