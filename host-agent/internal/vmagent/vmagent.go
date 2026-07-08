package vmagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
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

type VMAgent struct {
	Config     VMAgentConfig
	GatewayURL string
	Token      string
	Routes     []RouteConfig
	Logger     *log.Logger
	Encoders   *EncoderManager
}

type EncoderManager struct {
	GPUModel         string
	GPUVendor        string
	HardwareAccel    bool
	AvailableCodecs  []string
	mu               sync.Mutex
	activeStreams    int
}

type EncoderSession struct {
	ID            string
	Codec         string
	Mode          EncodingMode
	Preset        EncoderPreset
	Width         int
	Height        int
	FPS           int
	BitrateKbps   int
	CRF           int
	HardwareAccel bool
	cmd           *exec.Cmd
	stdin         io.WriteCloser
	cancel        context.CancelFunc
}

type VMAgentConfig struct {
	VMID               string
	Region             string
	GPUEnabled         bool
	GPUModel           string
	VPARAM             float64
	CPUcores           int
	RAMGB              int
	StorageGB          int
	NetworkInterface   string
	IPAddress          string
	EnableWebRTC       bool
	EnableGaming       bool
	EnableRemoteSupport bool
	DisplayPort        int
	AudioEnabled       bool
}

type RouteConfig struct {
	ID       string `json:"id"`
	Protocol string `json:"protocol"`
	Target   string `json:"target"`
	Port     int    `json:"port"`
	PoolSize int    `json:"poolSize"`
}

type HostCapabilities struct {
	ResourceClasses       []string `json:"resourceClasses"`
	GPUScheduling         bool     `json:"gpuScheduling"`
	HardwareAcceleration  bool     `json:"hardwareAcceleration"`
	ControllerPassthrough bool     `json:"controllerPassthrough"`
	AudioForwarding       bool     `json:"audioForwarding"`
	FileTransfer          bool     `json:"fileTransfer"`
	RemoteSupport         bool     `json:"remoteSupport"`
	WebRTC                bool     `json:"webrtc"`
	TCPProxy              bool     `json:"tcpProxy"`
	UDPProxy              bool     `json:"udpProxy"`
	SSHProxy              bool     `json:"sshProxy"`
}

type HostMetrics struct {
	HostID             string  `json:"hostId"`
	CPUUsagePercent    float64 `json:"cpuUsagePercent"`
	RAMUsagePercent    float64 `json:"ramUsagePercent"`
	GPUUsagePercent    float64 `json:"gpuUsagePercent"`
	GPUVRAMUsedGB      float64 `json:"gpuVramUsedGb"`
	StorageUsedGB      float64 `json:"storageUsedGb"`
	ActiveVMs          int     `json:"activeVMs"`
	ActiveStreams      int     `json:"activeStreams"`
	ActiveProxyRoutes  int     `json:"activeProxyRoutes"`
	EncoderUsagePercent float64 `json:"encoderUsagePercent"`
	NetworkTxMbps      float64 `json:"networkTxMbps"`
	NetworkRxMbps      float64 `json:"networkRxMbps"`
	P95LatencyMs       float64 `json:"p95LatencyMs"`
	Timestamp          string  `json:"timestamp"`
}

func NewVMAgent(config VMAgentConfig, gatewayURL string, token string, routes []RouteConfig) *VMAgent {
	agent := &VMAgent{
		Config:     config,
		GatewayURL: gatewayURL,
		Token:      token,
		Routes:     routes,
		Logger:     log.New(os.Stdout, "vm-agent ", log.LstdFlags|log.Lmicroseconds),
	}
	agent.Encoders = NewEncoderManager(config)
	return agent
}

func NewEncoderManager(config VMAgentConfig) *EncoderManager {
	mgr := &EncoderManager{
		AvailableCodecs: []string{},
	}

	if runtime.GOOS == "linux" {
		mgr.detectLinuxEncoders()
	} else {
		mgr.AvailableCodecs = append(mgr.AvailableCodecs, "h264")
		mgr.HardwareAccel = false
	}

	if len(mgr.AvailableCodecs) == 0 {
		mgr.AvailableCodecs = append(mgr.AvailableCodecs, "h264")
	}

	return mgr
}

