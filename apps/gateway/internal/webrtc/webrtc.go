package webrtc

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
)

const (
	MimeTypeH263 = "video/H263"
)

const (
	defaultReadTimeout    = 30 * time.Second
	defaultWriteTimeout   = 10 * time.Second
	defaultPingInterval   = 15 * time.Second
	maxMessageSize        = 64 * 1024
	maxConcurrentSessions = 1000
)

type Server struct {
	logger          *log.Logger
	upgrader        websocket.Upgrader
	sessions        map[string]*Session
	sessionsMutex   sync.RWMutex
	codecSupport    CodecSupport
	sessionLimiter  chan struct{}
	turnServer      string
	stunServer      string
	networkMonitor  *NetworkMonitor
	shutdownCh      chan struct{}
	shutdownOnce    sync.Once
}

type CodecSupport struct {
	H263Enabled   bool
	H264Enabled   bool
	H265Enabled   bool
	AV1Enabled    bool
	VP8Enabled    bool
	VP9Enabled    bool
	OpusEnabled   bool
	AACEnabled    bool
	HardwareAccel bool
}

type Session struct {
	ID             string
	ClientID       string
	VideoCodec     string
	AudioCodec     string
	Preset         string
	EncodingMode   EncodingMode
	EncoderCfg     EncoderConfig
	State          SessionState
	CreatedAt      time.Time
	UpdatedAt      time.Time
	ExpiresAt      time.Time
	PeerConnection *webrtc.PeerConnection
	VideoTrack     *webrtc.TrackLocalStaticRTP
	AudioTrack     *webrtc.TrackLocalStaticRTP
	DataChannel    *webrtc.DataChannel
	Stats          SessionStats
	Mutex          sync.RWMutex
	stopCh         chan struct{}
}

type SessionState string

const (
	SessionStateCreated    SessionState = "created"
	SessionStateConnecting SessionState = "connecting"
	SessionStateActive     SessionState = "active"
	SessionStateReconnecting SessionState = "reconnecting"
	SessionStateEnded      SessionState = "ended"
	SessionStateFailed     SessionState = "failed"
)

type SessionStats struct {
	BytesReceived      uint64
	BytesSent          uint64
	PacketsReceived    uint64
	PacketsSent        uint64
	PacketsLost        uint32
	CurrentBitrate     uint32 // Mbps, computed from delta between samples
	CurrentFPS         uint32
	RTT                uint32
	Jitter             uint32
	LastSampledAt      time.Time
	// lastBytesReceived tracks the BytesReceived value at the previous sample so
	// CurrentBitrate reflects the instantaneous rate rather than the session-long
	// average that decays over time.
	lastBytesReceived  uint64
}

type NetworkMonitor struct {
	sessions map[string]*NetworkQualitySnapshot
	mutex    sync.RWMutex
}

type NetworkQualitySnapshot struct {
	RTT               time.Duration
	Jitter            time.Duration
	PacketLossPercent float64
	DownstreamMbps    float64
	UpstreamMbps      float64
	Score             float64
	SampledAt         time.Time
}

type SignalMessage struct {
	SessionID string          `json:"sessionId"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	SentAt    time.Time       `json:"sentAt"`
}

type CreateSessionRequest struct {
	ClientID      string       `json:"clientId"`
	VideoCodecs   []string     `json:"videoCodecs"`
	AudioCodecs   []string     `json:"audioCodecs"`
	Preset        string       `json:"preset"`
	EncodingMode  EncodingMode `json:"encodingMode"`
	Region        string       `json:"region,omitempty"`
	GPURequired   bool         `json:"gpuRequired"`
	Resolution    Resolution   `json:"resolution"`
	RequestedFPS  uint32       `json:"requestedFps"`
	LatencyTarget uint32       `json:"latencyTarget"`
}

type Resolution struct {
	Width       uint32 `json:"width"`
	Height      uint32 `json:"height"`
	RefreshRate uint32 `json:"refreshRate"`
}

type CreateSessionResponse struct {
	SessionID    string   `json:"sessionId"`
	ClientID     string   `json:"clientId"`
	VideoCodec   string   `json:"videoCodec"`
	AudioCodec   string   `json:"audioCodec"`
	SignalingURL string   `json:"signalingUrl"`
	TURNServers  []string `json:"turnServers"`
	STUNServers  []string `json:"stunServers"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

func NewServer(logger *log.Logger, codecSupport CodecSupport, turnServer, stunServer string) (*Server, error) {
	if logger == nil {
		logger = log.Default()
	}

	allowAll := func(r *http.Request) bool {
		return true
	}

	return &Server{
		logger:         logger,
		sessions:       make(map[string]*Session),
		codecSupport:   codecSupport,
		sessionLimiter: make(chan struct{}, maxConcurrentSessions),
		turnServer:     turnServer,
		stunServer:     stunServer,
		networkMonitor: &NetworkMonitor{
			sessions: make(map[string]*NetworkQualitySnapshot),
		},
		upgrader: websocket.Upgrader{
			ReadBufferSize:     32 * 1024,
			WriteBufferSize:    32 * 1024,
			EnableCompression:  true,
			CheckOrigin:        allowAll,
		},
		shutdownCh: make(chan struct{}),
	}, nil
}

// Shutdown signals the server to release delayed-deletion timers and stop
// scheduling new session cleanup work. Safe to call multiple times.
func (s *Server) Shutdown() {
	s.shutdownOnce.Do(func() {
		if s.shutdownCh != nil {
			close(s.shutdownCh)
		}
	})
}

// safeConn serializes all writes to a websocket.Conn. Gorilla's docs require
// callers to guarantee only one goroutine calls a write method at a time.
// Every write also sets a fresh SetWriteDeadline so a stalled peer cannot
// block a write goroutine indefinitely (which would leak the goroutine + its
// per-session state on every dead client).
type safeConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (c *safeConn) WriteJSON(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(defaultWriteTimeout))
	return c.conn.WriteJSON(v)
}

