package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	defaultPoolSize         = 2
	defaultDialSeconds      = 10
	defaultReconnectSeconds = 1
)

type Config struct {
	GatewayURL      string
	Token           string
	Routes          []RouteConfig
	DialTimeout     time.Duration
	ReconnectDelay  time.Duration
	InsecureSkipTLS bool
}

type RouteConfig struct {
	ID       string `json:"id"`
	Target   string `json:"target"`
	PoolSize int    `json:"poolSize,omitempty"`
}

type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

type VMInstance struct {
	ID         string
	Name       string
	State      string
	PID        int
	MonitorFD  int
	SocketPath string
	CPUCores   int
	RAMMB      int
	DiskGB     int
	GPUEnabled bool
}

type HostCapabilities struct {
	GPUModel       string   `json:"gpuModel"`
	GPUVendor      string   `json:"gpuVendor"`
	VRAMGB         float64  `json:"vramGb"`
	EncoderSupport []string `json:"encoderSupport"`
	CPUCores       int      `json:"cpuCores"`
	RAMGB          int      `json:"ramGb"`
	DiskGB         int      `json:"diskGb"`
	NetworkMbps    int      `json:"networkMbps"`
	Region         string   `json:"region"`
}

type HostStatus struct {
	VMs              []VMInstance     `json:"vms"`
	Capabilities     HostCapabilities `json:"capabilities"`
	CPUUsagePercent  float64          `json:"cpuUsagePercent"`
	RAMUsagePercent  float64          `json:"ramUsagePercent"`
	GPUUsagePercent  float64          `json:"gpuUsagePercent"`
	ActiveStreams    int              `json:"activeStreams"`
	ActiveTunnels    int              `json:"activeTunnels"`
}

