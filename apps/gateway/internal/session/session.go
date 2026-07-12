package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

type SessionManager struct {
	logger            *log.Logger
	sessions          map[string]*Session
	sessionsMutex     sync.RWMutex
	hostAllocator     *HostAllocator
	scheduler         *SessionScheduler
	auditLogger       *AuditLogger
	pendingApprovals  map[string]*PendingApproval
	approvalsMutex    sync.RWMutex
}

type Session struct {
	ID                string             `json:"id"`
	UserID            string             `json:"userId"`
	HostID            string             `json:"hostId"`
	VMID              string             `json:"vmId"`
	Type              SessionType        `json:"type"`
	Mode              SessionMode        `json:"mode"`
	ResourceClass     ResourceClass      `json:"resourceClass"`
	State             SessionState       `json:"state"`
	StreamProfile     StreamProfile      `json:"streamProfile"`
	Capabilities      SessionCapabilities `json:"capabilities"`
	Permissions       SessionPermissions  `json:"permissions"`
	NetworkQuality    *NetworkQualitySnapshot `json:"networkQuality,omitempty"`
	CreatedAt         time.Time          `json:"createdAt"`
	UpdatedAt         time.Time          `json:"updatedAt"`
	ExpiresAt         time.Time          `json:"expiresAt"`
	EndedAt           *time.Time         `json:"endedAt,omitempty"`
	ConnectionToken   string             `json:"connectionToken"`
	TemporaryLinks    []TemporaryAccessLink `json:"temporaryLinks"`
	Mutex             sync.RWMutex       `json:"-"`
}

type SessionType string

const (
	SessionTypeDesktop       SessionType = "desktop"
	SessionTypeGaming        SessionType = "gaming"
	SessionTypeRemoteSupport SessionType = "remote-support"
	SessionTypeHost          SessionType = "host"
)

type SessionMode string

const (
	SessionModeDesktop       SessionMode = "desktop"
	SessionModeDevelopment   SessionMode = "development"
	SessionModeGaming        SessionMode = "gaming"
	SessionModeRemoteSupport SessionMode = "remote-support"
)

type ResourceClass string

const (
	ResourceClassBasic       ResourceClass = "basic"
	ResourceClassStandard    ResourceClass = "standard"
	ResourceClassGaming      ResourceClass = "gaming"
	ResourceClassWorkstation ResourceClass = "workstation"
)

type SessionState string

const (
	SessionStateRequested      SessionState = "requested"
	SessionStateQueued         SessionState = "queued"
	SessionStateProvisioning   SessionState = "provisioning"
	SessionStateWaitingApproval SessionState = "waiting-for-approval"
	SessionStateConnecting     SessionState = "connecting"
	SessionStateActive         SessionState = "active"
	SessionStateReconnecting   SessionState = "reconnecting"
	SessionStateEnded          SessionState = "ended"
	SessionStateExpired        SessionState = "expired"
	SessionStateFailed         SessionState = "failed"
)

type EncodingMode string

const (
	EncodingModeSpeed    EncodingMode = "speed"
	EncodingModeQuality  EncodingMode = "quality"
	EncodingModeBalanced EncodingMode = "balanced"
)

type EncoderPreset string

const (
	EncoderPresetUltrafast EncoderPreset = "ultrafast"
	EncoderPresetSuperfast EncoderPreset = "superfast"
	EncoderPresetVeryfast  EncoderPreset = "veryfast"
	EncoderPresetFaster    EncoderPreset = "faster"
	EncoderPresetFast      EncoderPreset = "fast"
	EncoderPresetMedium    EncoderPreset = "medium"
	EncoderPresetSlow      EncoderPreset = "slow"
	EncoderPresetSlower    EncoderPreset = "slower"
	EncoderPresetVeryslow  EncoderPreset = "veryslow"
	EncoderPresetPlacebo   EncoderPreset = "placebo"
)