func (c *safeConn) WriteMessage(mt int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(defaultWriteTimeout))
	return c.conn.WriteMessage(mt, data)
}

func (s *Server) HandleSignal(w http.ResponseWriter, r *http.Request) {
	sessionID := extractSessionID(r.URL.Path)
	if sessionID == "" {
		http.Error(w, "session ID required", http.StatusBadRequest)
		return
	}

	rawConn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Printf("websocket upgrade failed for session %s: %v", sessionID, err)
		return
	}
	defer rawConn.Close()

	conn := &safeConn{conn: rawConn}

	rawConn.SetReadLimit(maxMessageSize)
	rawConn.SetReadDeadline(time.Now().Add(defaultReadTimeout))
	rawConn.SetPongHandler(func(string) error {
		rawConn.SetReadDeadline(time.Now().Add(defaultReadTimeout))
		return nil
	})

	s.sessionsMutex.RLock()
	_, exists := s.sessions[sessionID]
	s.sessionsMutex.RUnlock()

	if !exists {
		s.logger.Printf("session %s not found", sessionID)
		_ = conn.WriteJSON(map[string]string{"error": "session not found"})
		return
	}

	s.logger.Printf("signaling connected for session %s", sessionID)

	pingTicker := time.NewTicker(defaultPingInterval)
	defer pingTicker.Stop()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if err := rawConn.SetReadDeadline(time.Now().Add(defaultReadTimeout)); err != nil {
				s.logger.Printf("signaling read deadline set error for session %s: %v", sessionID, err)
				return
			}
			// Use ReadMessage (not ReadJSON) so we can tolerate two client
			// dialects on the same signaling endpoint: the RTCPeerConnection
			// clients under /connect/page.tsx send text JSON (offer/answer/ice),
			// while the RemoteDesktop app's useTunnelConnection sends framed
			// binary heartbeats. ReadJSON treated the framed binary as a fatal
			// decode error and returned, which produced an infinite 5s
			// reconnect loop on every RemoteDesktop session.
			mt, data, err := rawConn.ReadMessage()
			if err != nil {
				// 1000 (Normal), 1001 (Going Away), 1005 (No Status), and
				// 1006 (Abnormal) all describe expected browser-side close
				// paths (tab closed, navigation, viewer unmounted). Only
				// truly unexpected close codes are worth logging.
				if websocket.IsUnexpectedCloseError(err,
					websocket.CloseNormalClosure,
					websocket.CloseGoingAway,
					websocket.CloseNoStatusReceived,
					websocket.CloseAbnormalClosure,
				) {
					s.logger.Printf("signaling read error for session %s: %v", sessionID, err)
				}
				return
			}
			if mt != websocket.TextMessage {
				// Binary frame: framed tunnel heartbeat from useTunnelConnection.
				// Ignore silently — the tunnel protocol has no server-side
				// obligations here; the connection stays open.
				continue
			}
			var msg SignalMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				// Malformed JSON on a text frame — skip rather than disconnect;
				// the peer may recover on the next well-formed message.
				continue
			}
			if err := s.handleSignalMessage(sessionID, &msg, conn); err != nil {
				s.logger.Printf("signaling message error for session %s: %v", sessionID, err)
				_ = conn.WriteJSON(map[string]string{"error": err.Error()})
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case <-pingTicker.C:
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					s.logger.Printf("ping failed for session %s: %v", sessionID, err)
					return
				}
			case <-done:
				return
			}
		}
	}()

	<-done
}

