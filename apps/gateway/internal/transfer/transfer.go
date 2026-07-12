package transfer

import (
	cryptorand "crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"log"
	"sync"
	"time"
)

const (
	maxFileSize       = 10 * 1024 * 1024 * 1024 // 10GB max file size
	chunkSize         = 64 * 1024 // 64KB chunks
	maxConcurrentTransfers = 10
)

type TransferManager struct {
	logger         *log.Logger
	transfers      map[string]*FileTransfer
	transfersMutex sync.RWMutex
	limiter        chan struct{}
}

type FileTransfer struct {
	ID           string
	SessionID    string
	Direction    TransferDirection // 'upload' or 'download'
	Filename     string
	Size         int64
	ContentType  string
	State        TransferState
	BytesTransferred int64
	StartedAt    time.Time
	CompletedAt  *time.Time
	Error        string
	Metadata     map[string]string
	// limiterReleased guards against over-release when Complete → Cancel or
	// Complete → Fail is called on an already-terminal transfer. Every drain
	// of the shared limiter must be matched to exactly one CreateTransfer
	// slot acquisition, otherwise the pool leaks in either direction.
	limiterReleased bool
	Mutex        sync.RWMutex
}

type TransferDirection string

const (
	TransferDirectionUpload   TransferDirection = "upload"
	TransferDirectionDownload TransferDirection = "download"
)

type TransferState string

const (
	TransferStatePending   TransferState = "pending"
	TransferStateActive    TransferState = "active"
	TransferStateCompleted TransferState = "completed"
	TransferStateFailed    TransferState = "failed"
	TransferStateCancelled TransferState = "cancelled"
)

type ChunkRequest struct {
	TransferID  string
	ChunkIndex  int
	ChunkSize   int
	Offset      int64
	Data        []byte
	Checksum    string
}

type ChunkResponse struct {
	TransferID  string
	ChunkIndex  int
	Success     bool
	Error       string
}

type ClipboardData struct {
	MimeType string
	Data     string
	Timestamp time.Time
}

type ClipboardManager struct {
	logger    *log.Logger
	sessions  map[string]*SessionClipboard
	sessionsMutex sync.RWMutex
}

type SessionClipboard struct {
	SessionID   string
	Enabled     bool
	ReadAllowed bool
	WriteAllowed bool
	Data        *ClipboardData
	History     []ClipboardData
	MaxHistory  int
	Mutex       sync.RWMutex
}

func NewTransferManager(logger *log.Logger) *TransferManager {
	if logger == nil {
		logger = log.Default()
	}

	return &TransferManager{
		logger:    logger,
		transfers: make(map[string]*FileTransfer),
		limiter:   make(chan struct{}, maxConcurrentTransfers),
	}
}

func (m *TransferManager) CreateTransfer(sessionID string, direction TransferDirection, filename string, size int64, contentType string) (*FileTransfer, error) {
	// Validate BEFORE acquiring the limiter token — otherwise an oversize
	// request permanently leaks a concurrency slot on the "size too large"
	// error path.
	if size > maxFileSize {
		return nil, errors.New("file size exceeds maximum allowed")
	}
	if size < 0 {
		return nil, errors.New("file size must be non-negative")
	}
	// Zero-size transfers previously acquired a limiter slot but never released
	// it (UpdateProgress / ProcessChunk both guard on `Size > 0` when
	// auto-completing). After maxConcurrentTransfers such requests the pool
	// was permanently exhausted. Reject them outright — empty-file semantics
	// belong at the storage layer.
	if size == 0 {
		return nil, errors.New("file size must be greater than zero; use storage.Upload for empty files")
	}

	// Acquire limiter
	select {
	case m.limiter <- struct{}{}:
	default:
		return nil, errors.New("maximum concurrent transfers reached")
	}

	transfer := &FileTransfer{
		ID:           generateTransferID(),
		SessionID:    sessionID,
		Direction:    direction,
		Filename:     filename,
		Size:         size,
		ContentType:  contentType,
		State:        TransferStatePending,
		StartedAt:    time.Now(),
		Metadata:     make(map[string]string),
	}

	m.transfersMutex.Lock()
	m.transfers[transfer.ID] = transfer
	m.transfersMutex.Unlock()

	m.logger.Printf("created transfer %s (session=%s, direction=%s, filename=%s, size=%d)", 
		transfer.ID, sessionID, direction, filename, size)

	return transfer, nil
}

func (m *TransferManager) GetTransfer(transferID string) (*FileTransfer, error) {
	m.transfersMutex.RLock()
	defer m.transfersMutex.RUnlock()

	transfer, exists := m.transfers[transferID]
	if !exists {
		return nil, errors.New("transfer not found")
	}

	return transfer, nil
}

