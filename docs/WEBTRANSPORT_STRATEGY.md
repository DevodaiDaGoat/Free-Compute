# WebTransport / QUIC Strategy

Migration plan for adding HTTP/3 WebTransport as a low-latency transport alongside WebRTC and WebSocket.

---

## Rationale

| Metric | WebSocket (TCP) | WebRTC DataChannel (SCTP) | WebTransport (QUIC) |
|--------|----------------|---------------------------|---------------------|
| HOL blocking | Yes (TCP) | Partial (SCTP streams) | No (QUIC streams + datagrams) |
| 0-RTT reconnect | No | No | Yes |
| Connection migration | No | ICE restart needed | Native (connection ID) |
| Browser support | Universal | Universal | Chrome 97+, Firefox 114+ |
| Unordered delivery | No | No (SCTP) | Yes (datagrams) |
| Per-packet overhead | ~14 bytes | ~28 bytes | ~8 bytes (datagram) |

**Target workloads for WebTransport:**

- Gamepad/keyboard/mouse input datagrams (< 100 bytes, latency-sensitive, loss-tolerant)
- Session telemetry (periodic small packets, loss-tolerant)
- File transfer streams (reliable, ordered, isolated from input)

---

## Phase 1: Gateway QUIC Listener

### QUIC Endpoint

```go
import "github.com/quic-go/quic-go"

type WebTransportServer struct {
    listener   *quic.Listener
    sessions   sync.Map  // sessionID → *quic.Connection
    cert       tls.Certificate
}
```

Port allocation:

| Port | Transport | Purpose |
|------|-----------|---------|
| 8080 | HTTP/1.1 + HTTP/2 | REST API, WebSocket upgrade proxy routes |
| 8084 | HTTP/3 QUIC | WebTransport datagrams + streams |

### TLS Config

```go
tlsConfig := &tls.Config{
    Certificates: []tls.Certificate{cert},
    NextProtos:   []string{"h3", "freecompute/1"},
}
```

---

## Phase 2: WebTransport Session Multiplexer

### Stream Types

```
WebTransport Session (QUIC Connection)
    │
    ├── Unidirectional Stream (Datagram) — client → server
    │   ├── Stream ID 0x00: Gamepad state (binary, 64 bytes)
    │   ├── Stream ID 0x01: Mouse move (binary, 16 bytes)
    │   ├── Stream ID 0x02: Keyboard event (binary, 12 bytes)
    │   └── Stream ID 0x03: Telemetry report (JSON, ~200 bytes)
    │
    ├── Unidirectional Stream (Datagram) — server → client
    │   ├── Stream ID 0x10: Cursor position (binary, 24 bytes)
    │   ├── Stream ID 0x11: Notification (JSON, ~100 bytes)
    │   └── Stream ID 0x12: Audio metadata (binary, 8 bytes)
    │
    ├── Bidirectional Stream (Reliable) — file transfer
    │   ├── Client opens stream
    │   ├── Sends file header (name, size, MIME type)
    │   └── Sends file chunks (256 KB each, streamed)
    │
    └── Bidirectional Stream (Reliable) — clipboard sync
        ├── Client or server opens stream
        ├── Sends clipboard data (serialized)
        └── Stream closes on sync complete
```

### Datagram vs Stream Decision

| Criterion | Use Datagram | Use Stream |
|-----------|-------------|------------|
| Payload < 1200 bytes | ✅ | — |
| Loss-tolerant | ✅ | — |
| Needs ordering | — | ✅ |
| Needs reliability | — | ✅ |
| Burst transfer > 10 KB | — | ✅ |

---

## Phase 3: Frontend WebTransport Client

### Progressive Enhancement

```typescript
function createTransport(url: string): Transport {
    if (typeof WebTransport !== 'undefined') {
        return new WebTransportClient(url);
    }
    return new WebSocketFallbackClient(url);
}
```

### WebTransportClient API

```typescript
class WebTransportClient implements Transport {
    private transport: WebTransport;

    async connect(url: string): Promise<void> {
        this.transport = new WebTransport(url);
        await this.transport.ready;
    }

    // Unreliable, unordered — for gamepad/mouse/telemetry
    sendDatagram(data: ArrayBuffer): void {
        this.transport.datagrams.writable.getWriter().write(data);
    }

    onDatagram(cb: (data: ArrayBuffer) => void): void {
        const reader = this.transport.datagrams.readable.getReader();
        // pump reader in background
    }

    // Reliable, ordered — for file transfer / clipboard
    async createStream(): Promise<WebTransportBidirectionalStream> {
        return this.transport.createBidirectionalStream();
    }
}
```

### Connection URL Format

```
wt://gateway.freecompute.io:8084/session/{sessionID}?token={jwt}
```

---

## Phase 4: Fallback & Coexistence

### Capability Negotiation

```json
// GET /capabilities response additions
{
    "transports": [
        { "type": "webrtc",  "priority": 1, "signaling": "/signal/" },
        { "type": "websocket", "priority": 2, "endpoint": "/ws/" },
        { "type": "webtransport", "priority": 0, "endpoint": "wt://:8084/" }
    ]
}
```

Client tries transports in priority order. Falls back if connect fails:

```
WebTransport → WebRTC DataChannel → WebSocket fallback
```

### Simultaneous Usage

A session can use multiple transports concurrently:

- **WebRTC** for video/audio media tracks
- **WebTransport** for gamepad input (lowest latency)
- **WebSocket** for clipboard/file transfer (widest compatibility)

---

## Phase 5: QUIC Connection Migration

QUIC's connection ID allows seamless network transitions:

1. Client switches WiFi → cellular.
2. QUIC connection ID remains the same.
3. Gateway continues sending packets to the new client address.
4. No ICE restart, no SDP renegotiation, no visible disruption.

This is the killer feature for mobile gaming sessions and users on flaky connections.

---

## Dependency & Build Impact

| Dependency | Version | Size | License |
|------------|---------|------|---------|
| `github.com/quic-go/quic-go` | v0.48+ | ~500 KB | MIT |
| `github.com/quic-go/webtransport-go` | v0.8+ | ~50 KB | MIT |

Both are pure Go (no CGo), already used in production by projects like `github.com/lucas-clemente/quic-go`.

---

## Testing Strategy

| Layer | Test | Tools |
|-------|------|-------|
| Unit | QUIC stream read/write round-trip | `go test` + `quic-go` in-memory transport |
| Integration | WebTransport ↔ WebSocket fallback parity | Custom test harness, both transports |
| Browser | WebTransport API in Chrome/Firefox | Playwright, `webtransport-echo` test page |
| Scale | 1000 concurrent QUIC connections, datagram throughput | Custom `quic-go` load generator |

---

## Timeline

| Phase | Duration | Deliverable |
|-------|----------|-------------|
| P1: QUIC Listener | 1 week | Gateway listens on :8084, accepts QUIC connections |
| P2: Stream Mux | 1 week | Datagram input relay to session (WebTransport → VM) |
| P3: Frontend Client | 1 week | WebTransport client class with WebSocket fallback |
| P4: Cap Negotiation | 3 days | /capabilities advertises WebTransport, client auto-selects |
| P5: Connection Migr. | 1 week | Session survives network change, metrics on migration count |
| Total | ~5 weeks | Shipping in alpha |