func (s *Server) CreateSession(req *CreateSessionRequest) (resp *CreateSessionResponse, err error) {
	select {
	case s.sessionLimiter <- struct{}{}:
	default:
		return nil, errors.New("maximum concurrent sessions reached")
	}

	releaseSlot := true
	defer func() {
		if releaseSlot {
			select {
			case <-s.sessionLimiter:
			default:
			}
		}
	}()

	if req.EncodingMode == "" {
		req.EncodingMode = EncodingModeBalanced
	}

	videoCodec := s.selectVideoCodec(req.VideoCodecs, req.Preset)
	audioCodec := s.selectAudioCodec(req.AudioCodecs)
	expiresAt := time.Now().Add(24 * time.Hour)

	encoderProfile, err := SelectEncoderProfile(videoCodec, req.EncodingMode, s.codecSupport.HardwareAccel)
	if err != nil {
		return nil, fmt.Errorf("encoder profile: %w", err)
	}

	iceServers := []webrtc.ICEServer{}
	if s.stunServer != "" {
		iceServers = append(iceServers, webrtc.ICEServer{URLs: []string{s.stunServer}})
	} else {
		iceServers = append(iceServers, webrtc.ICEServer{URLs: []string{"stun:stun.l.google.com:19302"}})
	}
	if s.turnServer != "" {
		iceServers = append(iceServers, webrtc.ICEServer{URLs: []string{s.turnServer}})
	}

	mediaEngine := &webrtc.MediaEngine{}
	if err := s.registerCodecs(mediaEngine, videoCodec, audioCodec); err != nil {
		return nil, fmt.Errorf("register codecs: %w", err)
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine))
	pc, err := api.NewPeerConnection(webrtc.Configuration{
		ICEServers: iceServers,
	})
	if err != nil {
		return nil, fmt.Errorf("create peer connection: %w", err)
	}

	videoTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: s.codecMimeType(videoCodec), ClockRate: 90000},
		"video", "freecompute",
	)
	if err != nil {
		pc.Close()
		return nil, fmt.Errorf("create video track: %w", err)
	}

	rtpSender, err := pc.AddTrack(videoTrack)
	if err != nil {
		pc.Close()
		return nil, fmt.Errorf("add video track: %w", err)
	}

	go func() {
		buf := getRTPBuf()
		defer putRTPBuf(buf)
		for {
			if _, _, rtcpErr := rtpSender.Read(*buf); rtcpErr != nil {
				return
			}
		}
	}()

	audioTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000},
		"audio", "freecompute",
	)
	if err != nil {
		pc.Close()
		return nil, fmt.Errorf("create audio track: %w", err)
	}

	rtpSender2, err := pc.AddTrack(audioTrack)
	if err != nil {
		pc.Close()
		return nil, fmt.Errorf("add audio track: %w", err)
	}

	go func() {
		buf := getRTPBuf()
		defer putRTPBuf(buf)
		for {
			if _, _, rtcpErr := rtpSender2.Read(*buf); rtcpErr != nil {
				return
			}
		}
	}()

	dc, err := pc.CreateDataChannel("data", nil)
	if err != nil {
		pc.Close()
		return nil, fmt.Errorf("create data channel: %w", err)
	}

	sessionID := generateSessionID()
	session := &Session{
		ID:             sessionID,
		ClientID:       req.ClientID,
		VideoCodec:     videoCodec,
		AudioCodec:     audioCodec,
		Preset:         req.Preset,
		EncodingMode:   req.EncodingMode,
		EncoderCfg:     encoderProfile.Config,
		State:          SessionStateCreated,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		ExpiresAt:      expiresAt,
		PeerConnection: pc,
		VideoTrack:     videoTrack,
		AudioTrack:     audioTrack,
		DataChannel:    dc,
		stopCh:         make(chan struct{}),
	}

	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		s.logger.Printf("session %s ICE state: %s", sessionID, state.String())
		switch state {
		case webrtc.ICEConnectionStateConnected:
			session.Mutex.Lock()
			session.State = SessionStateActive
			session.UpdatedAt = time.Now()
			session.Mutex.Unlock()
		case webrtc.ICEConnectionStateFailed, webrtc.ICEConnectionStateDisconnected:
			session.Mutex.Lock()
			session.State = SessionStateReconnecting
			session.UpdatedAt = time.Now()
			session.Mutex.Unlock()
		case webrtc.ICEConnectionStateClosed:
			s.EndSession(sessionID, "ICE closed")
		}
	})

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		s.logger.Printf("session %s connection state: %s", sessionID, state.String())
	})

	s.sessionsMutex.Lock()
	s.sessions[sessionID] = session
	s.sessionsMutex.Unlock()

	s.startStatsCollector(session)

	releaseSlot = false

	s.logger.Printf("created session %s with codec %s/%s encoding=%s preset=%s (H265=%v)", sessionID, videoCodec, audioCodec, req.EncodingMode, encoderProfile.Config.Preset, s.codecSupport.H265Enabled)

	return &CreateSessionResponse{
		SessionID:    sessionID,
		ClientID:     req.ClientID,
		VideoCodec:   videoCodec,
		AudioCodec:   audioCodec,
		SignalingURL: fmt.Sprintf("/signal/%s", sessionID),
		TURNServers:  s.getTURNServers(),
		STUNServers:  s.getSTUNServers(),
		ExpiresAt:    expiresAt,
	}, nil
}