func main() {
	_ = runtime.GOMAXPROCS(runtime.NumCPU())
	logger := log.New(os.Stdout, "host-agent ", log.LstdFlags|log.LUTC|log.Lmicroseconds)

	cfg, err := loadConfig()
	if err != nil {
		logger.Fatalf("config error: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	vmManager := NewVMManager(logger)
	caps := detectCapabilities(logger)
	logger.Printf("detected capabilities: GPU=%s VRAM=%.1fGB encoders=%v", caps.GPUModel, caps.VRAMGB, caps.EncoderSupport)

	tailman := NewTailscaleManager(logger, cfg.GatewayURL, cfg.Token)
	if err := tailman.Discover(); err != nil {
		logger.Printf("tailscale init: %v", err)
	}
	if tailman.HostIP() != "" {
		if err := tailman.RegisterWithGateway(); err != nil {
			logger.Printf("tailscale register: %v", err)
		}
	}

	var wg sync.WaitGroup
	for _, route := range cfg.Routes {
		route := route
		for i := 0; i < route.poolSize(); i++ {
			wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			time.Sleep(time.Duration(slot) * 200 * time.Millisecond)
			runTunnelLoop(ctx, cfg, route, slot, logger, vmManager)
		}(i)
		}
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		runStatusReporter(ctx, cfg, logger, vmManager, caps)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		runTailscaleRegisterLoop(ctx, tailman, logger)
	}()

	wg.Wait()
}

func loadConfig() (Config, error) {
	cfg := Config{
		GatewayURL:      strings.TrimRight(os.Getenv("FREECOMPUTE_AGENT_GATEWAY_URL"), "/"),
		Token:           os.Getenv("FREECOMPUTE_AGENT_TOKEN"),
		DialTimeout:     secondsFromEnv("FREECOMPUTE_AGENT_DIAL_SECONDS", defaultDialSeconds),
		ReconnectDelay:  secondsFromEnv("FREECOMPUTE_AGENT_RECONNECT_SECONDS", defaultReconnectSeconds),
		InsecureSkipTLS: os.Getenv("FREECOMPUTE_AGENT_INSECURE_SKIP_TLS") == "1",
	}
	if cfg.GatewayURL == "" {
		return cfg, errors.New("FREECOMPUTE_AGENT_GATEWAY_URL is required")
	}

	rawRoutes := strings.TrimSpace(os.Getenv("FREECOMPUTE_AGENT_ROUTES"))
	if rawRoutes == "" {
		return cfg, errors.New("FREECOMPUTE_AGENT_ROUTES is required")
	}
	if err := json.Unmarshal([]byte(rawRoutes), &cfg.Routes); err != nil {
		return cfg, fmt.Errorf("parse FREECOMPUTE_AGENT_ROUTES: %w", err)
	}
	for _, route := range cfg.Routes {
		if strings.TrimSpace(route.ID) == "" {
			return cfg, errors.New("agent route id is required")
		}
		if _, _, err := net.SplitHostPort(route.Target); err != nil {
			return cfg, fmt.Errorf("route %q target must be host:port: %w", route.ID, err)
		}
	}

	return cfg, nil
}

func (r RouteConfig) poolSize() int {
	if r.PoolSize <= 0 {
		return defaultPoolSize
	}
	return r.PoolSize
}

func runTunnelLoop(ctx context.Context, cfg Config, route RouteConfig, slot int, logger *log.Logger, vm *VMManager) {
	baseDelay := cfg.ReconnectDelay
	maxDelay := 60 * time.Second
	attempt := 0
	consecutiveFails := 0
	lastLoggedFail := time.Time{}
	const suppressAfter = 3 // log freely for first 3 failures, then throttle

	for {
		err := runTunnelOnce(ctx, cfg, route, logger, vm)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			consecutiveFails++
			shouldLog := consecutiveFails <= suppressAfter ||
				time.Since(lastLoggedFail) >= 60*time.Second
			if shouldLog {
				if consecutiveFails == suppressAfter+1 {
					logger.Printf("route=%s slot=%d: target unavailable, suppressing further logs (retrying silently)", route.ID, slot)
				} else {
					logger.Printf("route=%s slot=%d disconnected: %v", route.ID, slot, err)
				}
				lastLoggedFail = time.Now()
			}
		} else {
			if consecutiveFails > suppressAfter {
				logger.Printf("route=%s slot=%d: reconnected after %d failures", route.ID, slot, consecutiveFails)
			}
			consecutiveFails = 0
			attempt = 0 // reset backoff on successful connection
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(computeBackoff(baseDelay, maxDelay, attempt)):
			if attempt < 30 {
				attempt++
			}
		}
	}
}

func computeBackoff(base, max time.Duration, attempt int) time.Duration {
	d := base << uint(attempt)
	if d > max {
		d = max
	}
	jitter := time.Duration(float64(d) * (0.75 + 0.5*rand.Float64()))
	return jitter
}



func runTunnelOnce(ctx context.Context, cfg Config, route RouteConfig, logger *log.Logger, vm *VMManager) error {
	localConn, err := dialLocalTarget(ctx, cfg, route)
	if err != nil {
		return err
	}
	defer localConn.Close()

	gatewayConn, buffered, err := connectToGateway(ctx, cfg, route)
	if err != nil {
		return err
	}
	defer gatewayConn.Close()

	logger.Printf("route=%s connected gateway to local target=%s", route.ID, route.Target)
	bridge(ctx, &bufferedConn{Conn: gatewayConn, reader: buffered}, localConn)
	return nil
}

