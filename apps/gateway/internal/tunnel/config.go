package tunnel

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
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
	defaultMaxIdleConns           = 1024
	defaultMaxIdleConnsPerHost    = 128
	defaultProxyFlushMilliseconds = -1
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
}

type RouteConfig struct {
	ID                 string   `json:"id"`
	Protocol           Protocol `json:"protocol"`
	Listen             string   `json:"listen,omitempty"`
	Target             string   `json:"target,omitempty"`
	IdleTimeoutSeconds int      `json:"idleTimeoutSeconds,omitempty"`
	RequireAuth        *bool    `json:"requireAuth,omitempty"`
}

func LoadConfigFromEnv() (Config, error) {
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
	}

	rawRoutes := strings.TrimSpace(os.Getenv("FREECOMPUTE_TUNNEL_ROUTES"))
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