func (s *Server) registerCodecs(me *webrtc.MediaEngine, videoCodec, audioCodec string) error {
	switch videoCodec {
	case "h264":
		if err := me.RegisterCodec(webrtc.RTPCodecParameters{
			RTPCodecCapability: webrtc.RTPCodecCapability{
				MimeType:    webrtc.MimeTypeH264,
				ClockRate:   90000,
				Channels:    0,
				SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
			},
			PayloadType: 96,
		}, webrtc.RTPCodecTypeVideo); err != nil {
			return err
		}
	case "h265":
		if err := me.RegisterCodec(webrtc.RTPCodecParameters{
			RTPCodecCapability: webrtc.RTPCodecCapability{
				MimeType:  webrtc.MimeTypeH265,
				ClockRate: 90000,
				Channels:  0,
			},
			PayloadType: 96,
		}, webrtc.RTPCodecTypeVideo); err != nil {
			return err
		}
	case "vp8":
		if err := me.RegisterCodec(webrtc.RTPCodecParameters{
			RTPCodecCapability: webrtc.RTPCodecCapability{
				MimeType:  webrtc.MimeTypeVP8,
				ClockRate: 90000,
				Channels:  0,
			},
			PayloadType: 96,
		}, webrtc.RTPCodecTypeVideo); err != nil {
			return err
		}
	case "vp9":
		if err := me.RegisterCodec(webrtc.RTPCodecParameters{
			RTPCodecCapability: webrtc.RTPCodecCapability{
				MimeType:  webrtc.MimeTypeVP9,
				ClockRate: 90000,
				Channels:  0,
			},
			PayloadType: 96,
		}, webrtc.RTPCodecTypeVideo); err != nil {
			return err
		}
	case "av1":
		if err := me.RegisterCodec(webrtc.RTPCodecParameters{
			RTPCodecCapability: webrtc.RTPCodecCapability{
				MimeType:  webrtc.MimeTypeAV1,
				ClockRate: 90000,
				Channels:  0,
			},
			PayloadType: 96,
		}, webrtc.RTPCodecTypeVideo); err != nil {
			return err
		}
	case "h263":
		if err := me.RegisterCodec(webrtc.RTPCodecParameters{
			RTPCodecCapability: webrtc.RTPCodecCapability{
				MimeType:    MimeTypeH263,
				ClockRate:   90000,
				Channels:    0,
				SDPFmtpLine: "profile=0;level=45",
			},
			PayloadType: 97,
		}, webrtc.RTPCodecTypeVideo); err != nil {
			return err
		}
	}

	if err := me.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:  webrtc.MimeTypeOpus,
			ClockRate: 48000,
			Channels:  2,
		},
		PayloadType: 111,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		return err
	}

	return nil
}

func (s *Server) codecMimeType(codec string) string {
	switch codec {
	case "h264":
		return webrtc.MimeTypeH264
	case "h265":
		return webrtc.MimeTypeH265
	case "vp8":
		return webrtc.MimeTypeVP8
	case "vp9":
		return webrtc.MimeTypeVP9
	case "av1":
		return webrtc.MimeTypeAV1
	case "h263":
		return MimeTypeH263
	default:
		return webrtc.MimeTypeH264
	}
}

func (s *Server) startStatsCollector(session *Session) {
	// Start the bandwidth estimator (GCC-style adaptive bitrate + codec switch).
	bwe := NewBandwidthEstimator(session, s.logger)
	bwe.Start()

	go func() {
		defer bwe.Stop()
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-session.stopCh:
				return
			case <-ticker.C:
				if session.PeerConnection == nil {
					return
				}
				stats := session.PeerConnection.GetStats()
				// BytesSent / PacketsSent are also mutated via atomic.AddUint64
				// from WriteVideoRTP / WriteAudioRTP (and via lock-protected
				// increments from HandleMediaIngest). Use atomics here too so
				// all writers agree on a single access mode.
				var bytesReceived, packetsReceived uint64
				var packetsLost, jitter, rtt uint32
				var haveInbound bool
				var bytesSent, packetsSentPion uint64
				var haveOutbound bool
				for _, stat := range stats {
					switch v := stat.(type) {
					case webrtc.InboundRTPStreamStats:
						bytesReceived = v.BytesReceived
						packetsLost = uint32(v.PacketsLost)
						jitter = uint32(v.Jitter * 1000)
						packetsReceived = uint64(v.PacketsReceived)
						haveInbound = true
					case webrtc.OutboundRTPStreamStats:
						bytesSent = v.BytesSent
						packetsSentPion = uint64(v.PacketsSent)
						haveOutbound = true
					case *webrtc.ICECandidatePairStats:
						if v.CurrentRoundTripTime > 0 {
							rtt = uint32(v.CurrentRoundTripTime * 1000)
						}
					}
				}
				session.Mutex.Lock()
				if haveInbound {
					atomic.StoreUint64(&session.Stats.BytesReceived, bytesReceived)
					atomic.StoreUint64(&session.Stats.PacketsReceived, packetsReceived)
					session.Stats.PacketsLost = packetsLost
					session.Stats.Jitter = jitter
				}
				if haveOutbound {
					// Only overwrite from pion's numbers if the atomic write path
					// hasn't recorded anything for this session — otherwise we
					// clobber counts from WriteVideoRTP/HandleMediaIngest.
					if atomic.LoadUint64(&session.Stats.BytesSent) == 0 {
						atomic.StoreUint64(&session.Stats.BytesSent, bytesSent)
					}
					if atomic.LoadUint64(&session.Stats.PacketsSent) == 0 {
						atomic.StoreUint64(&session.Stats.PacketsSent, packetsSentPion)
					}
				}
				if rtt > 0 {
					session.Stats.RTT = rtt
				}
				// Compute bitrate from the DELTA of bytes since the last sample,
				// not from cumulative BytesReceived. The cumulative form divides
				// total bytes by session elapsed time — a smoothed average that
				// looks like "the bitrate is declining" even at steady state.
				br := atomic.LoadUint64(&session.Stats.BytesReceived)
				if session.Stats.LastSampledAt.IsZero() {
					session.Stats.CurrentBitrate = 0
					session.Stats.lastBytesReceived = br
				} else {
					elapsed := time.Since(session.Stats.LastSampledAt).Seconds()
					if elapsed > 0 && br >= session.Stats.lastBytesReceived {
						deltaBytes := br - session.Stats.lastBytesReceived
						session.Stats.CurrentBitrate = uint32(float64(deltaBytes*8) / elapsed / 1_000_000)
					}
					session.Stats.lastBytesReceived = br
				}
				session.Stats.LastSampledAt = time.Now()
				session.Mutex.Unlock()
			}
		}
	}()
}