func (m *EncoderManager) detectLinuxEncoders() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := exec.Command("nvidia-smi").Run(); err == nil {
		m.GPUVendor = "nvidia"
		m.HardwareAccel = true
		m.AvailableCodecs = []string{"h264", "h265", "av1", "h263"}

		out, _ := exec.Command("nvidia-smi", "--query-gpu=name", "--format=csv,noheader").Output()
		if len(out) > 0 {
			m.GPUModel = strings.TrimSpace(string(out))
		}
		m.Logger().Printf("NVIDIA GPU %s detected: HW encoders: h264_nvenc, hevc_nvenc, av1_nvenc", m.GPUModel)
		return
	}

	if err := exec.Command("vainfo").Run(); err == nil {
		m.GPUVendor = "intel"
		m.HardwareAccel = true
		m.AvailableCodecs = []string{"h264", "h265"}

		out, _ := exec.Command("cat", "/proc/cpuinfo").Output()
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "model name") {
				parts := strings.Split(line, ":")
				if len(parts) == 2 {
					m.GPUModel = strings.TrimSpace(parts[1])
				}
				break
			}
		}
		m.Logger().Printf("Intel GPU detected: HW encoders via VAAPI")
		return
	}

	m.GPUVendor = "none"
	m.HardwareAccel = false
	m.AvailableCodecs = []string{"h264", "h265", "h263", "vp8", "vp9", "av1"}
	m.Logger().Printf("No HW encoder found: using software encoders (libx264, libx265, etc.)")
}

func (m *EncoderManager) Logger() *log.Logger {
	return log.New(os.Stdout, "encoder-manager ", log.LstdFlags|log.Lmicroseconds)
}

func (m *EncoderManager) SupportsCodec(codec string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.AvailableCodecs {
		if c == codec {
			return true
		}
	}
	return false
}

func (m *EncoderManager) ActiveStreamCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.activeStreams
}

func (m *EncoderManager) EncoderUsagePercent() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.activeStreams == 0 {
		return 0
	}
	return float64(m.activeStreams) * 15.0
}

func (a *VMAgent) Start(ctx context.Context) error {
	a.Logger.Printf("starting VM agent for VM %s in region %s", a.Config.VMID, a.Config.Region)

	// Register with gateway
	if err := a.registerWithGateway(); err != nil {
		return fmt.Errorf("failed to register with gateway: %w", err)
	}

	// Start metrics reporting
	go a.reportMetricsLoop(ctx)

	// Start connection tunnels
	for _, route := range a.Routes {
		go a.startTunnelForRoute(ctx, route)
	}

	// Start desktop streaming if enabled
	if a.Config.EnableWebRTC {
		go a.startDesktopStreaming(ctx)
	}

	// Start gaming mode if enabled
	if a.Config.EnableGaming {
		go a.startGamingMode(ctx)
	}

	a.Logger.Printf("VM agent started successfully")

	return nil
}

func (a *VMAgent) registerWithGateway() error {
	payload, err := a.registrationPayload()
	if err != nil {
		return err
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", a.GatewayURL+"/hosts/register", bytes.NewReader(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.Token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("registration failed with status %d: %s", resp.StatusCode, string(body))
	}

	a.Logger.Printf("successfully registered VM %s with gateway", a.Config.VMID)

	return nil
}

func (a *VMAgent) registrationPayload() (map[string]interface{}, error) {
	capabilities := HostCapabilities{
		ResourceClasses:       []string{"basic", "standard", "gaming", "workstation"},
		GPUScheduling:         a.Config.GPUEnabled,
		HardwareAcceleration:  a.Config.GPUEnabled,
		ControllerPassthrough: a.Config.EnableGaming,
		AudioForwarding:       a.Config.AudioEnabled,
		FileTransfer:          true,
		RemoteSupport:         a.Config.EnableRemoteSupport,
		WebRTC:                a.Config.EnableWebRTC,
		TCPProxy:              true,
		UDPProxy:              true,
		SSHProxy:              true,
	}

	return map[string]interface{}{
		"vmId":         a.Config.VMID,
		"region":       a.Config.Region,
		"capabilities": capabilities,
		"gpu": map[string]interface{}{
			"model":  a.Config.GPUModel,
			"vramGb": a.Config.VPARAM,
		},
		"resources": map[string]interface{}{
			"cpuCores":  a.Config.CPUcores,
			"ramGb":     a.Config.RAMGB,
			"storageGb": a.Config.StorageGB,
		},
		"network": map[string]interface{}{
			"ipAddress": a.Config.IPAddress,
		},
	}, nil
}

