package tunnel

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultGatewayAddr            = ":8080"
	defaultShutdownSeconds        = 10
	defaultDialSeconds            = 10
	defaultAgentWaitSeconds       = 15
	defaultIdleSeconds            = 120
	defaultHTTPReadHeaderSeconds  = 5
	defaultHTTPIdleSeconds        = 120
	defaultUpstreamIdleSeconds    = 90
	defaultResponseHeaderSeconds  = 10
	defaultExpectContinueSeconds  = 1
	defaultMaxIdleConns           = 8192
	defaultMaxIdleConnsPerHost    = 1024
	defaultProxyFlushMilliseconds = 5
	defaultQualityEWMAAlpha       = 0.3
	defaultQualityGoodRTTMs       = 30.0
	defaultQualityFairRTTMs       = 100.0
	defaultQualityGoodLossRatio   = 0.01
	defaultQualityFairLossRatio   = 0.05
	defaultQualityGoodJitterMs    = 15.0
	defaultQualityFairJitterMs    = 40.0
	defaultQualityMinBandwidthBps = 100_000
	agentTarget                   = "agent"
)

type Protocol string

const (
	ProtocolHTTP      Protocol = "http"
	ProtocolHTTPS     Protocol = "https"
	ProtocolTCP       Protocol = "tcp"
	ProtocolUDP       Protocol = "udp"
	ProtocolWebSocket Protocol = "websocket"
	ProtocolWebRTC    Protocol = "webrtc"
	ProtocolP2P       Protocol = "p2p"
	ProtocolSSH       Protocol = "ssh"
)

type Config struct {
	Addr                  string
	TunnelToken           string
	Routes                []RouteConfig
	ShutdownTimeout       time.Duration
	DialTimeout           time.Duration
	AgentWaitTimeout      time.Duration
	HTTPReadHeaderTimeout time.Duration
	HTTPIdleTimeout       time.Duration
	UpstreamIdleTimeout   time.Duration
	ResponseHeaderTimeout time.Duration
	ExpectContinueTimeout time.Duration
	MaxIdleConns          int
	MaxIdleConnsPerHost   int
	ProxyFlushInterval    time.Duration
	CDNHostname           string
	EdgeHostname          string
	APIHostname           string
	QUICAddr              string
	MeshPeers             []string
	QoSConfig             QoSConfig
	Quality               QualityConfig
	TCPCCAlgo             string
	TCPBufferSize         int
	UDPBufferSize         int
	EnableCompression     bool
	EnableProxyCache      bool
	MaxProxyCacheMB       int
	EnableSessionReplay   bool
	EnableSpeedTest       bool
	RecordingDir          string
	DBPath                string
	ModerationLLMURL      string
	ModerationLLMKey      string
	DefaultBrowsingMode   string
	DNSTTLSeconds         int
	DNSMaxEntries         int
	TCPFastOpen           int
	TCPDeferAccept        int
	HTTPReadBuffer        int
	HTTPWriteBuffer       int
	RateLimitRPM          int
	MaxConnsPerUser       int
}

type RouteConfig struct {
	ID                 string        `json:"id"`
	Protocol           Protocol      `json:"protocol"`
	Listen             string        `json:"listen,omitempty"`
	Target             string        `json:"target,omitempty"`
	IdleTimeoutSeconds int           `json:"idleTimeoutSeconds,omitempty"`
	RequireAuth        *bool         `json:"requireAuth,omitempty"`
	BrowsingMode       string        `json:"browsingMode,omitempty"`
	Cache              *CacheConfig  `json:"cache,omitempty"`
	QoS                *QoSConfig    `json:"qos,omitempty"`
}

type CacheConfig struct {
	TTLSeconds     int    `json:"ttl_seconds,omitempty"`
	MaxSizeMB      int    `json:"max_size_mb,omitempty"`
	CacheControl   string `json:"cache_control,omitempty"`
}

type QoSConfig struct {
	DSCP int `json:"dscp,omitempty"`
}

type QualityConfig struct {
	EWMAAlpha       float64
	GoodRTTMs       float64
	FairRTTMs       float64
	GoodLossRatio   float64
	FairLossRatio   float64
	GoodJitterMs    float64
	FairJitterMs    float64
	MinBandwidthBps uint64
}