func (s *Server) GetSession(sessionID string) (*Session, error) {
	s.sessionsMutex.RLock()
	defer s.sessionsMutex.RUnlock()
	session, exists := s.sessions[sessionID]
	if !exists {
		return nil, errors.New("session not found")
	}
	return session, nil
}

func (s *Server) ListSessions() []map[string]interface{} {
	s.sessionsMutex.RLock()
	defer s.sessionsMutex.RUnlock()

	sessions := make([]map[string]interface{}, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, map[string]interface{}{
			"id":             session.ID,
			"state":          session.State,
			"signalingUrl":   fmt.Sprintf("/signal/%s", session.ID),
			"videoCodec":     session.VideoCodec,
			"audioCodec":     session.AudioCodec,
			"preset":         session.Preset,
			"encodingMode":   session.EncodingMode,
			"createdAt":      session.CreatedAt,
		})
	}
	return sessions
}

func (s *Server) EndSession(sessionID string, reason string) error {
	s.sessionsMutex.Lock()
	session, exists := s.sessions[sessionID]
	s.sessionsMutex.Unlock()

	if !exists {
		return errors.New("session not found")
	}

	session.Mutex.Lock()
	if session.State == SessionStateEnded {
		session.Mutex.Unlock()
		return nil
	}
	session.State = SessionStateEnded
	session.UpdatedAt = time.Now()
	if session.stopCh != nil {
		close(session.stopCh)
	}
	pc := session.PeerConnection
	session.Mutex.Unlock()

	// Close the PeerConnection OUTSIDE session.Mutex: pion synchronously fires
	// OnICEConnectionStateChange(Closed) from Close(), which re-enters
	// EndSession → session.Mutex.Lock() → deadlock. State is already Ended so
	// the recursive call short-circuits at the state check above.
	if pc != nil {
		_ = pc.Close()
	}

	select {
	case <-s.sessionLimiter:
	default:
	}

	s.logger.Printf("ended session %s (reason: %s)", sessionID, reason)

	// Schedule delayed deletion so clients can still fetch final stats.
	// Use time.AfterFunc so the timer can be short-circuited on shutdown
	// (via s.shutdownCh) instead of leaking a sleeping goroutine per session.
	timer := time.AfterFunc(5*time.Minute, func() {
		s.sessionsMutex.Lock()
		delete(s.sessions, sessionID)
		s.sessionsMutex.Unlock()
	})
	// Stop the timer if the server is shutting down before it fires.
	if s.shutdownCh != nil {
		go func() {
			select {
			case <-s.shutdownCh:
				timer.Stop()
			case <-time.After(5*time.Minute + time.Second):
				// timer either fired or was stopped; no cleanup needed
			}
		}()
	}

	return nil
}

func (s *Server) UpdateNetworkQuality(sessionID string, quality *NetworkQualitySnapshot) {
	s.networkMonitor.mutex.Lock()
	defer s.networkMonitor.mutex.Unlock()
	s.networkMonitor.sessions[sessionID] = quality

	s.sessionsMutex.RLock()
	session, exists := s.sessions[sessionID]
	s.sessionsMutex.RUnlock()

	if exists {
		session.Mutex.Lock()
		session.Stats.RTT = uint32(quality.RTT.Milliseconds())
		session.Stats.Jitter = uint32(quality.Jitter.Milliseconds())
		session.Stats.LastSampledAt = time.Now()
		session.Mutex.Unlock()
	}
}