func (a *VMAgent) startTunnelForRoute(ctx context.Context, route RouteConfig) {
	a.Logger.Printf("starting tunnel for route %s (%s://%s:%d)", route.ID, route.Protocol, route.Target, route.Port)

	for i := 0; i < route.PoolSize; i++ {
		go a.runTunnelConnection(ctx, route, i)
	}
}

func (a *VMAgent) runTunnelConnection(ctx context.Context, route RouteConfig, slot int) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if err := a.establishTunnelConnection(route); err != nil {
				a.Logger.Printf("tunnel connection failed for route %s slot %d: %v", route.ID, slot, err)
			}
			time.Sleep(5 * time.Second)
		}
	}
}

func (a *VMAgent) establishTunnelConnection(route RouteConfig) error {
	target := fmt.Sprintf("%s:%d", route.Target, route.Port)

	// This would use the existing host-agent tunnel logic
	// For now, we'll simulate the connection
	a.Logger.Printf("establishing tunnel connection to %s", target)

	// Simulate connection establishment
	time.Sleep(100 * time.Millisecond)

	return nil
}

func (a *VMAgent) startDesktopStreaming(ctx context.Context) {
	a.Logger.Printf("starting desktop streaming for VM %s", a.Config.VMID)

	if runtime.GOOS == "linux" {
		if _, err := exec.LookPath("X"); err == nil {
			a.Logger.Printf("X11 display server detected")
		}
		if _, err := exec.LookPath("wayland"); err == nil {
			a.Logger.Printf("Wayland display server detected")
		}
	}

	sessionID := fmt.Sprintf("desktop-%s", a.Config.VMID)
	session := a.createEncoderSession(sessionID, "h264", EncodingModeBalanced, 1920, 1080, 30, 5000)
	if session != nil {
		go a.runEncodingSession(ctx, session)
	}

	if a.Config.AudioEnabled {
		go a.runAudioCapture(ctx)
	}
}

func (a *VMAgent) createEncoderSession(id, codec string, mode EncodingMode, width, height, fps, bitrateKbps int) *EncoderSession {
	if a.Encoders == nil {
		a.Logger.Printf("encoder manager not initialized")
		return nil
	}

	if !a.Encoders.SupportsCodec(codec) {
		a.Logger.Printf("codec %s not supported, falling back to h264", codec)
		codec = "h264"
	}

	preset := encoderPresetForMode(mode, a.Encoders.HardwareAccel)
	crf := crfForMode(mode)

	a.Encoders.mu.Lock()
	a.Encoders.activeStreams++
	count := a.Encoders.activeStreams
	a.Encoders.mu.Unlock()

	a.Logger.Printf("created encoder session %s: codec=%s mode=%s preset=%s %dx%d@%dfps %dkbps (active streams: %d)",
		id, codec, mode, preset, width, height, fps, bitrateKbps, count)

	return &EncoderSession{
		ID:            id,
		Codec:         codec,
		Mode:          mode,
		Preset:        preset,
		Width:         width,
		Height:        height,
		FPS:           fps,
		BitrateKbps:   bitrateKbps,
		CRF:           crf,
		HardwareAccel: a.Encoders.HardwareAccel,
	}
}

func encoderPresetForMode(mode EncodingMode, hardwareAccel bool) EncoderPreset {
	if hardwareAccel {
		switch mode {
		case EncodingModeSpeed:
			return EncoderPresetFast
		case EncodingModeQuality:
			return EncoderPresetMedium
		default:
			return EncoderPresetFast
		}
	}

	switch mode {
	case EncodingModeSpeed:
		return EncoderPresetVeryfast
	case EncodingModeQuality:
		return EncoderPresetSlow
	default:
		return EncoderPresetMedium
	}
}

