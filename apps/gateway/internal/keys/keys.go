package keys

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/freecompute/free-compute/apps/gateway/internal/auth"
)

type KeyType string

const (
	KeyTypeEd25519 KeyType = "ed25519"
	KeyTypeECDSA   KeyType = "ecdsa"
	KeyTypeRSA     KeyType = "rsa"
)

type SSHKey struct {
	ID          string    `json:"id"`
	UserID      string    `json:"userId"`
	Name        string    `json:"name"`
	PublicKey   string    `json:"publicKey"`
	Fingerprint string    `json:"fingerprint"`
	KeyType     KeyType   `json:"keyType"`
	KeySize     int       `json:"keySize"`
	LastUsed    time.Time `json:"lastUsed"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Manager struct {
	mu    sync.RWMutex
	keys  map[string][]*SSHKey
	logger *log.Logger
	nextID int
}

func NewManager(logger *log.Logger) *Manager {
	if logger == nil {
		logger = log.Default()
	}
	return &Manager{
		keys:   make(map[string][]*SSHKey),
		logger: logger,
		nextID: 1,
	}
}

func (m *Manager) Add(userID, name, publicKey string) (*SSHKey, error) {
	fingerprint, keyType, keySize, err := parsePublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid public key: %w", err)
	}

	m.mu.Lock()
	for _, k := range m.keys[userID] {
		if k.Fingerprint == fingerprint {
			m.mu.Unlock()
			return nil, fmt.Errorf("key already exists with fingerprint %s", fingerprint)
		}
	}

	id := fmt.Sprintf("key_%d_%x", m.nextID, time.Now().UnixNano())
	m.nextID++

	key := &SSHKey{
		ID:          id,
		UserID:      userID,
		Name:        name,
		PublicKey:   publicKey,
		Fingerprint: fingerprint,
		KeyType:     keyType,
		KeySize:     keySize,
		CreatedAt:   time.Now(),
	}

	m.keys[userID] = append(m.keys[userID], key)
	m.mu.Unlock()

	m.logger.Printf("SSH key added: %s (%s %d) for user %s", shortID(id, 8), keyType, keySize, shortID(userID, 8))
	return key, nil
}

func (m *Manager) List(userID string) []*SSHKey {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := m.keys[userID]
	result := make([]*SSHKey, len(keys))
	copy(result, keys)
	return result
}

func (m *Manager) Delete(id, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	keys := m.keys[userID]
	for i, k := range keys {
		if k.ID == id {
			m.keys[userID] = append(keys[:i], keys[i+1:]...)
			m.logger.Printf("SSH key deleted: %s", shortID(id, 8))
			return nil
		}
	}
	return fmt.Errorf("key not found")
}

func (m *Manager) Get(id, userID string) *SSHKey {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, k := range m.keys[userID] {
		if k.ID == id {
			return k
		}
	}
	return nil
}

func (m *Manager) GetAuthorizedKeys(userID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var keys []string
	for _, k := range m.keys[userID] {
		keys = append(keys, k.PublicKey)
	}
	return keys
}

func parsePublicKey(pubKey string) (fingerprint string, keyType KeyType, keySize int, err error) {
	pubKey = strings.TrimSpace(pubKey)

	block, _ := pem.Decode([]byte(pubKey))
	if block != nil {
		return parsePEMKey(block)
	}

	parts := strings.Fields(pubKey)
	if len(parts) < 2 {
		return "", "", 0, fmt.Errorf("invalid format")
	}

	keyTypeStr := parts[0]
	keyData := parts[1]

	data, err := decodeBase64(keyData)
	if err != nil {
		return "", "", 0, fmt.Errorf("invalid base64: %w", err)
	}

	hash := sha256.Sum256(data)
	// OpenSSH fingerprint format is unpadded base64 of the raw sha256 digest.
	fingerprint = "SHA256:" + base64.RawStdEncoding.EncodeToString(hash[:])

	switch {
	case strings.HasPrefix(keyTypeStr, "ssh-ed25519"):
		return fingerprint, KeyTypeEd25519, 256, nil
	case strings.HasPrefix(keyTypeStr, "ecdsa-sha2-"):
		return fingerprint, KeyTypeECDSA, 256, nil
	case strings.HasPrefix(keyTypeStr, "ssh-rsa"):
		return fingerprint, KeyTypeRSA, len(data) * 8, nil
	default:
		return fingerprint, KeyTypeRSA, len(data) * 8, nil
	}
}

func parsePEMKey(block *pem.Block) (string, KeyType, int, error) {
	switch block.Type {
	case "PUBLIC KEY":
		key, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return "", "", 0, fmt.Errorf("parse PKIX key: %w", err)
		}
		return parseCryptoKey(key)
	case "RSA PUBLIC KEY":
		key, err := x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return "", "", 0, fmt.Errorf("parse RSA key: %w", err)
		}
		data, _ := x509.MarshalPKIXPublicKey(key)
		hash := sha256.Sum256(data)
		return "SHA256:" + base64.RawStdEncoding.EncodeToString(hash[:]), KeyTypeRSA, key.Size() * 8, nil
	default:
		return "", "", 0, fmt.Errorf("unsupported PEM type: %s", block.Type)
	}
}

func parseCryptoKey(key crypto.PublicKey) (string, KeyType, int, error) {
	data, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return "", "", 0, fmt.Errorf("marshal key: %w", err)
	}
	hash := sha256.Sum256(data)
	fingerprint := "SHA256:" + base64.RawStdEncoding.EncodeToString(hash[:])

	switch k := key.(type) {
	case *rsa.PublicKey:
		return fingerprint, KeyTypeRSA, k.Size() * 8, nil
	case *ecdsa.PublicKey:
		return fingerprint, KeyTypeECDSA, k.Curve.Params().BitSize, nil
	case ed25519.PublicKey:
		return fingerprint, KeyTypeEd25519, 256, nil
	default:
		return fingerprint, KeyTypeRSA, 2048, nil
	}
}

// decodeBase64 decodes standard OpenSSH key material (base64, may be padded or
// unpadded). Prior implementation used fmt.Sscanf %x which parses hex, not
// base64, so every OpenSSH-format add silently produced an incorrect
// fingerprint.
func decodeBase64(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty key data")
	}
	if data, err := base64.StdEncoding.DecodeString(s); err == nil {
		return data, nil
	}
	// Some clients emit unpadded base64.
	if data, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return data, nil
	}
	return nil, fmt.Errorf("invalid base64 key data")
}

// resolveUserID picks the effective userID for a key operation. The caller's
// JWT-derived ID always wins to prevent IDOR (e.g. ?userId=someone-else). Only
// admins may target a different account, and only via an explicit query param.
func resolveUserID(r *http.Request) (string, bool) {
	user := auth.UserFromContext(r)
	if user == nil {
		return "", false
	}
	// Compare against the role-level ladder so any admin-or-higher role
	// (superadmin etc.) gets the override path. `user.Role == "admin"` as a
	// literal missed mixed-case values and future roles.
	if auth.RoleLevelOf(user.Role) >= auth.RoleAdmin {
		if q := r.URL.Query().Get("userId"); q != "" {
			return q, true
		}
	}
	return user.ID, true
}

func (m *Manager) HandleKeys(w http.ResponseWriter, r *http.Request) {
	userID, ok := resolveUserID(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	switch r.Method {
	case "GET":
		keys := m.List(userID)
		writeJSON(w, http.StatusOK, map[string]any{"keys": keys})

	case "POST":
		var req struct {
			Name      string `json:"name"`
			PublicKey string `json:"publicKey"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if req.Name == "" || req.PublicKey == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and publicKey required"})
			return
		}

		key, err := m.Add(userID, req.Name, req.PublicKey)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, key)

	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (m *Manager) HandleKeyOps(w http.ResponseWriter, r *http.Request) {
	userID, ok := resolveUserID(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	path := r.URL.Path
	id := strings.TrimPrefix(path, "/keys/")
	if idx := strings.IndexByte(id, '/'); idx >= 0 {
		id = id[:idx]
	}

	if id == "" {
		http.Error(w, `{"error":"key id required"}`, http.StatusBadRequest)
		return
	}

	switch r.Method {
	case "DELETE":
		if err := m.Delete(id, userID); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	case "GET":
		key := m.Get(id, userID)
		if key == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "key not found"})
			return
		}
		writeJSON(w, http.StatusOK, key)
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}

// shortID returns up to n leading chars of s for logging. Prior code did
// bare `id[:8]` which panics when the caller supplied a string shorter than
// the slice bound.
func shortID(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