type EncoderConfig struct {
	Mode          EncodingMode  `json:"mode"`
	Preset        EncoderPreset `json:"preset"`
	CodecProfile  string        `json:"codecProfile,omitempty"`
	CodecLevel    string        `json:"codecLevel,omitempty"`
	PixelFormat   string        `json:"pixelFormat,omitempty"`
	GopSize       int           `json:"gopSize,omitempty"`
	BFrames       int           `json:"bFrames,omitempty"`
	RefFrames     int           `json:"refFrames,omitempty"`
	MaxBitrateKbps int          `json:"maxBitrateKbps,omitempty"`
	CRF           int           `json:"crf,omitempty"`
	QP            int           `json:"qp,omitempty"`
	HardwareAccel bool          `json:"hardwareAccel"`
}

type StreamProfile struct {
	Preset              string        `json:"preset"`
	Transport           string        `json:"transport"`
	VideoCodecs         []string      `json:"videoCodecs"`
	ActiveVideoCodec    string        `json:"activeVideoCodec"`
	AudioCodecs         []string      `json:"audioCodecs"`
	ActiveAudioCodec    string        `json:"activeAudioCodec"`
	EncodingMode        EncodingMode  `json:"encodingMode"`
	EncoderCfg          EncoderConfig `json:"encoderCfg"`
	MinBitrateKbps      int           `json:"minBitrateKbps"`
	StartBitrateKbps    int           `json:"startBitrateKbps"`
	MaxBitrateKbps      int           `json:"maxBitrateKbps"`
	ResolutionWidth     int           `json:"resolutionWidth"`
	ResolutionHeight    int           `json:"resolutionHeight"`
	RefreshRateHz       int           `json:"refreshRateHz"`
	AdaptiveBitrate     bool          `json:"adaptiveBitrate"`
	AdaptiveResolution  bool          `json:"adaptiveResolution"`
	PacketLossRecovery  bool          `json:"packetLossRecovery"`
	AudioEnabled        bool          `json:"audioEnabled"`
	LatencyTargetMs     int           `json:"latencyTargetMs"`
	KeyframeIntervalMs  int           `json:"keyframeIntervalMs"`
}

type SessionCapabilities struct {
	ClipboardSync      bool        `json:"clipboardSync"`
	FileTransfer       bool        `json:"fileTransfer"`
	MultiMonitor       bool        `json:"multiMonitor"`
	AudioForwarding    bool        `json:"audioForwarding"`
	SessionRecording   RecordingMode `json:"sessionRecording"`
	Fullscreen         bool        `json:"fullscreen"`
	HighRefreshRate    bool        `json:"highRefreshRate"`
	ControllerRumble   bool        `json:"controllerRumble"`
	SupportedInputs    []string    `json:"supportedInputs"`
}

type RecordingMode string

const (
	RecordingModeDisabled RecordingMode = "disabled"
	RecordingModeOptional RecordingMode = "optional"
	RecordingModeRequired RecordingMode = "required"
)

type SessionPermissions struct {
	RequiresUserApproval  bool        `json:"requiresUserApproval"`
	AllowRemoteControl    bool        `json:"allowRemoteControl"`
	AllowClipboardRead    bool        `json:"allowClipboardRead"`
	AllowClipboardWrite   bool        `json:"allowClipboardWrite"`
	AllowFileUpload       bool        `json:"allowFileUpload"`
	AllowFileDownload     bool        `json:"allowFileDownload"`
	AllowAudioForwarding  bool        `json:"allowAudioForwarding"`
	AllowSessionRecording bool        `json:"allowSessionRecording"`
	MaxDurationSeconds    int         `json:"maxDurationSeconds"`
	IdleTimeoutSeconds    int         `json:"idleTimeoutSeconds"`
	ApprovedByUserID      string      `json:"approvedByUserId"`
	ApprovedAt            time.Time   `json:"approvedAt"`
}

type NetworkQualitySnapshot struct {
	RTTMs               int       `json:"rttMs"`
	JitterMs            int       `json:"jitterMs"`
	PacketLossPercent   float64   `json:"packetLossPercent"`
	DownstreamMbps      float64   `json:"downstreamMbps"`
	UpstreamMbps        float64   `json:"upstreamMbps"`
	Score               float64   `json:"score"`
	SampledAt           time.Time `json:"sampledAt"`
}