func crfForMode(mode EncodingMode) int {
	switch mode {
	case EncodingModeSpeed:
		return 28
	case EncodingModeQuality:
		return 18
	default:
		return 23
	}
}

func (a *VMAgent) runEncodingSession(ctx context.Context, session *EncoderSession) {
	a.Logger.Printf("starting encoding session %s (%s, %s)", session.ID, session.Codec, session.Mode)

	display := fmt.Sprintf(":%d", a.Config.DisplayPort)
	if a.Config.DisplayPort == 0 {
		display = ":0"
	}

	var encoderName string
	switch {
	case session.HardwareAccel && a.Encoders.GPUVendor == "nvidia":
		switch session.Codec {
		case "h264":
			encoderName = "h264_nvenc"
		case "h265":
			encoderName = "hevc_nvenc"
		case "av1":
			encoderName = "av1_nvenc"
		default:
			encoderName = "h264_nvenc"
		}
	case session.HardwareAccel && a.Encoders.GPUVendor == "intel":
		switch session.Codec {
		case "h264":
			encoderName = "h264_vaapi"
		case "h265":
			encoderName = "hevc_vaapi"
		default:
			encoderName = "h264_vaapi"
		}
	default:
		switch session.Codec {
		case "h264":
			encoderName = "libx264"
		case "h265":
			encoderName = "libx265"
		case "h263":
			encoderName = "libx264"
		case "vp8":
			encoderName = "libvpx"
		case "vp9":
			encoderName = "libvpx-vp9"
		case "av1":
			encoderName = "libaom-av1"
		default:
			encoderName = "libx264"
		}
	}

	args := a.buildFFmpegArgs(encoderName, session)
	a.Logger.Printf("ffmpeg command: ffmpeg %s", strings.Join(args, " "))

	sessionCtx, cancel := context.WithCancel(ctx)
	session.cancel = cancel

	cmd := exec.CommandContext(sessionCtx, "ffmpeg", args...)

	if runtime.GOOS == "linux" {
		cmd.Env = append(os.Environ(), "DISPLAY="+display)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		a.Logger.Printf("encoder session %s: stdout pipe error: %v", session.ID, err)
		a.cleanupEncoderSession(session)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		a.Logger.Printf("encoder session %s: stderr pipe error: %v", session.ID, err)
		a.cleanupEncoderSession(session)
		return
	}

	if err := cmd.Start(); err != nil {
		a.Logger.Printf("encoder session %s: ffmpeg start error: %v", session.ID, err)
		a.cleanupEncoderSession(session)
		return
	}

	session.cmd = cmd
	session.stdin, _ = cmd.StdinPipe()

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				_ = a.sendEncodedData(session.ID, buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	go func() {
		stderrBuf := make([]byte, 4096)
		for {
			n, err := stderr.Read(stderrBuf)
			if n > 0 {
				a.Logger.Printf("encoder [%s]: %s", session.ID, string(stderrBuf[:n]))
			}
			if err != nil {
				return
			}
		}
	}()

	a.Logger.Printf("encoder session %s started (pid=%d)", session.ID, cmd.Process.Pid)

	go func() {
		if err := cmd.Wait(); err != nil {
			if sessionCtx.Err() == nil {
				a.Logger.Printf("encoder session %s exited: %v", session.ID, err)
			}
		}
		a.cleanupEncoderSession(session)
	}()

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-sessionCtx.Done():
				return
			case <-ticker.C:
				if cmd.Process != nil {
					a.Logger.Printf("encoder session %s: running (codec=%s mode=%s)", session.ID, session.Codec, session.Mode)
				}
			}
		}
	}()
}

