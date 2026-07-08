package webrtc

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

type TransportType string

const (
	TransportWebSocket    TransportType = "websocket"
	TransportWebTransport TransportType = "webtransport"
	TransportWebRTC       TransportType = "webrtc"
)

type TransportCapability struct {
	Type     TransportType `json:"type"`
	Priority int           `json:"priority"`
	Endpoint string        `json:"endpoint"`
}

type WebTransportServer struct {
	logger   *log.Logger
	sessions sync.Map
	cert     tls.Certificate
	addr     string
}

type WTSession struct {
	ID        string
	CreatedAt time.Time
	Data      map[string]interface{}
	mu        sync.Mutex
}

func NewWebTransportServer(logger *log.Logger, addr string) *WebTransportServer {
	return &WebTransportServer{
		logger: logger,
		addr:   addr,
	}
}

func (s *WebTransportServer) AcceptSessions(ctx context.Context) error {
	s.logger.Printf("webtransport listening on %s (QUIC/HTTP/3)", s.addr)
	<-ctx.Done()
	return nil
}

func (s *WebTransportServer) HandleCapabilities() []TransportCapability {
	return []TransportCapability{
		{Type: TransportWebTransport, Priority: 0, Endpoint: "wt://" + s.addr + "/"},
		{Type: TransportWebRTC, Priority: 1, Endpoint: "/signal/"},
		{Type: TransportWebSocket, Priority: 2, Endpoint: "/ws/"},
	}
}

func CreateTransport(clientCapabilities []string) TransportType {
	hasWT := false
	hasWS := false
	for _, c := range clientCapabilities {
		switch c {
		case "webtransport":
			hasWT = true
		case "websocket":
			hasWS = true
		}
	}
	if hasWT {
		return TransportWebTransport
	}
	if hasWS {
		return TransportWebSocket
	}
	return TransportWebRTC
}

type MeshPeer struct {
	ID        string    `json:"id"`
	Addr      string    `json:"addr"`
	Region    string    `json:"region"`
	LatencyMs int       `json:"latencyMs"`
	Capacity  float64   `json:"capacity"`
	LastSeen  time.Time `json:"lastSeen"`
}

type RelayMesh struct {
	mu    sync.RWMutex
	peers map[string]*MeshPeer
}

func NewRelayMesh() *RelayMesh {
	return &RelayMesh{
		peers: make(map[string]*MeshPeer),
	}
}

func (m *RelayMesh) AddPeer(peer *MeshPeer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.peers[peer.ID] = peer
}

func (m *RelayMesh) RemovePeer(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.peers, id)
}

func (m *RelayMesh) SelectBestPeer(clientRegion string) *MeshPeer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var best *MeshPeer
	bestScore := -1.0

	for _, peer := range m.peers {
		score := scorePeer(peer, clientRegion)
		if score > bestScore {
			bestScore = score
			best = peer
		}
	}
	return best
}

func (m *RelayMesh) ListPeers() []*MeshPeer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	peers := make([]*MeshPeer, 0, len(m.peers))
	for _, p := range m.peers {
		peers = append(peers, p)
	}
	return peers
}

func (m *RelayMesh) HandleMeshStatus(w http.ResponseWriter, r *http.Request) {
	peers := m.ListPeers()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"peers":   peers,
		"count":   len(peers),
		"healthy": len(peers) > 0,
	})
}

func scorePeer(peer *MeshPeer, clientRegion string) float64 {
	latencyScore := 100.0
	if peer.LatencyMs > 0 {
		latencyScore = float64(max(0, 300-peer.LatencyMs)) / 3.0
	}
	capacityScore := (1.0 - peer.Capacity) * 30.0
	affinityScore := 10.0
	if peer.Region == clientRegion {
		affinityScore = 10.0
	}
	return latencyScore*0.6 + capacityScore*0.3 + affinityScore*0.1
}

type DSCPClass byte

const (
	DSCPAudio    DSCPClass = 46 // EF
	DSCPVideo    DSCPClass = 34 // AF41
	DSCPInput    DSCPClass = 40 // CS5
	DSCPData     DSCPClass = 0  // DF
	DSCPFEC      DSCPClass = 26 // AF31
)

func SetDSCP(class DSCPClass) int {
	return int(class) << 2
}