type TemporaryAccessLink struct {
	ID               string            `json:"id"`
	SessionID        string            `json:"sessionId"`
	CreatedByUserID  string            `json:"createdByUserId"`
	URL              string            `json:"url"`
	OneTimeUse       bool              `json:"oneTimeUse"`
	Permissions      SessionPermissions `json:"permissions"`
	CreatedAt        time.Time         `json:"createdAt"`
	ExpiresAt        time.Time         `json:"expiresAt"`
	RevokedAt        *time.Time        `json:"revokedAt,omitempty"`
	AccessCount      int               `json:"accessCount"`
}

type PendingApproval struct {
	SessionID      string     `json:"sessionId"`
	RequestedBy    string     `json:"requestedBy"`
	TargetDeviceID string     `json:"targetDeviceId"`
	RequestedAt    time.Time  `json:"requestedAt"`
	ExpiresAt      time.Time  `json:"expiresAt"`
}

type CreateSessionRequest struct {
	UserID            string
	Type              SessionType
	Mode              SessionMode
	ResourceClass     ResourceClass
	VMID              string
	HostID            string
	Region            string
	GPUPreferred      bool
	GPURequired       bool
	StreamPreset      string
	RequestedResolution Resolution
	RequestedInputs   []string
	RequestedCapabilities SessionCapabilities
	Permissions       SessionPermissions
}

type Resolution struct {
	Width       int
	Height      int
	RefreshRate int
}

type CreateSessionResponse struct {
	Session         *Session `json:"session"`
	SignalingURL    string   `json:"signalingUrl"`
	ConnectionToken string   `json:"connectionToken"`
	EstimatedReady  int      `json:"estimatedReady"`
}

func NewSessionManager(logger *log.Logger, hostAllocator *HostAllocator) *SessionManager {
	if logger == nil {
		logger = log.Default()
	}

	if hostAllocator == nil {
		hostAllocator = NewHostAllocator(logger)
	}

	return &SessionManager{
		logger:           logger,
		sessions:         make(map[string]*Session),
		hostAllocator:    hostAllocator,
		scheduler:        NewSessionScheduler(logger, hostAllocator),
		auditLogger:      NewAuditLogger(logger),
		pendingApprovals: make(map[string]*PendingApproval),
	}
}

func (m *SessionManager) CreateSession(ctx context.Context, req *CreateSessionRequest) (*CreateSessionResponse, error) {
	// Validate request
	if req.UserID == "" {
		return nil, errors.New("user ID is required")
	}

	// Generate session ID
	sessionID := generateSessionID()
	connectionToken := generateConnectionToken()

	// Set defaults
	if req.StreamPreset == "" {
		req.StreamPreset = "safe"
	}
	if req.RequestedResolution.Width == 0 {
		req.RequestedResolution.Width = 1920
	}
	if req.RequestedResolution.Height == 0 {
		req.RequestedResolution.Height = 1080
	}
	if req.RequestedResolution.RefreshRate == 0 {
		req.RequestedResolution.RefreshRate = 60
	}
	if req.Permissions.MaxDurationSeconds == 0 {
		req.Permissions.MaxDurationSeconds = 3600 // 1 hour default
	}
	if req.Permissions.IdleTimeoutSeconds == 0 {
		req.Permissions.IdleTimeoutSeconds = 600 // 10 minutes default
	}

	// Build stream profile
	streamProfile := m.buildStreamProfile(req.StreamPreset, req.RequestedResolution, req.Mode)

	// Check if approval is required for remote support
	if req.Type == SessionTypeRemoteSupport && req.Permissions.RequiresUserApproval {
		return m.createSessionRequiringApproval(ctx, req, sessionID, connectionToken, streamProfile)
	}

	// Allocate host
	host, err := m.hostAllocator.AllocateHost(ctx, req.Type, req.ResourceClass, req.Region, req.GPURequired)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate host: %w", err)
	}

	// Create session
	session := &Session{
		ID:              sessionID,
		UserID:          req.UserID,
		HostID:          host.ID,
		Type:            req.Type,
		Mode:            req.Mode,
		ResourceClass:   req.ResourceClass,
		State:           SessionStateQueued,
		StreamProfile:   streamProfile,
		Capabilities:    req.RequestedCapabilities,
		Permissions:     req.Permissions,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		ExpiresAt:       time.Now().Add(time.Duration(req.Permissions.MaxDurationSeconds) * time.Second),
		ConnectionToken: connectionToken,
	}

	m.sessionsMutex.Lock()
	m.sessions[sessionID] = session
	m.sessionsMutex.Unlock()

	// Log audit event
	m.auditLogger.Log(sessionID, req.UserID, "created", map[string]interface{}{
		"type":        req.Type,
		"mode":        req.Mode,
		"resourceClass": req.ResourceClass,
		"hostId":      host.ID,
	})

	// Start provisioning with a fresh Background context — the HTTP handler's
	// ctx is cancelled when the client disconnects, so using it here cancelled
	// provisioning on any normal completed HTTP request. The approval path at
	// line 430 already uses Background(); align this path with it.
	go m.provisionSession(context.Background(), session)

	m.logger.Printf("created session %s (type=%s, mode=%s, host=%s)", sessionID, req.Type, req.Mode, host.ID)

	return &CreateSessionResponse{
		Session:         session,
		SignalingURL:    fmt.Sprintf("/signal/%s", sessionID),
		ConnectionToken: connectionToken,
		EstimatedReady:  30, // 30 seconds estimated
	}, nil
}