func (m *TransferManager) StartTransfer(transferID string) error {
	transfer, err := m.GetTransfer(transferID)
	if err != nil {
		return err
	}

	transfer.Mutex.Lock()
	defer transfer.Mutex.Unlock()

	if transfer.State != TransferStatePending {
		return errors.New("transfer is not in pending state")
	}

	transfer.State = TransferStateActive
	transfer.StartedAt = time.Now()

	m.logger.Printf("started transfer %s", transferID)

	return nil
}

func (m *TransferManager) UpdateProgress(transferID string, bytesTransferred int64) error {
	transfer, err := m.GetTransfer(transferID)
	if err != nil {
		return err
	}

	transfer.Mutex.Lock()
	defer transfer.Mutex.Unlock()

	transfer.BytesTransferred = bytesTransferred

	// Check if complete. Guard on state==Active so a caller sending progress
	// past the declared size doesn't drain the limiter twice. Size==0 is
	// rejected up front in CreateTransfer, so this branch only fires when the
	// full declared body has arrived.
	if transfer.State == TransferStateActive && transfer.Size > 0 && transfer.BytesTransferred >= transfer.Size {
		now := time.Now()
		transfer.State = TransferStateCompleted
		transfer.CompletedAt = &now
		m.releaseLimiterLocked(transfer)
		m.logger.Printf("completed transfer %s (bytes=%d)", transferID, bytesTransferred)
	}

	return nil
}

// releaseLimiterLocked drains one slot from the shared limiter iff the
// transfer has not already released. Must be called with transfer.Mutex held.
func (m *TransferManager) releaseLimiterLocked(transfer *FileTransfer) {
	if transfer.limiterReleased {
		return
	}
	transfer.limiterReleased = true
	select {
	case <-m.limiter:
	default:
	}
}

func (m *TransferManager) FailTransfer(transferID string, errorMsg string) error {
	transfer, err := m.GetTransfer(transferID)
	if err != nil {
		return err
	}

	transfer.Mutex.Lock()
	defer transfer.Mutex.Unlock()

	transfer.State = TransferStateFailed
	transfer.Error = errorMsg
	m.releaseLimiterLocked(transfer)

	m.logger.Printf("failed transfer %s: %s", transferID, errorMsg)

	return nil
}

func (m *TransferManager) CancelTransfer(transferID string) error {
	transfer, err := m.GetTransfer(transferID)
	if err != nil {
		return err
	}

	transfer.Mutex.Lock()
	defer transfer.Mutex.Unlock()

	transfer.State = TransferStateCancelled
	m.releaseLimiterLocked(transfer)

	m.logger.Printf("cancelled transfer %s", transferID)

	return nil
}

func (m *TransferManager) ProcessChunk(chunk *ChunkRequest) (*ChunkResponse, error) {
	transfer, err := m.GetTransfer(chunk.TransferID)
	if err != nil {
		return &ChunkResponse{
			TransferID: chunk.TransferID,
			ChunkIndex: chunk.ChunkIndex,
			Success:    false,
			Error:      "transfer not found",
		}, nil
	}

	transfer.Mutex.Lock()
	defer transfer.Mutex.Unlock()

	if transfer.State != TransferStateActive {
		return &ChunkResponse{
			TransferID: chunk.TransferID,
			ChunkIndex: chunk.ChunkIndex,
			Success:    false,
			Error:      "transfer is not active",
		}, nil
	}

	// Cap chunk-accumulated total at Size so a misbehaving client can't grow
	// BytesTransferred without bound. Reject the individual chunk if the
	// running total would exceed the declared size.
	newTotal := transfer.BytesTransferred + int64(len(chunk.Data))
	if transfer.Size > 0 && newTotal > transfer.Size {
		return &ChunkResponse{
			TransferID: chunk.TransferID,
			ChunkIndex: chunk.ChunkIndex,
			Success:    false,
			Error:      "chunk exceeds declared transfer size",
		}, nil
	}
	transfer.BytesTransferred = newTotal

	// Auto-complete on final chunk and release the concurrency slot the
	// creator acquired. UpdateProgress had this branch; ProcessChunk did not,
	// so chunk-based uploads permanently leaked one slot per transfer and
	// after maxConcurrentTransfers uploads every new one failed.
	if transfer.Size > 0 && transfer.BytesTransferred >= transfer.Size && transfer.State == TransferStateActive {
		transfer.State = TransferStateCompleted
		now := time.Now()
		transfer.CompletedAt = &now
		m.releaseLimiterLocked(transfer)
	}

	return &ChunkResponse{
		TransferID: chunk.TransferID,
		ChunkIndex: chunk.ChunkIndex,
		Success:    true,
	}, nil
}