func (s *Server) GetNetworkQuality(sessionID string) (*NetworkQualitySnapshot, error) {
	s.networkMonitor.mutex.RLock()
	defer s.networkMonitor.mutex.RUnlock()
	quality, exists := s.networkMonitor.sessions[sessionID]
	if !exists {
		return nil, errors.New("network quality not found")
	}
	return quality, nil
}

func (s *Server) handleSignalMessage(sessionID string, msg *SignalMessage, wsConn *safeConn) error {
	s.sessionsMutex.RLock()
	session, exists := s.sessions[sessionID]
	s.sessionsMutex.RUnlock()

	if !exists {
		return errors.New("session not found")
	}

	switch msg.Type {
	case "offer":
		var offer webrtc.SessionDescription
		if err := json.Unmarshal(msg.Payload, &offer); err != nil {
			return fmt.Errorf("unmarshal offer: %w", err)
		}
		if err := session.PeerConnection.SetRemoteDescription(offer); err != nil {
			return fmt.Errorf("set remote description: %w", err)
		}
		answer, err := session.PeerConnection.CreateAnswer(nil)
		if err != nil {
			return fmt.Errorf("create answer: %w", err)
		}
		if err := session.PeerConnection.SetLocalDescription(answer); err != nil {
			return fmt.Errorf("set local description: %w", err)
		}
		answerBytes, err := json.Marshal(answer)
		if err != nil {
			return fmt.Errorf("marshal answer: %w", err)
		}
		session.Mutex.Lock()
		session.State = SessionStateConnecting
		session.UpdatedAt = time.Now()
		session.Mutex.Unlock()
		if err := wsConn.WriteJSON(SignalMessage{
			SessionID: sessionID,
			Type:      "answer",
			Payload:   answerBytes,
			SentAt:    time.Now(),
		}); err != nil {
			return fmt.Errorf("write answer: %w", err)
		}

	case "ice-candidate":
		var candidate webrtc.ICECandidateInit
		if err := json.Unmarshal(msg.Payload, &candidate); err != nil {
			return fmt.Errorf("unmarshal ICE candidate: %w", err)
		}
		if err := session.PeerConnection.AddICECandidate(candidate); err != nil {
			return fmt.Errorf("add ICE candidate: %w", err)
		}

	case "renegotiate":
		answer, err := session.PeerConnection.CreateAnswer(nil)
		if err != nil {
			return fmt.Errorf("create renegotiate answer: %w", err)
		}
		if err := session.PeerConnection.SetLocalDescription(answer); err != nil {
			return fmt.Errorf("set local description: %w", err)
		}
		answerBytes, err := json.Marshal(answer)
		if err != nil {
			return fmt.Errorf("marshal renegotiate answer: %w", err)
		}
		if err := wsConn.WriteJSON(SignalMessage{
			SessionID: sessionID,
			Type:      "answer",
			Payload:   answerBytes,
			SentAt:    time.Now(),
		}); err != nil {
			return fmt.Errorf("write renegotiate answer: %w", err)
		}

	case "close":
		return s.EndSession(sessionID, "client requested close")

	default:
		return fmt.Errorf("unknown signal type: %s", msg.Type)
	}

	return nil
}

func (s *Server) WriteVideoRTP(sessionID string, data []byte) error {
	session, err := s.GetSession(sessionID)
	if err != nil {
		return err
	}
	session.Mutex.RLock()
	track := session.VideoTrack
	session.Mutex.RUnlock()
	if track == nil {
		return errors.New("video track not available")
	}
	n, err := track.Write(data)
	if err != nil {
		return err
	}
	// All writers to PacketsSent / BytesSent use atomics; the stats collector
	// reads via atomic.LoadUint64 and writes deltas via atomic.StoreUint64.
	// See HandleMediaIngest + startStatsCollector for the matching paths.
	atomic.AddUint64(&session.Stats.PacketsSent, 1)
	atomic.AddUint64(&session.Stats.BytesSent, uint64(n))
	return nil
}

func (s *Server) WriteAudioRTP(sessionID string, data []byte) error {
	session, err := s.GetSession(sessionID)
	if err != nil {
		return err
	}
	session.Mutex.RLock()
	track := session.AudioTrack
	session.Mutex.RUnlock()
	if track == nil {
		return errors.New("audio track not available")
	}
	n, err := track.Write(data)
	if err != nil {
		return err
	}
	atomic.AddUint64(&session.Stats.PacketsSent, 1)
	atomic.AddUint64(&session.Stats.BytesSent, uint64(n))
	return nil
}

func (s *Server) SendData(sessionID string, data []byte) error {
	session, err := s.GetSession(sessionID)
	if err != nil {
		return err
	}
	session.Mutex.RLock()
	defer session.Mutex.RUnlock()
	if session.DataChannel == nil {
		return errors.New("data channel not available")
	}
	return session.DataChannel.Send(data)
}

