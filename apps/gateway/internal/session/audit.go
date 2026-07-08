package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

type AuditLogger struct {
	logger     *log.Logger
	entries    []AuditEntry
	entriesMutex sync.RWMutex
	maxEntries int
}

type AuditEntry struct {
	ID         string
	SessionID  string
	ActorUserID string
	Action     string
	IPAddress  string
	UserAgent  string
	Metadata   map[string]interface{}
	CreatedAt  time.Time
}

func NewAuditLogger(logger *log.Logger) *AuditLogger {
	if logger == nil {
		logger = log.Default()
	}

	return &AuditLogger{
		logger:     logger,
		entries:    make([]AuditEntry, 0),
		maxEntries: 10000, // Keep last 10,000 entries
	}
}

func (a *AuditLogger) Log(sessionID string, actorUserID string, action string, metadata map[string]interface{}) {
	entry := AuditEntry{
		ID:          generateAuditID(),
		SessionID:   sessionID,
		ActorUserID: actorUserID,
		Action:      action,
		Metadata:    metadata,
		CreatedAt:   time.Now(),
	}

	a.entriesMutex.Lock()
	defer a.entriesMutex.Unlock()

	a.entries = append(a.entries, entry)

	// Trim if too many entries
	if len(a.entries) > a.maxEntries {
		a.entries = a.entries[len(a.entries)-a.maxEntries:]
	}

	a.logger.Printf("audit: session=%s actor=%s action=%s", sessionID, actorUserID, action)
}

func (a *AuditLogger) GetSessionAuditLog(sessionID string) []AuditEntry {
	a.entriesMutex.RLock()
	defer a.entriesMutex.RUnlock()

	entries := make([]AuditEntry, 0)
	for _, entry := range a.entries {
		if entry.SessionID == sessionID {
			entries = append(entries, entry)
		}
	}

	return entries
}

func (a *AuditLogger) GetUserAuditLog(userID string) []AuditEntry {
	a.entriesMutex.RLock()
	defer a.entriesMutex.RUnlock()

	entries := make([]AuditEntry, 0)
	for _, entry := range a.entries {
		if entry.ActorUserID == userID {
			entries = append(entries, entry)
		}
	}

	return entries
}

func (a *AuditLogger) GetRecentEntries(limit int) []AuditEntry {
	a.entriesMutex.RLock()
	defer a.entriesMutex.RUnlock()

	if limit > len(a.entries) {
		limit = len(a.entries)
	}

	start := len(a.entries) - limit
	return a.entries[start:]
}

func (a *AuditLogger) ExportJSON(sessionID string) ([]byte, error) {
	entries := a.GetSessionAuditLog(sessionID)
	return json.MarshalIndent(entries, "", "  ")
}

func generateAuditID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return time.Now().Format("20060102150405") + "_" + fmt.Sprintf("%08x", time.Now().UnixNano())
	}
	return time.Now().Format("20060102150405") + "_" + hex.EncodeToString(b)
}