func LoadConfigFromEnv() (Config, error) {
	peersRaw := strings.TrimSpace(os.Getenv("FREECOMPUTE_MESH_PEERS"))
	var peers []string
	if peersRaw != "" {
		peers = strings.Split(peersRaw, ",")
	}

	cfg := Config{
		Addr:                  valueOrDefault(os.Getenv("FREECOMPUTE_GATEWAY_ADDR"), defaultGatewayAddr),
		TunnelToken:           os.Getenv("FREECOMPUTE_TUNNEL_TOKEN"),
		ShutdownTimeout:       secondsFromEnv("FREECOMPUTE_GATEWAY_SHUTDOWN_SECONDS", defaultShutdownSeconds),
		DialTimeout:           secondsFromEnv("FREECOMPUTE_TUNNEL_DIAL_SECONDS", defaultDialSeconds),
		AgentWaitTimeout:      secondsFromEnv("FREECOMPUTE_TUNNEL_AGENT_WAIT_SECONDS", defaultAgentWaitSeconds),
		HTTPReadHeaderTimeout: secondsFromEnv("FREECOMPUTE_GATEWAY_READ_HEADER_SECONDS", defaultHTTPReadHeaderSeconds),
		HTTPIdleTimeout:       secondsFromEnv("FREECOMPUTE_GATEWAY_IDLE_SECONDS", defaultHTTPIdleSeconds),
		UpstreamIdleTimeout:   secondsFromEnv("FREECOMPUTE_PROXY_UPSTREAM_IDLE_SECONDS", defaultUpstreamIdleSeconds),
		ResponseHeaderTimeout: secondsFromEnv("FREECOMPUTE_PROXY_RESPONSE_HEADER_SECONDS", defaultResponseHeaderSeconds),
		ExpectContinueTimeout: secondsFromEnv("FREECOMPUTE_PROXY_EXPECT_CONTINUE_SECONDS", defaultExpectContinueSeconds),
		MaxIdleConns:          intFromEnv("FREECOMPUTE_PROXY_MAX_IDLE_CONNS", defaultMaxIdleConns),
		MaxIdleConnsPerHost:   intFromEnv("FREECOMPUTE_PROXY_MAX_IDLE_CONNS_PER_HOST", defaultMaxIdleConnsPerHost),
		ProxyFlushInterval:    millisecondsFromEnv("FREECOMPUTE_PROXY_FLUSH_MS", defaultProxyFlushMilliseconds),
		CDNHostname:           valueOrDefault(os.Getenv("FREECOMPUTE_CDN_HOSTNAME"), ""),
		EdgeHostname:          valueOrDefault(os.Getenv("FREECOMPUTE_EDGE_HOSTNAME"), ""),
		APIHostname:           valueOrDefault(os.Getenv("FREECOMPUTE_API_HOSTNAME"), ""),
		QUICAddr:              valueOrDefault(os.Getenv("FREECOMPUTE_GATEWAY_QUIC_ADDR"), ":8084"),
		MeshPeers:             peers,
		Quality: QualityConfig{
			EWMAAlpha:       float64OrDefault(os.Getenv("FREECOMPUTE_QUALITY_EWMA_ALPHA"), defaultQualityEWMAAlpha),
			GoodRTTMs:       float64OrDefault(os.Getenv("FREECOMPUTE_QUALITY_GOOD_RTT_MS"), defaultQualityGoodRTTMs),
			FairRTTMs:       float64OrDefault(os.Getenv("FREECOMPUTE_QUALITY_FAIR_RTT_MS"), defaultQualityFairRTTMs),
			GoodLossRatio:   float64OrDefault(os.Getenv("FREECOMPUTE_QUALITY_GOOD_LOSS_RATIO"), defaultQualityGoodLossRatio),
			FairLossRatio:   float64OrDefault(os.Getenv("FREECOMPUTE_QUALITY_FAIR_LOSS_RATIO"), defaultQualityFairLossRatio),
			GoodJitterMs:    float64OrDefault(os.Getenv("FREECOMPUTE_QUALITY_GOOD_JITTER_MS"), defaultQualityGoodJitterMs),
			FairJitterMs:    float64OrDefault(os.Getenv("FREECOMPUTE_QUALITY_FAIR_JITTER_MS"), defaultQualityFairJitterMs),
			MinBandwidthBps: uint64(intFromEnv("FREECOMPUTE_QUALITY_MIN_BANDWIDTH_BPS", defaultQualityMinBandwidthBps)),
		},
		TCPCCAlgo:             valueOrDefault(os.Getenv("FREECOMPUTE_TCP_CC_ALGO"), "auto"),
		TCPBufferSize:         intFromEnv("FREECOMPUTE_TCP_BUFFER_SIZE", 8_388_608),
		UDPBufferSize:         intFromEnv("FREECOMPUTE_UDP_BUFFER_SIZE", 16_777_216),
		EnableCompression:     LoadCompressionConfig(),
		EnableProxyCache:      strings.TrimSpace(os.Getenv("FREECOMPUTE_DISABLE_PROXY_CACHE")) == "",
		MaxProxyCacheMB:       intFromEnv("FREECOMPUTE_PROXY_CACHE_MAX_SIZE_MB", 256),
		EnableSessionReplay:   strings.TrimSpace(os.Getenv("FREECOMPUTE_ENABLE_SESSION_REPLAY")) != "false",
		EnableSpeedTest:       strings.TrimSpace(os.Getenv("FREECOMPUTE_ENABLE_SPEED_TEST")) != "false",
		RecordingDir:          valueOrDefault(os.Getenv("FREECOMPUTE_RECORDING_DIR"), defaultStatePath("freecompute-recordings", true)),
		DBPath:                valueOrDefault(os.Getenv("FREECOMPUTE_DB_PATH"), defaultStatePath("freecompute.db", false)),
		ModerationLLMURL:       valueOrDefault(os.Getenv("FREECOMPUTE_MODERATION_LLM_URL"), ""),
		ModerationLLMKey:       valueOrDefault(os.Getenv("FREECOMPUTE_MODERATION_LLM_KEY"), ""),
		DefaultBrowsingMode:    valueOrDefault(strings.ToLower(os.Getenv("FREECOMPUTE_DEFAULT_BROWSING_MODE")), "casual"),
		DNSTTLSeconds:          intFromEnv("FREECOMPUTE_DNS_TTL_SECONDS", 600),
		DNSMaxEntries:          intFromEnv("FREECOMPUTE_DNS_MAX_ENTRIES", 4096),
		TCPFastOpen:            intFromEnv("FREECOMPUTE_TCP_FASTOPEN", 5),
		TCPDeferAccept:         intFromEnv("FREECOMPUTE_TCP_DEFER_ACCEPT", 5),
		HTTPReadBuffer:         intFromEnv("FREECOMPUTE_HTTP_READ_BUFFER", 0),
		HTTPWriteBuffer:        intFromEnv("FREECOMPUTE_HTTP_WRITE_BUFFER", 0),
		RateLimitRPM:           intFromEnv("FREECOMPUTE_RATE_LIMIT_RPM", 2000),
		MaxConnsPerUser:        intFromEnv("FREECOMPUTE_MAX_CONNS_PER_USER", 200),
	}

	// Prefer routes file over env var
	rawRoutes := strings.TrimSpace(os.Getenv("FREECOMPUTE_TUNNEL_ROUTES"))
	rawRoutesFile := strings.TrimSpace(os.Getenv("FREECOMPUTE_TUNNEL_ROUTES_FILE"))
	if rawRoutesFile != "" {
		data, err := os.ReadFile(rawRoutesFile)
		if err == nil {
			rawRoutes = string(data)
		}
	}
	if rawRoutes == "" {
		return cfg, nil
	}

	if err := json.Unmarshal([]byte(rawRoutes), &cfg.Routes); err != nil {
		return cfg, fmt.Errorf("parse FREECOMPUTE_TUNNEL_ROUTES: %w", err)
	}

	for i := range cfg.Routes {
		if err := cfg.Routes[i].Validate(); err != nil {
			return cfg, err
		}
	}

	return cfg, nil
}