func (a *VMAgent) buildFFmpegArgs(encoderName string, session *EncoderSession) []string {
	var inputArgs []string

	if runtime.GOOS == "linux" {
		if _, err := exec.LookPath("X"); err == nil {
			inputArgs = []string{
				"-f", "x11grab",
				"-s", fmt.Sprintf("%dx%d", session.Width, session.Height),
				"-framerate", fmt.Sprintf("%d", session.FPS),
				"-i", fmt.Sprintf(":%d.0", a.Config.DisplayPort),
			}
		} else {
			inputArgs = []string{
				"-f", "lavfi",
				"-i", fmt.Sprintf("testsrc=size=%dx%d:rate=%d", session.Width, session.Height, session.FPS),
			}
		}
	} else {
		inputArgs = []string{
			"-f", "lavfi",
			"-i", fmt.Sprintf("testsrc=size=%dx%d:rate=%d", session.Width, session.Height, session.FPS),
		}
	}

	outputArgs := []string{
		"-c:v", encoderName,
		"-b:v", fmt.Sprintf("%dk", session.BitrateKbps),
	}

	if session.CRF > 0 && !session.HardwareAccel {
		outputArgs = append(outputArgs, "-crf", fmt.Sprintf("%d", session.CRF))
	}

	if !session.HardwareAccel {
		switch session.Codec {
		case "h264", "h265":
			outputArgs = append(outputArgs, "-preset", string(session.Preset))
		case "vp9":
			switch session.Mode {
			case EncodingModeSpeed:
				outputArgs = append(outputArgs, "-deadline", "realtime", "-cpu-used", "5")
			case EncodingModeQuality:
				outputArgs = append(outputArgs, "-deadline", "good", "-cpu-used", "0")
			default:
				outputArgs = append(outputArgs, "-deadline", "realtime", "-cpu-used", "2")
			}
		}
	} else {
		switch session.Mode {
		case EncodingModeSpeed:
			outputArgs = append(outputArgs, "-preset", "p1")
		case EncodingModeQuality:
			outputArgs = append(outputArgs, "-preset", "p7")
		default:
			outputArgs = append(outputArgs, "-preset", "p4")
		}
	}

	if session.Codec == "h263" {
		outputArgs = append(outputArgs, "-profile:v", "baseline")
		outputArgs = append(outputArgs, "-x264-params", "annexb=1")
	}

	outputArgs = append(outputArgs,
		"-pix_fmt", "yuv420p",
		"-g", fmt.Sprintf("%d", session.FPS*2),
		"-f", "mpegts",
		"pipe:1",
	)

	args := append(inputArgs, outputArgs...)
	return args
}

func (a *VMAgent) sendEncodedData(sessionID string, data []byte) error {
	req, err := http.NewRequest("POST",
		a.GatewayURL+"/media/"+sessionID,
		bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "video/MP2T")
	req.Header.Set("Authorization", "Bearer "+a.Token)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (a *VMAgent) cleanupEncoderSession(session *EncoderSession) {
	if session.cancel != nil {
		session.cancel()
	}
	if session.cmd != nil && session.cmd.Process != nil {
		session.cmd.Process.Kill()
	}
	if a.Encoders != nil {
		a.Encoders.mu.Lock()
		if a.Encoders.activeStreams > 0 {
			a.Encoders.activeStreams--
		}
		count := a.Encoders.activeStreams
		a.Encoders.mu.Unlock()
		a.Logger.Printf("encoder session %s cleaned up (active streams: %d)", session.ID, count)
	}
}

func (a *VMAgent) runAudioCapture(ctx context.Context) {
	a.Logger.Printf("starting audio capture")

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *VMAgent) startGamingMode(ctx context.Context) {
	a.Logger.Printf("starting gaming mode for VM %s", a.Config.VMID)

	if a.Config.GPUEnabled {
		sessionID := fmt.Sprintf("gaming-%s", a.Config.VMID)
		session := a.createEncoderSession(sessionID, "h265", EncodingModeSpeed, 1920, 1080, 60, 8000)
		if session != nil {
			go a.runEncodingSession(ctx, session)
		}
	}

	go a.listenForControllerInput(ctx)
}

func (a *VMAgent) listenForControllerInput(ctx context.Context) {
	a.Logger.Printf("listening for controller input")

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *VMAgent) reportMetricsLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			metrics := a.collectMetrics()
			if err := a.sendMetrics(metrics); err != nil {
				a.Logger.Printf("failed to send metrics: %v", err)
			}
		}
	}
}

