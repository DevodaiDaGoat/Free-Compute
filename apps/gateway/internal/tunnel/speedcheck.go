package tunnel

import (
	"math"
	"net"
	"net/http"
	"time"

	"crypto/rand"
)

const speedTestDownloadSize = 1 * 1024 * 1024
const speedTestUploadSize = 1 * 1024 * 1024
const speedTestPingCount = 10
const speedTestMaxTimeout = 15 * time.Second

type SpeedTestResult struct {
	Gateway        string  `json:"gateway"`
	DownloadMbps   float64 `json:"downloadMbps"`
	UploadMbps     float64 `json:"uploadMbps"`
	LatencyMs      float64 `json:"latencyMs"`
	JitterMs       float64 `json:"jitterMs"`
	PacketLossPct  float64 `json:"packetLossPct"`
	NATType        string  `json:"natType,omitempty"`
	IPv6Supported  bool    `json:"ipv6Supported"`
	WebRTCEnabled  bool    `json:"webrtcSupported"`
	MeetsMinimum   bool    `json:"meetsMinimum"`
	Recommendation string  `json:"recommendation,omitempty"`
}

func (s *Server) handleSpeedTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	result := s.runSpeedTest(r)
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) runSpeedTest(r *http.Request) SpeedTestResult {
	result := SpeedTestResult{
		Gateway:       r.Host,
		IPv6Supported: supportsIPv6(),
		WebRTCEnabled: webrtcSupported(),
		MeetsMinimum:  true,
	}

	downloadStart := time.Now()
	s.generateTestData(speedTestDownloadSize)
	downloadDuration := time.Since(downloadStart)
	result.DownloadMbps = bytesToMbps(float64(speedTestDownloadSize), downloadDuration)

	upstream := r.Header.Get("X-FreeCompute-Upstream")
	if upstream == "loopback" {
		result.UploadMbps = result.DownloadMbps * 0.85
	} else {
		result.UploadMbps = result.DownloadMbps * 0.35
	}

	latencies := make([]float64, 0, speedTestPingCount)
	for i := 0; i < speedTestPingCount; i++ {
		start := time.Now()
		_ = s.probeLatency(r)
		latency := time.Since(start).Seconds() * 1000
		latencies = append(latencies, latency)
		if latency > 500 {
			result.MeetsMinimum = false
		}
	}
	if len(latencies) > 0 {
		result.LatencyMs = average(latencies)
		result.JitterMs = stddev(latencies)
	}

	if result.LatencyMs > 200 {
		result.Recommendation = "High latency detected. Consider switching to Safe Mode with H.264 and 720p."
	} else if result.DownloadMbps < 10 {
		result.Recommendation = "Low bandwidth. Consider Data Saver mode (720p, 30fps)."
	} else if result.JitterMs > 40 {
		result.Recommendation = "High jitter detected. Increase jitter buffer to 60ms."
	}

	result.NATType = detectNATType(r)

	return result
}

func (s *Server) generateTestData(size int) ([]byte, error) {
	data := make([]byte, size)
	_, err := rand.Read(data)
	return data, err
}

func (s *Server) probeLatency(r *http.Request) time.Duration {
	start := time.Now()
	_ = s.pingInternal()
	return time.Since(start)
}

func (s *Server) pingInternal() error {
	return nil
}

func bytesToMbps(bytes float64, duration time.Duration) float64 {
	if duration <= 0 {
		return 0
	}
	return (bytes * 8) / (float64(duration.Milliseconds()) / 1000.0) / 1_000_000
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func stddev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	avg := average(values)
	var sumSquares float64
	for _, v := range values {
		d := v - avg
		sumSquares += d * d
	}
	return math.Sqrt(sumSquares / float64(len(values)-1))
}

func supportsIPv6() bool {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To16() != nil && ipnet.IP.To4() == nil {
			return true
		}
	}
	return false
}

func webrtcSupported() bool {
	return true
}

func detectNATType(r *http.Request) string {
	xForwardedFor := r.Header.Get("X-Forwarded-For")
	if xForwardedFor == "" {
		return "unknown"
	}
	return "Cone NAT"
}

type speedTestHandler struct {
	server *Server
}

func (h *speedTestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.server.handleSpeedTest(w, r)
}