func (m *SessionManager) createSessionRequiringApproval(ctx context.Context, req *CreateSessionRequest, sessionID string, connectionToken string, streamProfile StreamProfile) (*CreateSessionResponse, error) {
	// Create session in waiting state
	session := &Session{
		ID:              sessionID,
		UserID:          req.UserID,
		Type:            req.Type,
		Mode:            req.Mode,
		ResourceClass:   req.ResourceClass,
		State:           SessionStateWaitingApproval,
		StreamProfile:   streamProfile,
		Capabilities:    req.RequestedCapabilities,
		Permissions:     req.Permissions,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		ExpiresAt:       time.Now().Add(time.Duration(req.Permissions.MaxDurationSeconds) * time.Second),
		ConnectionToken: connectionToken,
	}

	m.sessionsMutex.Lock()
	m.sessions[sessionID] = session
	m.sessionsMutex.Unlock()

	// Create pending approval
	pending := &PendingApproval{
		SessionID:      sessionID,
		RequestedBy:    req.UserID,
		TargetDeviceID: req.VMID,
		RequestedAt:    time.Now(),
		ExpiresAt:      time.Now().Add(5 * time.Minute), // 5 minutes to approve
	}

	m.approvalsMutex.Lock()
	m.pendingApprovals[sessionID] = pending
	m.approvalsMutex.Unlock()

	m.logger.Printf("created session %s requiring approval", sessionID)

	return &CreateSessionResponse{
		Session:         session,
		SignalingURL:    fmt.Sprintf("/signal/%s", sessionID),
		ConnectionToken: connectionToken,
		EstimatedReady:  0, // Unknown until approved
	}, nil
}

func (m *SessionManager) ApproveSession(sessionID string, approverUserID string) error {
	m.sessionsMutex.Lock()
	defer m.sessionsMutex.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return errors.New("session not found")
	}

	session.Mutex.Lock()
	defer session.Mutex.Unlock()

	if session.State != SessionStateWaitingApproval {
		return errors.New("session is not waiting for approval")
	}

	// Remove from pending approvals
	m.approvalsMutex.Lock()
	delete(m.pendingApprovals, sessionID)
	m.approvalsMutex.Unlock()

	// Update permissions
	session.Permissions.ApprovedByUserID = approverUserID
	session.Permissions.ApprovedAt = time.Now()
	session.State = SessionStateQueued
	session.UpdatedAt = time.Now()

	// Log audit event
	m.auditLogger.Log(sessionID, approverUserID, "approved", map[string]interface{}{
		"originalRequester": session.UserID,
	})

	// Start provisioning
	ctx := context.Background()
	go m.provisionSession(ctx, session)

	m.logger.Printf("approved session %s by user %s", sessionID, approverUserID)

	return nil
}

