package gaming

import (
	"errors"
	"log"
	"sync"
	"time"
)

type GamingManager struct {
	logger        *log.Logger
	sessions      map[string]*GamingSession
	sessionsMutex sync.RWMutex
}

type GamingSession struct {
	SessionID       string
	Active          bool
	Mode            GamingMode
	ControllerState map[string]*ControllerState
	GPUProfile      GPUProfile
	NetworkProfile  NetworkProfile
	PerformanceMetrics PerformanceMetrics
	Mutex           sync.RWMutex
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type GamingMode string

const (
	GamingModeStandard  GamingMode = "standard"
	GamingModeCompetitive GamingMode = "competitive"
	GamingModeCasual    GamingMode = "casual"
	GamingModeVR        GamingMode = "vr"
)

type ControllerState struct {
	ID              string
	Vendor          string // 'xbox', 'playstation', 'generic'
	Connected       bool
	Buttons         []ButtonState
	Axes            []float64
	Timestamp       time.Time
	RumbleEnabled   bool
	RumbleIntensity float64
	MotionData      *MotionData
}

type ButtonState struct {
	Index   int
	Pressed bool
	Value   float64
}

type MotionData struct {
	Accelerometer []float64
	Gyroscope     []float64
	Timestamp     time.Time
}

type GPUProfile struct {
	Model          string
	VRAMGB         float64
	EncoderSupport []string
	CurrentStreams int
	MaxStreams     int
	Utilization    float64
	Temperature    float64
	ClockSpeed     float64
}

type NetworkProfile struct {
	Protocol           string // 'webrtc', 'udp', 'tcp'
	PacketLoss         float64
	Jitter             float64
	RTT                float64
	Bandwidth          float64
	NetworkQuality     float64
	SimulatedLag       bool
	PacketReordering   bool
	NetworkOptimization bool
}

type PerformanceMetrics struct {
	FPS             float64
	FrameTime       float64
	Bitrate         float64
	Latency         float64
	DroppedFrames   int
	StutterCount    int
	Score           float64
	TargetFPS       int
	TargetLatency   float64
	LastSampledAt   time.Time
}

type GamingConfig struct {
	Mode                GamingMode
	TargetFPS           int
	TargetLatency       float64
	ControllerRumble    bool
	MotionControls      bool
	HDR                 bool
	RayTracing          bool
	AntiAliasing        string
	ShadowQuality       string
	TextureQuality      string
	EffectsQuality      string
	VSync               bool
	FrameRateLimit      int
	ResolutionScale     float64
	NetworkOptimization bool
}

func NewGamingManager(logger *log.Logger) *GamingManager {
	if logger == nil {
		logger = log.Default()
	}

	return &GamingManager{
		logger:   logger,
		sessions: make(map[string]*GamingSession),
	}
}

func (m *GamingManager) CreateGamingSession(sessionID string, config GamingConfig) (*GamingSession, error) {
	session := &GamingSession{
		SessionID:       sessionID,
		Active:          true,
		Mode:            config.Mode,
		ControllerState: make(map[string]*ControllerState),
		GPUProfile:      GPUProfile{MaxStreams: 10},
		NetworkProfile:  NetworkProfile{Protocol: "webrtc"},
		PerformanceMetrics: PerformanceMetrics{
			TargetFPS:     config.TargetFPS,
			TargetLatency: config.TargetLatency,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.sessionsMutex.Lock()
	m.sessions[sessionID] = session
	m.sessionsMutex.Unlock()

	m.logger.Printf("created gaming session %s (mode=%s, targetFPS=%d)", sessionID, config.Mode, config.TargetFPS)

	return session, nil
}

func (m *GamingManager) GetGamingSession(sessionID string) (*GamingSession, error) {
	m.sessionsMutex.RLock()
	defer m.sessionsMutex.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, errors.New("gaming session not found")
	}

	return session, nil
}

func (m *GamingManager) EndGamingSession(sessionID string) error {
	m.sessionsMutex.Lock()
	defer m.sessionsMutex.Unlock()

	if _, exists := m.sessions[sessionID]; exists {
		delete(m.sessions, sessionID)
		m.logger.Printf("ended gaming session %s", sessionID)
		return nil
	}

	return errors.New("gaming session not found")
}

func (m *GamingManager) RegisterController(sessionID string, controllerID string, vendor string) error {
	session, err := m.GetGamingSession(sessionID)
	if err != nil {
		return err
	}

	session.Mutex.Lock()
	defer session.Mutex.Unlock()

	controller := &ControllerState{
		ID:        controllerID,
		Vendor:    vendor,
		Connected: true,
		Buttons:   make([]ButtonState, 20),
		Axes:      make([]float64, 6),
		Timestamp: time.Now(),
	}

	session.ControllerState[controllerID] = controller
	session.UpdatedAt = time.Now()

	m.logger.Printf("registered controller %s (%s) for session %s", controllerID, vendor, sessionID)

	return nil
}

func (m *GamingManager) UnregisterController(sessionID string, controllerID string) error {
	session, err := m.GetGamingSession(sessionID)
	if err != nil {
		return err
	}

	session.Mutex.Lock()
	defer session.Mutex.Unlock()

	if controller, exists := session.ControllerState[controllerID]; exists {
		controller.Connected = false
		session.ControllerState[controllerID] = controller
		session.UpdatedAt = time.Now()
		m.logger.Printf("unregistered controller %s for session %s", controllerID, sessionID)
	}

	return nil
}

func (m *GamingManager) UpdateControllerState(sessionID string, controllerID string, buttons []ButtonState, axes []float64) error {
	session, err := m.GetGamingSession(sessionID)
	if err != nil {
		return err
	}

	session.Mutex.Lock()
	defer session.Mutex.Unlock()

	controller, exists := session.ControllerState[controllerID]
	if !exists {
		return errors.New("controller not found")
	}

	controller.Buttons = buttons
	controller.Axes = axes
	controller.Timestamp = time.Now()
	session.ControllerState[controllerID] = controller
	session.UpdatedAt = time.Now()

	return nil
}

func (m *GamingManager) SetControllerRumble(sessionID string, controllerID string, enabled bool, intensity float64) error {
	session, err := m.GetGamingSession(sessionID)
	if err != nil {
		return err
	}

	session.Mutex.Lock()
	defer session.Mutex.Unlock()

	controller, exists := session.ControllerState[controllerID]
	if !exists {
		return errors.New("controller not found")
	}

	controller.RumbleEnabled = enabled
	controller.RumbleIntensity = intensity
	session.ControllerState[controllerID] = controller
	session.UpdatedAt = time.Now()

	m.logger.Printf("set rumble for controller %s (enabled=%v, intensity=%.2f)", controllerID, enabled, intensity)

	return nil
}

func (m *GamingManager) UpdateMotionData(sessionID string, controllerID string, accelerometer []float64, gyroscope []float64) error {
	session, err := m.GetGamingSession(sessionID)
	if err != nil {
		return err
	}

	session.Mutex.Lock()
	defer session.Mutex.Unlock()

	controller, exists := session.ControllerState[controllerID]
	if !exists {
		return errors.New("controller not found")
	}

	controller.MotionData = &MotionData{
		Accelerometer: accelerometer,
		Gyroscope:     gyroscope,
		Timestamp:     time.Now(),
	}
	session.ControllerState[controllerID] = controller
	session.UpdatedAt = time.Now()

	return nil
}

func (m *GamingManager) UpdatePerformanceMetrics(sessionID string, fps float64, frameTime float64, bitrate float64, latency float64) error {
	session, err := m.GetGamingSession(sessionID)
	if err != nil {
		return err
	}

	session.Mutex.Lock()
	defer session.Mutex.Unlock()

	metrics := &session.PerformanceMetrics
	metrics.FPS = fps
	metrics.FrameTime = frameTime
	metrics.Bitrate = bitrate
	metrics.Latency = latency
	metrics.LastSampledAt = time.Now()

	// Calculate performance score
	metrics.Score = m.calculatePerformanceScore(metrics)

	// Detect stutter
	if frameTime > (1000.0/float64(metrics.TargetFPS))*2 {
		metrics.StutterCount++
	}

	session.UpdatedAt = time.Now()

	return nil
}

func (m *GamingManager) calculatePerformanceMetrics(sessionID string) (*PerformanceMetrics, error) {
	session, err := m.GetGamingSession(sessionID)
	if err != nil {
		return nil, err
	}

	session.Mutex.RLock()
	defer session.Mutex.RUnlock()

	return &session.PerformanceMetrics, nil
}

func (m *GamingManager) OptimizeForMode(sessionID string, mode GamingMode) error {
	session, err := m.GetGamingSession(sessionID)
	if err != nil {
		return err
	}

	session.Mutex.Lock()
	defer session.Mutex.Unlock()

	switch mode {
	case GamingModeCompetitive:
		// Prioritize low latency over visual quality
		session.PerformanceMetrics.TargetLatency = 10 // 10ms target
		session.PerformanceMetrics.TargetFPS = 144
		session.NetworkProfile.NetworkOptimization = true
	case GamingModeStandard:
		// Balance between quality and performance
		session.PerformanceMetrics.TargetLatency = 20 // 20ms target
		session.PerformanceMetrics.TargetFPS = 60
	case GamingModeCasual:
		// Prioritize visual quality
		session.PerformanceMetrics.TargetLatency = 50 // 50ms target
		session.PerformanceMetrics.TargetFPS = 30
	case GamingModeVR:
		// VR requires very low latency and high FPS
		session.PerformanceMetrics.TargetLatency = 5 // 5ms target
		session.PerformanceMetrics.TargetFPS = 90
	}

	session.Mode = mode
	session.UpdatedAt = time.Now()

	m.logger.Printf("optimized session %s for mode %s", sessionID, mode)

	return nil
}

func (m *GamingManager) calculatePerformanceScore(metrics *PerformanceMetrics) float64 {
	score := 0.0

	// FPS score (0-40 points)
	fpsScore := (metrics.FPS / float64(metrics.TargetFPS)) * 40
	if fpsScore > 40 {
		fpsScore = 40
	}
	score += fpsScore

	// Latency score (0-30 points)
	latencyScore := (1 - (metrics.Latency / metrics.TargetLatency)) * 30
	if latencyScore < 0 {
		latencyScore = 0
	}
	score += latencyScore

	// Bitrate score (0-20 points)
	bitrateScore := (metrics.Bitrate / 20000) * 20 // Assume 20Mbps is excellent
	if bitrateScore > 20 {
		bitrateScore = 20
	}
	score += bitrateScore

	// Stability score (0-10 points)
	stabilityScore := 10 - float64(metrics.StutterCount)
	if stabilityScore < 0 {
		stabilityScore = 0
	}
	score += stabilityScore

	return score
}

func (m *GamingManager) GetControllerState(sessionID string, controllerID string) (*ControllerState, error) {
	session, err := m.GetGamingSession(sessionID)
	if err != nil {
		return nil, err
	}

	session.Mutex.RLock()
	defer session.Mutex.RUnlock()

	controller, exists := session.ControllerState[controllerID]
	if !exists {
		return nil, errors.New("controller not found")
	}

	return controller, nil
}

func (m *GamingManager) GetAllControllers(sessionID string) ([]*ControllerState, error) {
	session, err := m.GetGamingSession(sessionID)
	if err != nil {
		return nil, err
	}

	session.Mutex.RLock()
	defer session.Mutex.RUnlock()

	controllers := make([]*ControllerState, 0, len(session.ControllerState))
	for _, controller := range session.ControllerState {
		if controller.Connected {
			controllers = append(controllers, controller)
		}
	}

	return controllers, nil
}

func (m *GamingManager) UpdateGPUProfile(sessionID string, utilization float64, temperature float64, clockSpeed float64) error {
	session, err := m.GetGamingSession(sessionID)
	if err != nil {
		return err
	}

	session.Mutex.Lock()
	defer session.Mutex.Unlock()

	session.GPUProfile.Utilization = utilization
	session.GPUProfile.Temperature = temperature
	session.GPUProfile.ClockSpeed = clockSpeed
	session.UpdatedAt = time.Now()

	return nil
}

func (m *GamingManager) UpdateNetworkProfile(sessionID string, packetLoss float64, jitter float64, rtt float64, bandwidth float64) error {
	session, err := m.GetGamingSession(sessionID)
	if err != nil {
		return err
	}

	session.Mutex.Lock()
	defer session.Mutex.Unlock()

	session.NetworkProfile.PacketLoss = packetLoss
	session.NetworkProfile.Jitter = jitter
	session.NetworkProfile.RTT = rtt
	session.NetworkProfile.Bandwidth = bandwidth
	session.NetworkProfile.NetworkQuality = m.calculateNetworkQuality(packetLoss, jitter, rtt)
	session.UpdatedAt = time.Now()

	return nil
}

func (m *GamingManager) calculateNetworkQuality(packetLoss float64, jitter float64, rtt float64) float64 {
	quality := 100.0

	// Penalize packet loss
	quality -= packetLoss * 100

	// Penalize jitter
	quality -= jitter * 2

	// Penalize RTT
	quality -= rtt * 0.5

	if quality < 0 {
		quality = 0
	}

	return quality
}