func connectToGateway(ctx context.Context, cfg Config, route RouteConfig) (net.Conn, *bufio.Reader, error) {
	gatewayURL, err := url.Parse(cfg.GatewayURL)
	if err != nil {
		return nil, nil, err
	}
	if gatewayURL.Scheme != "http" && gatewayURL.Scheme != "https" {
		return nil, nil, fmt.Errorf("unsupported gateway scheme %q", gatewayURL.Scheme)
	}

	dialer := &net.Dialer{Timeout: cfg.DialTimeout, KeepAlive: 5 * time.Second}
	addr := hostWithDefaultPort(gatewayURL)

	var conn net.Conn
	if gatewayURL.Scheme == "https" {
		tlsDialer := tls.Dialer{NetDialer: dialer, Config: &tls.Config{
			ServerName:         gatewayURL.Hostname(),
			InsecureSkipVerify: cfg.InsecureSkipTLS,
		}}
		conn, err = tlsDialer.DialContext(ctx, "tcp", addr)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return nil, nil, err
	}

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
		_ = setTCPKeepaliveAggressive(tcpConn)
	}

	pathPrefix := strings.TrimRight(gatewayURL.EscapedPath(), "/")
	path := pathPrefix + "/agent/" + url.PathEscape(route.ID)
	requestLines := []string{
		fmt.Sprintf("CONNECT %s HTTP/1.1", path),
		"Host: " + gatewayURL.Host,
		"User-Agent: freecompute-host-agent",
	}
	if cfg.Token != "" {
		requestLines = append(requestLines, "Authorization: Bearer "+cfg.Token)
	}
	request := strings.Join(requestLines, "\r\n") + "\r\n\r\n"

	if _, err := conn.Write([]byte(request)); err != nil {
		_ = conn.Close()
		return nil, nil, err
	}

	reader := bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	if err != nil {
		_ = conn.Close()
		return nil, nil, err
	}

	if response.StatusCode != http.StatusOK {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("gateway rejected tunnel route=%s status=%s", route.ID, response.Status)
	}

	return conn, reader, nil
}

func dialLocalTarget(ctx context.Context, cfg Config, route RouteConfig) (net.Conn, error) {
	dialer := net.Dialer{Timeout: cfg.DialTimeout, KeepAlive: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", route.Target)
	if err != nil {
		return nil, err
	}
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
		_ = setTCPKeepaliveAggressive(tcpConn)
	}
	return conn, nil
}

func bridge(ctx context.Context, left net.Conn, right net.Conn) {
	errCh := make(chan error, 2)
	go copyConn(errCh, left, right)
	go copyConn(errCh, right, left)
	select {
	case <-ctx.Done():
	case <-errCh:
	}
	// Drain second error to prevent goroutine leak
	select {
	case <-errCh:
	default:
	}
}

var copyBufferPool = sync.Pool{
	New: func() any {
		b := make([]byte, 256*1024)
		return &b
	},
}

func getCopyBuf() *[]byte {
	return copyBufferPool.Get().(*[]byte)
}

func putCopyBuf(buf *[]byte) {
	copyBufferPool.Put(buf)
}

func copyConn(errCh chan<- error, dst io.Writer, src io.Reader) {
	buf := getCopyBuf()
	defer putCopyBuf(buf)
	_, err := io.CopyBuffer(dst, src, *buf)
	if closeWriter, ok := dst.(interface{ CloseWrite() error }); ok {
		_ = closeWriter.CloseWrite()
	}
	errCh <- err
}

func (c *bufferedConn) Read(p []byte) (int, error) {
	if c.reader != nil && c.reader.Buffered() > 0 {
		return c.reader.Read(p)
	}
	return c.Conn.Read(p)
}

func hostWithDefaultPort(gatewayURL *url.URL) string {
	if gatewayURL.Port() != "" {
		return gatewayURL.Host
	}
	switch gatewayURL.Scheme {
	case "https":
		return net.JoinHostPort(gatewayURL.Hostname(), "443")
	default:
		return net.JoinHostPort(gatewayURL.Hostname(), "80")
	}
}

func secondsFromEnv(name string, fallback int) time.Duration {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return time.Duration(fallback) * time.Second
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return time.Duration(fallback) * time.Second
	}
	return time.Duration(parsed) * time.Second
}