func (m *SessionManager) GetSession(sessionID string) (*Session, error) {
	m.sessionsMutex.RLock()
	defer m.sessionsMutex.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, errors.New("session not found")
	}

	return session, nil
}

func (m *SessionManager) ListSessions() []*Session {
	m.sessionsMutex.RLock()
	defer m.sessionsMutex.RUnlock()

	sessions := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

func (m *SessionManager) UpdateSessionState(sessionID string, state SessionState) error {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return err
	}

	session.Mutex.Lock()
	defer session.Mutex.Unlock()

	session.State = state
	session.UpdatedAt = time.Now()

	if state == SessionStateEnded || state == SessionStateExpired || state == SessionStateFailed {
		now := time.Now()
		session.EndedAt = &now
	}

	m.logger.Printf("updated session %s state to %s", sessionID, state)

	return nil
}

func (m *SessionManager) EndSession(sessionID string, reason string) error {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return err
	}

	session.Mutex.Lock()
	defer session.Mutex.Unlock()

	session.State = SessionStateEnded
	now := time.Now()
	session.EndedAt = &now
	session.UpdatedAt = now

	// Log audit event
	m.auditLogger.Log(sessionID, session.UserID, "ended", map[string]interface{}{
		"reason": reason,
	})

	m.logger.Printf("ended session %s (reason: %s)", sessionID, reason)

	// Clean up after delay. Using time.AfterFunc so the runtime schedules a
	// single-shot timer instead of stranding a goroutine that sleeps 5min
	// with no cancellation path — one per ended session added up fast on a
	// gateway that saw hundreds of connections.
	time.AfterFunc(5*time.Minute, func() {
		m.sessionsMutex.Lock()
		delete(m.sessions, sessionID)
		m.sessionsMutex.Unlock()
	})

	return nil
}

func (m *SessionManager) UpdateNetworkQuality(sessionID string, quality *NetworkQualitySnapshot) error {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return err
	}

	session.Mutex.Lock()
	defer session.Mutex.Unlock()

	session.NetworkQuality = quality
	session.UpdatedAt = time.Now()

	return nil
}

func (m *SessionManager) CreateTemporaryAccessLink(sessionID string, createdByUserID string, oneTimeUse bool, permissions SessionPermissions) (*TemporaryAccessLink, error) {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	linkID := generateLinkID()
	url := fmt.Sprintf("/access/%s", linkID)

	link := &TemporaryAccessLink{
		ID:              linkID,
		SessionID:       sessionID,
		CreatedByUserID: createdByUserID,
		URL:             url,
		OneTimeUse:      oneTimeUse,
		Permissions:     permissions,
		CreatedAt:       time.Now(),
		ExpiresAt:       time.Now().Add(1 * time.Hour),
	}

	session.Mutex.Lock()
	session.TemporaryLinks = append(session.TemporaryLinks, *link)
	session.Mutex.Unlock()

	// Log audit event
	m.auditLogger.Log(sessionID, createdByUserID, "temporary-link-created", map[string]interface{}{
		"linkId":     linkID,
		"oneTimeUse": oneTimeUse,
	})

	return link, nil
}

func (m *SessionManager) provisionSession(ctx context.Context, session *Session) {
	// Update state to provisioning
	session.Mutex.Lock()
	session.State = SessionStateProvisioning
	session.UpdatedAt = time.Now()
	session.Mutex.Unlock()

	// Simulate provisioning (in real implementation, this would communicate
	// with host agent). Use a select on ctx.Done so a client disconnect or
	// server shutdown aborts the 5-second wait instead of stalling a goroutine.
	select {
	case <-time.After(5 * time.Second):
	case <-ctx.Done():
		session.Mutex.Lock()
		session.State = SessionStateFailed
		session.UpdatedAt = time.Now()
		session.Mutex.Unlock()
		m.logger.Printf("provisioning aborted for session %s: %v", session.ID, ctx.Err())
		return
	}

	// Update state to connecting
	session.Mutex.Lock()
	session.State = SessionStateConnecting
	session.UpdatedAt = time.Now()
	session.Mutex.Unlock()

	m.logger.Printf("provisioned session %s", session.ID)
}

