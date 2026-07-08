# Network Acceleration

Modern network protocols and transport optimizations to reduce latency, improve throughput, and enable real-time interactive experiences across the FreeCompute platform.

---

## 1. WebTransport (QUIC-based)

**What**: WebTransport is a W3C/WHATWG standard for browser-based communication over QUIC (HTTP/3). It provides:
- Unidirectional and bidirectional streams
- Datagrams (unreliable, low-latency)
- Native multiplexing over a single QUIC connection
- 0-RTT connection establishment (vs 1-3 RTT for WebSocket)
- No head-of-line blocking (unlike TCP/WebSocket)

**Where**: Replace WebSocket tunnels for real-time media, input, and data channels in modern browsers.

**Architecture**:

```
Browser (WebTransport) ──── QUIC ────▶ Gateway (QUIC listener)
                                            │
                                    ┌───────┴───────┐
                                    ▼               ▼
                              datagrams           streams
                           (game input,       (file transfer,
                            audio, video)      clipboard, large data)
```

**Gateway side**: Add a Go QUIC listener using `github.com/quic-go/quic-go`:

```go
type WebTransportServer struct {
    listener *quic.Listener
    sessions map[string]*quic.Connection
}

func (s *WebTransportServer) AcceptSessions(ctx context.Context) {
    for {
        conn, err := s.listener.Accept(ctx)
        go s.handleSession(conn)
    }
}

func (s *WebTransportServer) handleSession(conn quic.Connection) {
    // Accept unidirectional streams (datagrams)
    // Accept bidirectional streams (reliable)
    // Route to tunnel/agent backend
}
```

**Fallback**: Auto-detect WebTransport support via browser capability check. Fall back to WebSocket + WebRTC data channels when unavailable.

**Implementation priority**: Phase 1 — Datagram tunnel (replaces UDP WebSocket bridge for game input). Phase 2 — Stream tunnel (replaces TCP WebSocket bridge for file transfer). Phase 3 — Replace signaling long-poll with WebTransport streams.

---

## 2. HTTP/3 (QUIC) Gateway Frontend

**What**: Serve all gateway API and control-plane endpoints over HTTP/3 in addition to HTTP/1.1 and HTTP/2.

**Benefit**:
- 0-RTT for repeat connections
- Built-in TLS 1.3
- No TCP head-of-line blocking
- Better performance on lossy networks (mobile, wireless)
- Connection migration across IP changes

**Implementation**:

```go
func (s *Server) listenQUIC(ctx context.Context) error {
    tlsConfig := &tls.Config{
        Certificates: s.tlsCerts,
        NextProtos:   []string{"h3", "h2", "http/1.1"},
    }

    listener, err := quic.ListenAddr(s.quicAddr, tlsConfig, nil)
    if err != nil {
        return err
    }

    for {
        conn, err := listener.Accept(ctx)
        if err != nil { return err }

        go func() {
            // Handle HTTP/3 connection
            http3.ServeConn(conn)
        }()
    }
}
```