func detectCapabilities(logger *log.Logger) HostCapabilities {
	caps := HostCapabilities{
		GPUVendor:      "none",
		EncoderSupport: []string{},
	}

	caps.CPUCores = detectCPUCores()
	caps.RAMGB = detectRAMGB()
	caps.DiskGB = detectDiskGB()
	caps.NetworkMbps = 1000

	gpuModel, gpuVendor, vramGB := detectGPU()
	caps.GPUModel = gpuModel
	caps.GPUVendor = gpuVendor
	caps.VRAMGB = vramGB

	if gpuVendor == "nvidia" || gpuVendor == "amd" {
		caps.EncoderSupport = append(caps.EncoderSupport, "h264", "h265", "h263", "av1")
		logger.Printf("GPU detected: %s %s (%.1f GB VRAM) - hardware encoding available (H.264/H.265/H.263/AV1)", gpuVendor, gpuModel, vramGB)
	} else {
		caps.EncoderSupport = append(caps.EncoderSupport, "h264", "h263")
		logger.Printf("No GPU detected - software encoding only (H.264/H.263)")
	}

	return caps
}

func detectCPUCores() int {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "nproc")
	out, err := cmd.Output()
	if err != nil {
		return 4
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 4
	}
	return n
}

func detectRAMGB() int {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "free", "-g")
	out, err := cmd.Output()
	if err != nil {
		return 8
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Mem:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				n, err := strconv.Atoi(fields[1])
				if err == nil {
					return n
				}
			}
		}
	}
	return 8
}

func detectDiskGB() int {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "df", "-BG", "/")
	out, err := cmd.Output()
	if err != nil {
		return 100
	}
	lines := strings.Split(string(out), "\n")
	if len(lines) >= 2 {
		fields := strings.Fields(lines[1])
		for _, f := range fields {
			f = strings.TrimSuffix(f, "G")
			if n, err := strconv.Atoi(f); err == nil && n > 0 {
				return n
			}
		}
	}
	return 100
}

func detectGPU() (model, vendor string, vramGB float64) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "lspci", "-v")
	out, err := cmd.Output()
	if err != nil {
		return "", "none", 0
	}
	s := string(out)
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, "VGA") || strings.Contains(line, "3D") || strings.Contains(line, "Display") {
			lower := strings.ToLower(line)
			if strings.Contains(lower, "nvidia") {
				vendor = "nvidia"
				parts := strings.Fields(line)
				for i, p := range parts {
					if strings.Contains(strings.ToLower(p), "nvidia") && i+1 < len(parts) {
						model = strings.Join(parts[i:], " ")
						break
					}
				}
				if model == "" {
					model = "NVIDIA GPU"
				}
				vramGB = detectNvidiaVRAM()
			} else if strings.Contains(lower, "amd") || strings.Contains(lower, "radeon") {
				vendor = "amd"
				model = "AMD GPU"
				vramGB = 8
			} else if strings.Contains(lower, "intel") {
				vendor = "intel"
				model = "Intel GPU"
				vramGB = 0
			}
			break
		}
	}
	return model, vendor, vramGB
}

func detectNvidiaVRAM() float64 {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=memory.total", "--format=csv,noheader,nounits")
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	mb, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0
	}
	return float64(mb) / 1024.0
}

func runTailscaleRegisterLoop(ctx context.Context, tailman *TailscaleManager, logger *log.Logger) {
	if tailman.HostIP() == "" {
		return
	}
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := tailman.RegisterWithGateway(); err != nil {
				logger.Printf("tailscale re-register: %v", err)
			}
		}
	}
}

func runStatusReporter(ctx context.Context, cfg Config, logger *log.Logger, vm *VMManager, caps HostCapabilities) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	client := &http.Client{Timeout: 10 * time.Second}
	metricsURL := cfg.GatewayURL + "/hosts/metrics"

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			status := HostStatus{
				VMs:          vm.ListVMs(),
				Capabilities: caps,
			}
			statusBytes, err := json.Marshal(status)
			if err != nil {
				logger.Printf("status marshal error: %v", err)
				continue
			}
			resp, err := client.Post(metricsURL, "application/json", strings.NewReader(string(statusBytes)))
			if err != nil {
				logger.Printf("status report error: %v", err)
				continue
			}
			resp.Body.Close()
			logger.Printf("host status: cpus=%d ram=%dgb gpu=%s vram=%.1fgb encoders=%v", caps.CPUCores, caps.RAMGB, caps.GPUModel, caps.VRAMGB, caps.EncoderSupport)
		}
	}
}