func (m *SessionManager) buildStreamProfile(preset string, resolution Resolution, mode SessionMode) StreamProfile {
	encodingMode := EncodingModeBalanced
	if preset == "fast" {
		encodingMode = EncodingModeSpeed
	} else if preset == "safe" {
		encodingMode = EncodingModeQuality
	}

	hasGPU := false
	encoderCfg := buildDefaultEncoderConfig(encodingMode, hasGPU)

	profile := StreamProfile{
		Preset:             preset,
		Transport:          "webrtc",
		VideoCodecs:        []string{"h264", "h265", "vp8", "vp9"},
		AudioCodecs:        []string{"opus", "aac"},
		EncodingMode:       encodingMode,
		EncoderCfg:         encoderCfg,
		ResolutionWidth:    resolution.Width,
		ResolutionHeight:   resolution.Height,
		RefreshRateHz:      resolution.RefreshRate,
		AdaptiveBitrate:    true,
		AdaptiveResolution: preset == "safe",
		PacketLossRecovery:  true,
		AudioEnabled:       true,
	}

	if preset == "fast" {
		profile.MinBitrateKbps = 2000
		profile.StartBitrateKbps = 5000
		profile.MaxBitrateKbps = 20000
		profile.LatencyTargetMs = 20
		profile.KeyframeIntervalMs = 2000
		profile.VideoCodecs = []string{"h265", "h264", "h263"}
	} else {
		profile.MinBitrateKbps = 500
		profile.StartBitrateKbps = 1500
		profile.MaxBitrateKbps = 8000
		profile.LatencyTargetMs = 100
		profile.KeyframeIntervalMs = 4000
		profile.VideoCodecs = []string{"h264", "h265", "vp9", "vp8"}
	}

	if mode == SessionModeGaming {
		profile.VideoCodecs = []string{"h265", "h264", "av1"}
		profile.MinBitrateKbps = 3000
		profile.StartBitrateKbps = 8000
		profile.MaxBitrateKbps = 30000
		profile.LatencyTargetMs = 15
		profile.EncodingMode = EncodingModeSpeed
		profile.EncoderCfg = buildDefaultEncoderConfig(EncodingModeSpeed, hasGPU)
	}

	return profile
}

func buildDefaultEncoderConfig(mode EncodingMode, hardwareAccel bool) EncoderConfig {
	cfg := EncoderConfig{
		Mode:          mode,
		HardwareAccel: hardwareAccel,
	}

	switch mode {
	case EncodingModeSpeed:
		cfg.Preset = EncoderPresetVeryfast
		cfg.PixelFormat = "yuv420p"
		cfg.GopSize = 120
		cfg.BFrames = 0
		cfg.RefFrames = 2
		cfg.CRF = 28
		cfg.QP = 28
	case EncodingModeQuality:
		cfg.Preset = EncoderPresetSlow
		cfg.PixelFormat = "yuv420p"
		cfg.GopSize = 60
		cfg.BFrames = 3
		cfg.RefFrames = 5
		cfg.CRF = 18
		cfg.QP = 18
	default:
		cfg.Preset = EncoderPresetMedium
		cfg.PixelFormat = "yuv420p"
		cfg.GopSize = 90
		cfg.BFrames = 2
		cfg.RefFrames = 3
		cfg.CRF = 23
		cfg.QP = 23
	}

	if hardwareAccel {
		switch mode {
		case EncodingModeSpeed:
			cfg.Preset = EncoderPresetFast
		case EncodingModeQuality:
			cfg.Preset = EncoderPresetMedium
		default:
			cfg.Preset = EncoderPresetFast
		}
	}

	return cfg
}

func generateSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("sess_%d", time.Now().UnixNano())
	}
	return "sess_" + hex.EncodeToString(b)
}

func generateConnectionToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("token_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func generateLinkID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("link_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}