func LoadCompressionConfig() bool {
	return strings.TrimSpace(os.Getenv("FREECOMPUTE_ENABLE_COMPRESSION")) != "false"
}

func LoadProxyCacheConfig() (bool, int) {
	if strings.TrimSpace(os.Getenv("FREECOMPUTE_DISABLE_PROXY_CACHE")) != "" {
		return false, 0
	}
	return true, intFromEnv("FREECOMPUTE_PROXY_CACHE_MAX_SIZE_MB", 256)
}

func (r RouteConfig) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return fmt.Errorf("route id is required")
	}

	switch r.Protocol {
	case ProtocolHTTP, ProtocolHTTPS, ProtocolWebSocket:
		if _, err := parseProxyTarget(r.Target); err != nil {
			return fmt.Errorf("route %q target must be a URL: %w", r.ID, err)
		}
	case ProtocolTCP, ProtocolSSH:
		if r.Target == agentTarget {
			break
		}
		if _, _, err := net.SplitHostPort(r.Target); err != nil {
			return fmt.Errorf("route %q target must be host:port or %q: %w", r.ID, agentTarget, err)
		}
	case ProtocolUDP:
		if _, _, err := net.SplitHostPort(r.Target); err != nil {
			return fmt.Errorf("route %q target must be host:port: %w", r.ID, err)
		}
	case ProtocolWebRTC, ProtocolP2P:
		return nil
	default:
		return fmt.Errorf("route %q has unsupported protocol %q", r.ID, r.Protocol)
	}

	if r.Listen != "" {
		if _, _, err := net.SplitHostPort(r.Listen); err != nil {
			return fmt.Errorf("route %q listen must be host:port: %w", r.ID, err)
		}
	}

	return nil
}