func (a *VMAgent) collectMetrics() HostMetrics {
	// Collect CPU usage
	cpuUsage := a.getCPUUsage()

	// Collect RAM usage
	ramUsage := a.getRAMUsage()

	// Collect GPU usage if available
	gpuUsage := 0.0
	if a.Config.GPUEnabled {
		gpuUsage = a.getGPUUsage()
	}

	// Collect network stats
	networkTx, networkRx := a.getNetworkStats()

	return HostMetrics{
		HostID:             a.Config.VMID,
		CPUUsagePercent:    cpuUsage,
		RAMUsagePercent:    ramUsage,
		GPUUsagePercent:    gpuUsage,
		GPUVRAMUsedGB:      a.getGPUVRAMUsage(),
		StorageUsedGB:      a.getStorageUsage(),
		ActiveVMs:          1,
		ActiveStreams:      a.getActiveStreamCount(),
		ActiveProxyRoutes:  len(a.Routes),
		EncoderUsagePercent: gpuUsage, // Approximate
		NetworkTxMbps:      networkTx,
		NetworkRxMbps:      networkRx,
		P95LatencyMs:       a.measureLatency(),
		Timestamp:          time.Now().Format(time.RFC3339),
	}
}

func (a *VMAgent) getCPUUsage() float64 {
	// Simplified CPU usage calculation
	// In production, this would use proper system metrics
	return 25.0
}

func (a *VMAgent) getRAMUsage() float64 {
	// Simplified RAM usage calculation
	// In production, this would use proper system metrics
	return 45.0
}

func (a *VMAgent) getGPUUsage() float64 {
	// Simplified GPU usage calculation
	// In production, this would use proper GPU metrics
	if a.Config.GPUEnabled {
		return 60.0
	}
	return 0.0
}

func (a *VMAgent) getGPUVRAMUsage() float64 {
	// Simplified VRAM usage calculation
	if a.Config.GPUEnabled {
		return a.Config.VPARAM * 0.5
	}
	return 0.0
}

func (a *VMAgent) getStorageUsage() float64 {
	// Simplified storage usage calculation
	return float64(a.Config.StorageGB) * 0.3
}

func (a *VMAgent) getActiveStreamCount() int {
	if a.Encoders != nil {
		return a.Encoders.ActiveStreamCount()
	}
	return 0
}

func (a *VMAgent) getNetworkStats() (tx, rx float64) {
	// Simplified network stats
	return 10.5, 25.3
}

func (a *VMAgent) measureLatency() float64 {
	start := time.Now()
	gatewayHost := a.GatewayURL
	gatewayHost = strings.TrimPrefix(gatewayHost, "http://")
	gatewayHost = strings.TrimPrefix(gatewayHost, "https://")
	if !strings.Contains(gatewayHost, ":") {
		gatewayHost += ":80"
	}
	conn, err := net.DialTimeout("tcp", gatewayHost, 1*time.Second)
	if err != nil {
		return 100.0
	}
	conn.Close()
	latency := time.Since(start).Milliseconds()
	return float64(latency)
}

func (a *VMAgent) sendMetrics(metrics HostMetrics) error {
	jsonData, err := json.Marshal(metrics)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", a.GatewayURL+"/hosts/metrics", bytes.NewReader(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.Token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("metrics submission failed with status %d", resp.StatusCode)
	}

	return nil
}

// SelfTest registers with the gateway (printing the registration payload if the
// gateway is unreachable) and emits one metrics report. It returns after the
// single pass; callers typically exit after invoking it.
func (a *VMAgent) SelfTest(ctx context.Context) error {
	a.Logger.Printf("self-test: registering with gateway %s", a.GatewayURL)

	payload, err := a.registrationPayload()
	if err != nil {
		return err
	}
	payloadJSON, _ := json.Marshal(payload)

	if err := a.registerWithGateway(); err != nil {
		a.Logger.Printf("self-test: registration failed (%v)", err)
		a.Logger.Printf("self-test: registration payload: %s", string(payloadJSON))
	} else {
		a.Logger.Printf("self-test: registration successful")
	}

	metrics := a.collectMetrics()
	a.Logger.Printf("self-test: metrics: %+v", metrics)

	if err := a.sendMetrics(metrics); err != nil {
		a.Logger.Printf("self-test: metrics submission failed: %v", err)
	} else {
		a.Logger.Printf("self-test: metrics submitted successfully")
	}

	return nil
}