func (m *TransferManager) GetSessionTransfers(sessionID string) []*FileTransfer {
	m.transfersMutex.RLock()
	defer m.transfersMutex.RUnlock()

	transfers := make([]*FileTransfer, 0)
	for _, transfer := range m.transfers {
		if transfer.SessionID == sessionID {
			transfers = append(transfers, transfer)
		}
	}

	return transfers
}

func NewClipboardManager(logger *log.Logger) *ClipboardManager {
	if logger == nil {
		logger = log.Default()
	}

	return &ClipboardManager{
		logger:   logger,
		sessions: make(map[string]*SessionClipboard),
	}
}

func (m *ClipboardManager) RegisterSession(sessionID string, readAllowed bool, writeAllowed bool) *SessionClipboard {
	m.sessionsMutex.Lock()
	defer m.sessionsMutex.Unlock()

	session := &SessionClipboard{
		SessionID:    sessionID,
		Enabled:      true,
		ReadAllowed:  readAllowed,
		WriteAllowed: writeAllowed,
		History:     make([]ClipboardData, 0),
		MaxHistory:  10,
	}

	m.sessions[sessionID] = session
	m.logger.Printf("registered clipboard for session %s (read=%v, write=%v)", sessionID, readAllowed, writeAllowed)

	return session
}

func (m *ClipboardManager) UnregisterSession(sessionID string) {
	m.sessionsMutex.Lock()
	defer m.sessionsMutex.Unlock()

	if _, exists := m.sessions[sessionID]; exists {
		delete(m.sessions, sessionID)
		m.logger.Printf("unregistered clipboard for session %s", sessionID)
	}
}

func (m *ClipboardManager) GetSession(sessionID string) (*SessionClipboard, error) {
	m.sessionsMutex.RLock()
	defer m.sessionsMutex.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, errors.New("session clipboard not found")
	}

	return session, nil
}

func (m *ClipboardManager) Write(sessionID string, mimeType string, data string) error {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return err
	}

	session.Mutex.Lock()
	defer session.Mutex.Unlock()

	if !session.Enabled || !session.WriteAllowed {
		return errors.New("clipboard write not allowed")
	}

	// Create new clipboard data
	clipboardData := &ClipboardData{
		MimeType:  mimeType,
		Data:      data,
		Timestamp: time.Now(),
	}

	// Update current data
	session.Data = clipboardData

	// Add to history
	session.History = append(session.History, *clipboardData)
	if len(session.History) > session.MaxHistory {
		session.History = session.History[len(session.History)-session.MaxHistory:]
	}

	m.logger.Printf("clipboard write for session %s (mime=%s, size=%d)", sessionID, mimeType, len(data))

	return nil
}

func (m *ClipboardManager) Read(sessionID string) (*ClipboardData, error) {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	session.Mutex.RLock()
	defer session.Mutex.RUnlock()

	if !session.Enabled || !session.ReadAllowed {
		return nil, errors.New("clipboard read not allowed")
	}

	if session.Data == nil {
		return nil, errors.New("no clipboard data available")
	}

	m.logger.Printf("clipboard read for session %s (mime=%s, size=%d)", sessionID, session.Data.MimeType, len(session.Data.Data))

	return session.Data, nil
}

func (m *ClipboardManager) EncodeData(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func (m *ClipboardManager) DecodeData(encoded string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(encoded)
}

func (m *ClipboardManager) Clear(sessionID string) error {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return err
	}

	session.Mutex.Lock()
	defer session.Mutex.Unlock()

	session.Data = nil
	m.logger.Printf("clipboard cleared for session %s", sessionID)

	return nil
}

func generateTransferID() string {
	return time.Now().Format("20060102150405") + "_transfer_" + generateRandomString(8)
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	// Previously seeded every character from time.Now().UnixNano(), which is
	// invariant within a single call — so every byte was identical and two
	// transfers created in the same nanosecond produced identical IDs (map
	// key collision, silent overwrite of prior transfer state). crypto/rand
	// gives independent bytes and doesn't collide.
	b := make([]byte, length)
	if _, err := cryptorand.Read(b); err != nil {
		// Fall back to time-derived jitter with per-byte offset so at least
		// each byte differs. Should be effectively unreachable.
		nano := time.Now().UnixNano()
		for i := range b {
			b[i] = charset[(nano+int64(i)*17)&0x7fffffffffffffff%int64(len(charset))]
		}
		return string(b)
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b)
}

// Helper for reading file in chunks
func ReadFileChunk(reader io.Reader, offset int64, size int) ([]byte, error) {
	if seeker, ok := reader.(io.Seeker); ok {
		_, err := seeker.Seek(offset, io.SeekStart)
		if err != nil {
			return nil, err
		}
	}

	buf := make([]byte, size)
	n, err := io.ReadFull(reader, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, err
	}

	return buf[:n], nil
}