package tunnel

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"sync"
	"time"
)

const sessionRecordingMaxSize = 2 * 1024 * 1024 * 1024

type SessionRecorder struct {
	mu         sync.RWMutex
	sessions   map[string]*RecordingSession
	logger     *log.Logger
}

type RecordingSession struct {
	SessionID      string
	State          RecordingState
	StartedAt      time.Time
	EndedAt        *time.Time
	FilePath       string
	File           *os.File
	BytesWritten   int64
	Quality        RecordingQuality
	logger         *log.Logger
}

type RecordingState string

const (
	RecordingStateIdle    RecordingState = "idle"
	RecordingStateActive  RecordingState = "active"
	RecordingStateStopped RecordingState = "stopped"
)

type RecordingQuality string

const (
	RecordingQualityLow    RecordingQuality = "low"
	RecordingQualityMedium RecordingQuality = "medium"
	RecordingQualityHigh   RecordingQuality = "high"
)

type RecordingConfig struct {
	Quality RecordingQuality `json:"quality"`
	Format  string           `json:"format"`
}

func NewSessionRecorder(logger *log.Logger) *SessionRecorder {
	if logger == nil {
		logger = log.Default()
	}
	return &SessionRecorder{
		sessions: make(map[string]*RecordingSession),
		logger:   logger,
	}
}

func (r *SessionRecorder) StartRecording(sessionID string, config RecordingConfig) (*RecordingSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.sessions[sessionID]; exists {
		return nil, errors.New("session already being recorded")
	}
	if config.Quality == "" {
		config.Quality = RecordingQualityMedium
	}
	if config.Format == "" {
		config.Format = "mp4"
	}
	dir := os.TempDir()
	filePath := dir + "/freecompute-recording-" + sessionID + ".raw"
	file, err := os.Create(filePath)
	if err != nil {
		return nil, err
	}
	session := &RecordingSession{
		SessionID: sessionID,
		State:     RecordingStateActive,
		Quality:   config.Quality,
		FilePath:  filePath,
		File:      file,
		StartedAt: time.Now(),
		logger:    r.logger,
	}
	r.sessions[sessionID] = session
	r.logger.Printf("started recording session=%s quality=%s", sessionID, config.Quality)
	return session, nil
}

func (r *SessionRecorder) StopRecording(sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, exists := r.sessions[sessionID]
	if !exists {
		return errors.New("session not being recorded")
	}
	session.State = RecordingStateStopped
	now := time.Now()
	session.EndedAt = &now
	if session.File != nil {
		_ = session.File.Close()
		session.File = nil
	}
	r.logger.Printf("stopped recording session=%s bytes=%d", sessionID, session.BytesWritten)
	return nil
}

func (r *SessionRecorder) RecordFrame(sessionID string, frameData []byte) error {
	r.mu.RLock()
	session, exists := r.sessions[sessionID]
	r.mu.RUnlock()
	if !exists || session.State != RecordingStateActive {
		return nil
	}
	if session.File == nil {
		return errors.New("recording file not open")
	}
	n, err := session.File.Write(frameData)
	if n > 0 {
		session.BytesWritten += int64(n)
	}
	if session.BytesWritten > sessionRecordingMaxSize {
		_ = r.StopRecording(sessionID)
		return errors.New("recording max size exceeded")
	}
	return err
}

func (r *SessionRecorder) GetRecording(sessionID string) (*RecordingSession, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	session, exists := r.sessions[sessionID]
	if !exists {
		return nil, errors.New("no recording for session")
	}
	return session, nil
}

func (r *SessionRecorder) ListRecordings() []*RecordingSession {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*RecordingSession, 0, len(r.sessions))
	for _, session := range r.sessions {
		rec := *session
		result = append(result, &rec)
	}
	return result
}

func (r *SessionRecorder) DeleteRecording(sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, exists := r.sessions[sessionID]
	if !exists {
		return errors.New("no recording for session")
	}
	if session.File != nil {
		_ = session.File.Close()
	}
	if session.FilePath != "" {
		_ = os.Remove(session.FilePath)
	}
	delete(r.sessions, sessionID)
	r.logger.Printf("deleted recording session=%s", sessionID)
	return nil
}

func (r *SessionRecorder) BlockingWriter(sessionID string) (*Writer, error) {
	if _, err := r.GetRecording(sessionID); err != nil {
		return nil, err
	}
	return &Writer{recorder: r, sessionID: sessionID}, nil
}

type Writer struct {
	recorder  *SessionRecorder
	sessionID string
}

func (w *Writer) WriteFrame(p []byte) error {
	return w.recorder.RecordFrame(w.sessionID, p)
}

func (r *SessionRecorder) Stats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	active := 0
	var totalBytes int64
	for _, session := range r.sessions {
		if session.State == RecordingStateActive {
			active++
		}
		totalBytes += session.BytesWritten
	}
	return map[string]interface{}{
		"active":     active,
		"total":      len(r.sessions),
		"totalBytes": totalBytes,
		"maxBytes":   sessionRecordingMaxSize,
	}
}

func MarshalRecordingConfig(config RecordingConfig) ([]byte, error) {
	return json.Marshal(config)
}

func UnmarshalRecordingConfig(data []byte) (RecordingConfig, error) {
	var config RecordingConfig
	err := json.Unmarshal(data, &config)
	return config, err
}

func DefaultRecordingConfig() RecordingConfig {
	return RecordingConfig{
		Quality: RecordingQualityMedium,
		Format:  "mp4",
	}
}