func (s *Server) selectVideoCodec(requestedCodecs []string, preset string) string {
	cPriority := map[string]int{
		"h265": 6,
		"h264": 5,
		"av1":  4,
		"vp9":  3,
		"vp8":  2,
		"h263": 1,
	}

	if preset == "fast" && s.codecSupport.HardwareAccel {
		if s.codecSupport.H265Enabled {
			return "h265"
		}
		if s.codecSupport.H264Enabled {
			return "h264"
		}
	}

	if preset == "safe" {
		if s.codecSupport.H264Enabled {
			return "h264"
		}
		if s.codecSupport.VP8Enabled {
			return "vp8"
		}
		if s.codecSupport.H263Enabled {
			return "h263"
		}
	}

	bestCodec := ""
	bestPriority := -1

	for _, codec := range requestedCodecs {
		priority, exists := cPriority[codec]
		if !exists {
			continue
		}
		if !s.isCodecSupported(codec) {
			continue
		}
		if priority > bestPriority {
			bestPriority = priority
			bestCodec = codec
		}
	}

	if bestCodec != "" {
		return bestCodec
	}

	if s.codecSupport.H264Enabled {
		return "h264"
	}
	if s.codecSupport.VP8Enabled {
		return "vp8"
	}
	if s.codecSupport.H263Enabled {
		return "h263"
	}
	return "h264"
}

func (s *Server) selectAudioCodec(requestedCodecs []string) string {
	codecPriority := map[string]int{
		"opus": 2,
		"aac":  1,
	}

	bestCodec := ""
	bestPriority := -1

	for _, codec := range requestedCodecs {
		priority, exists := codecPriority[codec]
		if !exists {
			continue
		}
		if !s.isAudioCodecSupported(codec) {
			continue
		}
		if priority > bestPriority {
			bestPriority = priority
			bestCodec = codec
		}
	}

	if bestCodec != "" {
		return bestCodec
	}
	if s.codecSupport.OpusEnabled {
		return "opus"
	}
	if s.codecSupport.AACEnabled {
		return "aac"
	}
	return "opus"
}

func (s *Server) isCodecSupported(codec string) bool {
	switch codec {
	case "h263":
		return true
	case "h264":
		return s.codecSupport.H264Enabled
	case "h265":
		return s.codecSupport.H265Enabled
	case "av1":
		return s.codecSupport.AV1Enabled
	case "vp8":
		return s.codecSupport.VP8Enabled
	case "vp9":
		return s.codecSupport.VP9Enabled
	default:
		return false
	}
}

func (s *Server) isAudioCodecSupported(codec string) bool {
	switch codec {
	case "opus":
		return s.codecSupport.OpusEnabled
	case "aac":
		return s.codecSupport.AACEnabled
	default:
		return false
	}
}

func (s *Server) getTURNServers() []string {
	if s.turnServer != "" {
		return []string{s.turnServer}
	}
	return []string{}
}

func (s *Server) getSTUNServers() []string {
	if s.stunServer != "" {
		return []string{s.stunServer}
	}
	return []string{"stun:stun.l.google.com:19302"}
}

func extractResourceID(path string) string {
	parts := splitPath(path)
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

func extractSessionID(path string) string {
	parts := splitPath(path)
	if len(parts) >= 2 && parts[0] == "signal" {
		return parts[1]
	}
	return ""
}

func splitPath(path string) []string {
	if path == "" {
		return []string{}
	}
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		return []string{}
	}
	return strings.Split(path, "/")
}

var rtpBufferPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 1500)
		return &buf
	},
}

func getRTPBuf() *[]byte {
	return rtpBufferPool.Get().(*[]byte)
}

func putRTPBuf(buf *[]byte) {
	rtpBufferPool.Put(buf)
}

func generateSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("sess_%d", time.Now().UnixNano())
	}
	return "sess_" + hex.EncodeToString(b)
}