**Configuration**: Add `FREECOMPUTE_GATEWAY_QUIC_ADDR` env var (default `:8443`). Require TLS certificate for QUIC (use Let's Encrypt via autocert).

---

## 3. Connection Coalescing (HTTP/2 + QUIC)

**What**: Multiplex multiple logical tunnel connections over a single physical connection to the host agent. Instead of each route getting its own WebSocket/HTTP connection, coalesce all routes into one shared QUIC connection between gateway and agent.

**Current problem**: The host agent opens one tunnel per route via `CONNECT`. With 10 routes, that's 10 TCP+TLS connections. Each consumes a socket, a goroutine pair, and TLS handshake overhead.

**Target**: Single QUIC connection between gateway and host agent. Each route is a separate QUIC stream within the same connection.

```
Before:
┌─────────┐    10× TCP/TLS      ┌────────────┐
│ Gateway │ ◄──────────────────► │ Host Agent │
└─────────┘                      └────────────┘

After:
┌─────────┐    1× QUIC conn     ┌────────────┐
│ Gateway │ ◄──────────────────► │ Host Agent │
└─────────┘    10× streams       └────────────┘
```

**Benefits**:
- Single TLS handshake for all routes
- No head-of-line blocking between routes
- Connection migration without losing routes
- Reduced FD usage on both sides

---

## 4. Adaptive Bitrate with Connection Quality History

**What**: Use per-session connection quality telemetry to proactively set streaming parameters, rather than reacting to congestion.

**System**:
1. Collect per-second stats: RTT, jitter, packet loss, received bitrate
2. Store 60-second moving window per session
3. Predict next 5-second trend using linear regression on historical data
4. Adjust encoder bitrate, resolution, and frame rate preemptively

**Bitrate Adjustment Rules**:

| Condition | Action |
|-----------|--------|
| Loss < 1% and RTT < 30 ms | Increase target 10% (up to max) |
| Loss 1-3% | Hold current |
| Loss 3-5% | Decrease target 15% |
| Loss > 5% | Decrease target 30%, reduce resolution tier |
| RTT > 150 ms | Reduce frame rate cap by 10 FPS |
| Jitter > 40 ms | Increase jitter buffer to 60 ms |
| Throughput drops below encoder output | Force keyframe, reduce bitrate 25% |

**Implementation**:

```go
type AdaptiveBitrate struct {
    history   *ring.Buffer[QualitySample]
    current   BitrateConfig
    minBitrate int
    maxBitrate int
}

func (a *AdaptiveBitrate) OnSample(s QualitySample) {
    a.history.Push(s)
    trend := a.predictTrend()
    a.current = a.computeTarget(trend)
}

func (a *AdaptiveBitrate) predictTrend() Trend {
    // Linear regression over last 10 samples
    // Returns predicted loss, RTT, throughput
}
```

---

## 5. Edge Caching for Proxy Routes

**What**: Cache HTTP proxy responses at the gateway for routes with cacheable content. Dramatically reduces backend load for static assets served through the universal proxy.

**Configuration**: Per-route cache policy in `RouteConfig`:

```json
{
  "id": "web",
  "protocol": "http",
  "target": "http://localhost:3000",
  "cache": {
    "ttl_seconds": 300,
    "max_size_mb": 512,
    "cache_control": "respect-origin"
  }
}
```

**Implementation**: Use an in-memory LRU cache (`hashicorp/golang-lru`) partitioned by route:

```go
type ProxyCache struct {
    mu      sync.RWMutex
    entries map[string]*cacheEntry
    lru     *lru.Cache
    maxSize int64
}

func (c *ProxyCache) ServeCached(w http.ResponseWriter, r *http.Request, route *Route) bool {
    cacheKey := route.ID + ":" + r.URL.String()

    if entry, ok := c.get(cacheKey); ok {
        if time.Since(entry.timestamp) < route.Cache.TTL {
            w.Header().Set("X-Cache", "HIT")
            http.ServeContent(w, r, "", entry.timestamp, bytes.NewReader(entry.data))
            return true
        }
    }
    return false
}
```

**Cache policy**: Respect origin `Cache-Control`, `ETag`, and `Last-Modified` headers. Bust cache on `POST`, `PUT`, `DELETE`. Skip auth-required routes.

---

## 6. Preconnect and Early Hints

**What**: Proactively establish connections to anticipated targets before the client requests them.

**Frontend**: When the user loads the WebOS desktop, preconnect to the gateway:
- `<link rel="preconnect" href="https://api.freecompute.io">`
- `<link rel="dns-prefetch" href="//cdn.freecompute.io">`
- Early Hints (103 Early Hints HTTP status) for session setup resources

**Gateway**: When a WebRTC session is created, start dialling the best host agent immediately while the client completes the signaling handshake:
- `s.agents.take(ctx, bestRoute)` (non-blocking, fast path)
- Begin UDP port allocation for media relay
- Pre-allocate encoder resources

**Host Agent**: On agent tunnel registration, pre-establish 2-3 pooled connections per route (instead of waiting for demand):

```go
// agent_pool.go
func (p *agentPool) warmUp(routeID string, count int) {
    for i := 0; i < count; i++ {
        // Establish pooled connections ahead of demand
        go p.add(context.Background(), routeID, dialRoute(routeID))
    }
}
```

---

## 7. Protocol Negotiation via ALPN

**What**: Use TLS ALPN to negotiate the optimal protocol between client and gateway, and between gateway and host agent, without extra round trips.

**Client → Gateway**: ALPN offers `["h3", "h2", "http/1.1", "webtransport"]`. The gateway selects the best protocol it supports. WebTransport gets priority when available.

**Gateway → Host Agent**: ALPN offers `["fc-quic", "fc-ws"]`. If both sides support QUIC, the gateway and agent establish a single QUIC connection with coalesced streams as described in §3.

**Implementation**: Use Go's `tls.Config.NextProtos` with custom protocol identifiers:

```go
// Gateway to host agent ALPN negotiation
tlsConfig := &tls.Config{
    NextProtos: []string{
        "fc-quic-v1",     // Coalesced QUIC tunnel (preferred)
        "fc-ws-v1",       // WebSocket tunnel (fallback)
    },
}
```

---

## 8. UDP Packet Prioritization (DSCP)

**What**: Mark WebRTC media packets with DSCP (Differentiated Services Code Point) values to prioritize real-time traffic over bulk data.

| Traffic Type | DSCP Class | Binary | Use Case |
|-------------|-----------|--------|----------|
| Audio | EF (46) | `101110` | Opus/AAC audio stream |
| Video | AF41 (34) | `100010` | H.264/VP9 video frames |
| Game Input | CS5 (40) | `101000` | Controller/keyboard/mouse |
| Data | DF (0) | `000000` | File transfer, clipboard |
| FEC/RTX | AF31 (26) | `011010` | Retransmissions |

**Gateway**: Set socket option on UDP listeners and WebRTC ICE sockets:

```go
func setDSCP(conn *net.UDPConn, dscp byte) error {
    raw, err := conn.SyscallConn()
    if err != nil { return err }
    return raw.Control(func(fd uintptr) {
        syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, syscall.IP_TOS, int(dscp)<<2)
    })
}
```

**Note**: Requires root or `CAP_NET_ADMIN` capability. Graceful degradation when unavailable.

---

## Protocol Selection Decision Tree

```
Client connects ──► TLS handshake
                        │
                  ALPN offered:
              h3 / webtransport? ──Yes──► WebTransport (QUIC)
                        │
                  No ──► h2? ──Yes──► HTTP/2 + WebSocket
                        │
                  No ──► WebRTC supported?
                        │     Yes ──► WebRTC (media) + WebSocket (signal)
                        │     No  ──► Long-poll signaling + fallback
```

---

## Measurable Targets

| Metric | Current Baseline | With Acceleration |
|--------|-----------------|-------------------|
| Connection establishment | 2-3 RTT (TCP+TLS+WS) | 0-RTT (QUIC/WebTransport) |
| Tunnel latency (p50) | ~15 ms | ~8 ms (splice + QUIC) |
| Tunnel latency (p95) | ~45 ms | ~20 ms (DSCP + proactive) |
| Startup lag to first frame | ~3 s | ~800 ms (preconnect + 0-RTT) |
| Concurrent sessions per gateway | ~1,000 | ~10,000 (coalescing + bounded) |
| Bitrate adaptation speed | 5-10 s | <1 s (predictive ML) |
