package tunnel

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
)

const (
	checkpointHotTTL      = 5 * time.Minute
	checkpointWarmTTL     = 1 * time.Hour
	checkpointColdTTL     = 30 * 24 * time.Hour
	checkpointMaxSize     = 64 * 1024 * 1024
	checkpointCompactAfter = 1 * time.Hour
)

type SessionStateSnapshot struct {
	SessionID      string            `json:"sessionId"`
	Timestamp      time.Time         `json:"timestamp"`
	Delta          bool              `json:"delta,omitempty"`
	WindowState    map[string]string `json:"windowState,omitempty"`
	Clipboard      string            `json:"clipboard,omitempty"`
	InputSequence  uint32            `json:"inputSequence,omitempty"`
	Checksum       string            `json:"checksum"`
	Data           []byte            `json:"data,omitempty"`
}

type CheckpointStore struct {
	mu        sync.RWMutex
	snapshots map[string][]*SessionStateSnapshot
	maxSize   int64
	usedSize  int64
	logger    *log.Logger
}

func NewCheckpointStore(maxSize int64, logger *log.Logger) *CheckpointStore {
	if maxSize <= 0 {
		maxSize = checkpointMaxSize
	}
	if logger == nil {
		logger = log.Default()
	}
	return &CheckpointStore{
		snapshots: make(map[string][]*SessionStateSnapshot),
		maxSize:   maxSize,
		logger:    logger,
	}
}

func (s *CheckpointStore) Save(snapshot *SessionStateSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if snapshot.SessionID == "" {
		return errors.New("session id is required")
	}
	if snapshot.Checksum == "" {
		snapshot.Checksum = checksumSnapshot(snapshot)
	}
	snapshot.Timestamp = time.Now()

	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	compressed, err := compressCheckpoint(data)
	if err != nil {
		return err
	}
	snapshot.Data = compressed

	entrySize := int64(len(compressed))
	if entrySize > s.maxSize {
		return errors.New("checkpoint exceeds max size")
	}

	s.ensureCapacity(entrySize)

	snapshots := s.snapshots[snapshot.SessionID]
	if len(snapshots) > 0 && !snapshot.Delta {
		snapshots = []*SessionStateSnapshot{{}}
	} else if len(snapshots) > 0 && snapshot.Delta {
		last := snapshots[len(snapshots)-1]
		if last.Data != nil {
			s.usedSize -= int64(len(last.Data))
		}
	}
	snapshots = append(snapshots, snapshot)
	if len(snapshots) > 2 {
		if snapshots[0].Data != nil {
			s.usedSize -= int64(len(snapshots[0].Data))
		}
		snapshots = snapshots[len(snapshots)-2:]
	}
	s.snapshots[snapshot.SessionID] = snapshots
	s.usedSize += entrySize
	return nil
}

func (s *CheckpointStore) Load(sessionID string) (*SessionStateSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshots, ok := s.snapshots[sessionID]
	if !ok || len(snapshots) == 0 {
		return nil, errors.New("no checkpoint found")
	}
	snapshot := snapshots[len(snapshots)-1]
	decompressed, err := decompressCheckpoint(snapshot.Data)
	if err != nil {
		return nil, err
	}
	var result SessionStateSnapshot
	if err := json.Unmarshal(decompressed, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *CheckpointStore) History(sessionID string) ([]*SessionStateSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshots, ok := s.snapshots[sessionID]
	if !ok {
		return []*SessionStateSnapshot{}, nil
	}
	result := make([]*SessionStateSnapshot, 0, len(snapshots))
	for _, snap := range snapshots {
		decompressed, err := decompressCheckpoint(snap.Data)
		if err != nil {
			continue
		}
		var s2 SessionStateSnapshot
		if err := json.Unmarshal(decompressed, &s2); err != nil {
			continue
		}
		result = append(result, &s2)
	}
	return result, nil
}

func (s *CheckpointStore) Delete(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if snapshots, ok := s.snapshots[sessionID]; ok {
		for _, snap := range snapshots {
			if snap.Data != nil {
				s.usedSize -= int64(len(snap.Data))
			}
		}
	}
	delete(s.snapshots, sessionID)
}

func (s *CheckpointStore) Stats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	totalCheckpoints := 0
	for _, snaps := range s.snapshots {
		totalCheckpoints += len(snaps)
	}
	return map[string]interface{}{
		"sessions":          len(s.snapshots),
		"totalCheckpoints":  totalCheckpoints,
		"usedSizeMB":        float64(s.usedSize) / (1024 * 1024),
		"maxSizeMB":         float64(s.maxSize) / (1024 * 1024),
	}
}

func (s *CheckpointStore) ensureCapacity(needed int64) {
	for s.usedSize+needed > s.maxSize && len(s.snapshots) > 0 {
		var oldestSession string
		var oldestTime time.Time
		for sessionID, snaps := range s.snapshots {
			if len(snaps) > 0 && (oldestSession == "" || snaps[0].Timestamp.Before(oldestTime)) {
				oldestSession = sessionID
				oldestTime = snaps[0].Timestamp
			}
		}
		if oldestSession == "" {
			break
		}
		snaps := s.snapshots[oldestSession]
		if len(snaps) > 0 {
			if snaps[0].Data != nil {
				s.usedSize -= int64(len(snaps[0].Data))
			}
			snaps = snaps[1:]
			if len(snaps) == 0 {
				delete(s.snapshots, oldestSession)
			} else {
				s.snapshots[oldestSession] = snaps
			}
		}
	}
}

func (s *CheckpointStore) pruneExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for sessionID, snapshots := range s.snapshots {
		pruned := make([]*SessionStateSnapshot, 0, len(snapshots))
		for _, snap := range snapshots {
			if now.Sub(snap.Timestamp) < checkpointColdTTL {
				if !snap.Delta && now.Sub(snap.Timestamp) > checkpointWarmTTL {
					s.usedSize -= int64(len(snap.Data))
				} else {
					pruned = append(pruned, snap)
				}
			} else {
				s.usedSize -= int64(len(snap.Data))
			}
		}
		if len(pruned) == 0 {
			delete(s.snapshots, sessionID)
		} else {
			s.snapshots[sessionID] = pruned
		}
	}
}

func (s *CheckpointStore) StartPruning() {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			s.pruneExpired()
		}
	}()
}

func checksumSnapshot(s *SessionStateSnapshot) string {
	h := sha256.New()
	h.Write([]byte(s.SessionID))
	h.Write([]byte(s.Checksum))
	binary.Write(h, binary.BigEndian, uint64(s.Timestamp.UnixNano()))
	if s.Data != nil {
		h.Write(s.Data)
	}
	return hex.EncodeToString(h.Sum(nil))
}

var zstdEncoder *zstd.Encoder
var zstdDecoder *zstd.Decoder

func init() {
	enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedFastest))
	if err == nil {
		zstdEncoder = enc
	}
	dec, err := zstd.NewReader(nil)
	if err == nil {
		zstdDecoder = dec
	}
}

func compressCheckpoint(data []byte) ([]byte, error) {
	if zstdEncoder != nil {
		return zstdEncoder.EncodeAll(data, nil), nil
	}
	var buf bytes.Buffer
	w, err := zstd.NewWriter(&buf, zstd.WithEncoderLevel(zstd.SpeedFastest))
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decompressCheckpoint(data []byte) ([]byte, error) {
	if zstdDecoder != nil {
		return zstdDecoder.DecodeAll(data, nil)
	}
	r, err := zstd.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func GenerateCheckpointID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "ckpt-" + time.Now().Format("20060102150405")
	}
	return "ckpt-" + hex.EncodeToString(b)
}