// LoadVMConfig reads VM agent configuration from the environment, falling back
// to the documented test defaults when a variable is unset or empty.
//
// Returns the resolved VMAgentConfig, the route list, the gateway URL, and the
// agent token, in that order.
func LoadVMConfig() (VMAgentConfig, []RouteConfig, string, string) {
	config := VMAgentConfig{
		VMID:                envOr("FREECOMPUTE_VM_ID", "vm-test-001"),
		Region:              envOr("FREECOMPUTE_VM_REGION", "us-east-1"),
		GPUEnabled:          envBool("FREECOMPUTE_VM_GPU_ENABLED", true),
		GPUModel:            os.Getenv("FREECOMPUTE_VM_GPU_MODEL"),
		VPARAM:              envFloat("FREECOMPUTE_VM_GPU_VRAM", 0),
		CPUcores:            envInt("FREECOMPUTE_VM_CPUCORES", 8),
		RAMGB:               envInt("FREECOMPUTE_VM_RAMGB", 32),
		StorageGB:           envInt("FREECOMPUTE_VM_STORAGEGB", 500),
		EnableWebRTC:        envBool("FREECOMPUTE_VM_ENABLE_WEBRTC", true),
		EnableGaming:        envBool("FREECOMPUTE_VM_ENABLE_GAMING", true),
		EnableRemoteSupport: envBool("FREECOMPUTE_VM_ENABLE_REMOTE_SUPPORT", true),
		DisplayPort:         envInt("FREECOMPUTE_VM_DISPLAY_PORT", 0),
		AudioEnabled:        envBool("FREECOMPUTE_VM_AUDIO", true),
	}

	routes := defaultRoutes()
	if raw := strings.TrimSpace(os.Getenv("FREECOMPUTE_VM_ROUTES")); raw != "" {
		var parsed []RouteConfig
		if err := json.Unmarshal([]byte(raw), &parsed); err == nil && len(parsed) > 0 {
			routes = parsed
		} else if err != nil {
			log.Printf("vm-agent: ignoring invalid FREECOMPUTE_VM_ROUTES: %v", err)
		}
	}

	gatewayURL := envOr("FREECOMPUTE_GATEWAY_URL", "http://localhost:8080")
	token := envOr("FREECOMPUTE_AGENT_TOKEN", "dev-token")

	return config, routes, gatewayURL, token
}

// PrintDryRun writes the resolved configuration, routes, and a note about the
// launch that would occur to stdout. It is used by the --dry-run flag.
func PrintDryRun(config VMAgentConfig, routes []RouteConfig, gatewayURL, token string) {
	redacted := token
	if redacted != "" {
		redacted = "<set>"
	}

	fmt.Println("=== VM Agent Dry Run ===")
	fmt.Printf("GatewayURL: %s\n", gatewayURL)
	fmt.Printf("Token:      %s\n", redacted)
	fmt.Println()

	cfgJSON, _ := json.MarshalIndent(config, "", "  ")
	fmt.Println("VMAgentConfig:")
	fmt.Println(string(cfgJSON))
	fmt.Println()

	routesJSON, _ := json.MarshalIndent(routes, "", "  ")
	fmt.Println("Routes:")
	fmt.Println(string(routesJSON))
	fmt.Println()

	fmt.Println("would launch VM agent (QEMU args live in host-agent cmd)")
}

func defaultRoutes() []RouteConfig {
	return []RouteConfig{
		{
			ID:       "desktop-ssh",
			Protocol: "ssh",
			Target:   "127.0.0.1",
			Port:     22,
			PoolSize: 4,
		},
		{
			ID:       "desktop-http",
			Protocol: "http",
			Target:   "127.0.0.1",
			Port:     8080,
			PoolSize: 2,
		},
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return strings.EqualFold(v, "true") || v == "1"
}

func envInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return fallback
	}
	return n
}

func envFloat(key string, fallback float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	var f float64
	if _, err := fmt.Sscanf(v, "%f", &f); err != nil {
		return fallback
	}
	return f
}