func (s *Server) HandleMediaIngest(w http.ResponseWriter, r *http.Request) {
	sessionID := extractResourceID(r.URL.Path)
	if sessionID == "" {
		http.Error(w, "session ID required", http.StatusBadRequest)
		return
	}

	session, err := s.GetSession(sessionID)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	contentType := r.Header.Get("Content-Type")

	session.Mutex.RLock()
	videoTrack := session.VideoTrack
	audioTrack := session.AudioTrack
	videoCodec := session.VideoCodec
	session.Mutex.RUnlock()

	if strings.HasPrefix(contentType, "video/") {
		if videoTrack == nil {
			http.Error(w, "video track not available", http.StatusServiceUnavailable)
			return
		}
		var packetBuf [2]byte
		for {
			// Read 2-byte length prefix
			_, readErr := io.ReadFull(r.Body, packetBuf[:])
			if readErr != nil {
				if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
					break
				}
				s.logger.Printf("session %s video read error: %v", sessionID, readErr)
				return
			}
			pktLen := int(binary.BigEndian.Uint16(packetBuf[:]))
			if pktLen <= 0 || pktLen > 65535 {
				s.logger.Printf("session %s invalid RTP packet length %d", sessionID, pktLen)
				return
			}

			rtpBuf := getRTPBuf()
			if len(*rtpBuf) < pktLen {
				// Larger than pool buffer; allocate exact. Return the pool buffer
				// immediately so it's reused instead of leaked on every jumbo packet.
				putRTPBuf(rtpBuf)
				exact := make([]byte, pktLen)
				_, copyErr := io.ReadFull(r.Body, exact)
				if copyErr != nil {
					s.logger.Printf("session %s video read payload error: %v", sessionID, copyErr)
					return
				}
				if _, writeErr := videoTrack.Write(exact); writeErr != nil {
					if errors.Is(writeErr, io.ErrClosedPipe) || errors.Is(writeErr, io.EOF) {
						return
					}
					s.logger.Printf("session %s video write error: %v", sessionID, writeErr)
					return
				}
				// PacketsSent / BytesSent use atomics — WriteVideoRTP + stats
				// collector agree on this access mode. Holding session.Mutex
				// per packet here would also block AllocateHost readers.
				atomic.AddUint64(&session.Stats.PacketsSent, 1)
				atomic.AddUint64(&session.Stats.BytesSent, uint64(pktLen))
			} else {
				buf := *rtpBuf
				_, copyErr := io.ReadFull(r.Body, buf[:pktLen])
				if copyErr != nil {
					putRTPBuf(rtpBuf)
					s.logger.Printf("session %s video read payload error: %v", sessionID, copyErr)
					return
				}
				if _, writeErr := videoTrack.Write(buf[:pktLen]); writeErr != nil {
					putRTPBuf(rtpBuf)
					if errors.Is(writeErr, io.ErrClosedPipe) || errors.Is(writeErr, io.EOF) {
						return
					}
					s.logger.Printf("session %s video write error: %v", sessionID, writeErr)
					return
				}
				putRTPBuf(rtpBuf)
				atomic.AddUint64(&session.Stats.PacketsSent, 1)
				atomic.AddUint64(&session.Stats.BytesSent, uint64(pktLen))
			}
		}
		session.Mutex.RLock()
		encMode := session.EncodingMode
		encCfg := session.EncoderCfg
		session.Mutex.RUnlock()
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"status":       "ok",
			"codec":        videoCodec,
			"encodingMode": encMode,
			"encoder":      encCfg,
		})
	} else if strings.HasPrefix(contentType, "audio/") {
		if audioTrack == nil {
			http.Error(w, "audio track not available", http.StatusServiceUnavailable)
			return
		}
		buf := getRTPBuf()
		defer putRTPBuf(buf)
		buffer := *buf
		for {
			n, readErr := r.Body.Read(buffer)
			if n > 0 {
				if _, writeErr := audioTrack.Write(buffer[:n]); writeErr != nil {
					if errors.Is(writeErr, io.ErrClosedPipe) || errors.Is(writeErr, io.EOF) {
						return
					}
					s.logger.Printf("session %s audio write error: %v", sessionID, writeErr)
					return
				}
				atomic.AddUint64(&session.Stats.PacketsSent, 1)
				atomic.AddUint64(&session.Stats.BytesSent, uint64(n))
			}
			if readErr != nil {
				if errors.Is(readErr, io.EOF) {
					break
				}
				http.Error(w, readErr.Error(), http.StatusInternalServerError)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	} else {
		http.Error(w, "unsupported content type", http.StatusBadRequest)
	}
}

func (s *Server) HandleDataIngest(w http.ResponseWriter, r *http.Request) {
	sessionID := extractResourceID(r.URL.Path)
	if sessionID == "" {
		http.Error(w, "session ID required", http.StatusBadRequest)
		return
	}

	session, err := s.GetSession(sessionID)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Bound the body: an unbounded io.ReadAll on r.Body lets a hostile caller
	// send an arbitrarily large upload and OOM the gateway. maxMessageSize is
	// the same cap the websocket signal path uses (64 KiB) — DataChannel is
	// meant for small control messages, not bulk transfer.
	r.Body = http.MaxBytesReader(w, r.Body, maxMessageSize)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	session.Mutex.RLock()
	dc := session.DataChannel
	session.Mutex.RUnlock()

	if dc == nil {
		http.Error(w, "data channel not available", http.StatusInternalServerError)
		return
	}

	if err := dc.Send(body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) HandleRequestKeyframe(w http.ResponseWriter, r *http.Request) {
	sessionID := extractResourceID(r.URL.Path)
	if sessionID == "" {
		http.Error(w, "session ID required", http.StatusBadRequest)
		return
	}

	session, err := s.GetSession(sessionID)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	session.Mutex.RLock()
	pc := session.PeerConnection
	session.Mutex.RUnlock()

	if pc == nil {
		http.Error(w, "no peer connection", http.StatusServiceUnavailable)
		return
	}

	_ = pc.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{
		MediaSSRC: 0,
	}})

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
