package tunnel

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCapabilitiesAdvertiseUniversalProxySurface(t *testing.T) {
	server, err := NewServer(Config{}, nil)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/capabilities", nil)
	server.handleCapabilities(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var body struct {
		Gateway    string                     `json:"gateway"`
		Protocols  []Protocol                 `json:"protocols"`
		ClientPath map[string]map[string]any  `json:"clientPaths"`
		BunnyCDN   map[string]json.RawMessage `json:"bunnyCdn"`
		RouteModes []string                   `json:"routeModes"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode capabilities: %v", err)
	}

	if body.Gateway != "freecompute-universal-proxy" {
		t.Fatalf("gateway = %q", body.Gateway)
	}
	for _, protocol := range []Protocol{ProtocolHTTP, ProtocolHTTPS, ProtocolTCP, ProtocolUDP, ProtocolSSH, ProtocolWebRTC, ProtocolP2P} {
		if !containsProtocol(body.Protocols, protocol) {
			t.Fatalf("protocol %q missing from capabilities %v", protocol, body.Protocols)
		}
	}
	if body.ClientPath["browser"]["websocketTunnelPath"] == "" {
		t.Fatalf("browser websocket tunnel path missing")
	}
	if body.ClientPath["webos-app"]["rawUdpListener"] != true {
		t.Fatalf("webos-app raw udp listener capability missing")
	}
	if _, ok := body.BunnyCDN["bypassCache"]; !ok {
		t.Fatalf("bunnycdn bypass cache policy missing")
	}
}

func TestRoutesRequireBearerTokenWhenConfigured(t *testing.T) {
	server, err := NewServer(Config{
		TunnelToken: "secret",
		Routes: []RouteConfig{{
			ID:       "web",
			Protocol: ProtocolHTTP,
			Target:   "http://127.0.0.1:3000",
		}},
	}, nil)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	unauthorized := httptest.NewRecorder()
	server.handleRoutes(unauthorized, httptest.NewRequest(http.MethodGet, "/routes", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want %d", unauthorized.Code, http.StatusUnauthorized)
	}

	authorized := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/routes", nil)
	request.Header.Set("Authorization", "Bearer secret")
	server.handleRoutes(authorized, request)
	if authorized.Code != http.StatusOK {
		t.Fatalf("authorized status = %d, want %d", authorized.Code, http.StatusOK)
	}
}

func TestWebSocketTargetsNormalizeToHTTPTransportSchemes(t *testing.T) {
	registry, err := NewRegistry([]RouteConfig{{
		ID:       "game-control",
		Protocol: ProtocolWebSocket,
		Target:   "wss://example.test/control",
	}}, RegistryOptions{})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	route, ok := registry.Get("game-control")
	if !ok {
		t.Fatalf("route missing")
	}
	if route.targetURL.Scheme != "https" {
		t.Fatalf("normalized target scheme = %q, want https", route.targetURL.Scheme)
	}
}

func TestAgentTunnelWaitIsBounded(t *testing.T) {
	server, err := NewServer(Config{
		AgentWaitTimeout: 10 * time.Millisecond,
		Routes: []RouteConfig{{
			ID:       "vm-ssh",
			Protocol: ProtocolSSH,
			Target:   agentTarget,
		}},
	}, nil)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	route, ok := server.registry.Get("vm-ssh")
	if !ok {
		t.Fatalf("route missing")
	}

	startedAt := time.Now()
	conn, cleanup, err := server.openTCP(context.Background(), route)
	if conn != nil {
		_ = conn.Close()
	}
	if cleanup != nil {
		cleanup()
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("openTCP() error = %v, want context deadline exceeded", err)
	}
	if elapsed := time.Since(startedAt); elapsed > time.Second {
		t.Fatalf("agent wait took %s, want bounded wait", elapsed)
	}
}

func containsProtocol(protocols []Protocol, expected Protocol) bool {
	for _, protocol := range protocols {
		if protocol == expected {
			return true
		}
	}

	return false
}