func (r RouteConfig) IdleTimeout() time.Duration {
	if r.IdleTimeoutSeconds <= 0 {
		return time.Duration(defaultIdleSeconds) * time.Second
	}

	return time.Duration(r.IdleTimeoutSeconds) * time.Second
}

func (r RouteConfig) AuthRequired(globalToken string) bool {
	if r.RequireAuth != nil {
		return *r.RequireAuth
	}

	return globalToken != ""
}

func (r RouteConfig) UsesAgentTunnel() bool {
	return (r.Protocol == ProtocolTCP || r.Protocol == ProtocolSSH) && r.Target == agentTarget
}

func parseProxyTarget(rawTarget string) (*url.URL, error) {
	target, err := url.Parse(rawTarget)
	if err != nil {
		return nil, err
	}
	if target.Scheme == "" || target.Host == "" {
		return nil, fmt.Errorf("target must include scheme and host")
	}

	switch target.Scheme {
	case "http", "https":
	case "ws":
		target.Scheme = "http"
	case "wss":
		target.Scheme = "https"
	default:
		return nil, fmt.Errorf("unsupported target scheme %q", target.Scheme)
	}

	return target, nil
}

func valueOrDefault(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}

	return value
}

// defaultStatePath resolves a stateful file/directory path. When
// FREECOMPUTE_STATE_DIR is set, everything lives under that dir so restarts
// with a fresh state dir don't inherit stale DB rows from $TEMP (which caused
// admin-seed collisions when the .admin-password sidecar was missing). Falls
// back to $TEMP for backwards compatibility. `isDir` doesn't change the path,
// it's kept for future callers that may need to distinguish.
func defaultStatePath(name string, isDir bool) string {
	_ = isDir
	if dir := strings.TrimSpace(os.Getenv("FREECOMPUTE_STATE_DIR")); dir != "" {
		return filepath.Join(dir, name)
	}
	return filepath.Join(os.TempDir(), name)
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

func float64OrDefault(raw string, fallback float64) float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func millisecondsFromEnv(name string, fallback int) time.Duration {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return time.Duration(fallback) * time.Millisecond
	}

	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return time.Duration(fallback) * time.Millisecond
	}

	return time.Duration(parsed) * time.Millisecond
}

func intFromEnv(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return fallback
	}

	return parsed
}
