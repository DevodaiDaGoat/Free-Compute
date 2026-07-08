# Next-Generation Features

New feature proposals beyond the current roadmap, focused on performance, user experience, and platform scale.

---

## 1. WebAssembly (WASM) Native Client

**What**: A WebAssembly-based client runtime that runs in the browser at near-native performance, providing alternative protocol implementations that bypass JavaScript overhead.

**Where**: `packages/wasm-client/` — shared between frontend, desktop Electron wrapper, and headless testing.

**Capabilities**:
- **Quadratic** performance for WebRTC SDP parsing/negotiation
- **TURN/STUN** protocol handling in WASM (bypass JS DNS resolution overhead)
- **Opus** audio decoding in WASM (lower latency than browser WebCodecs on some platforms)
- **Bitstream** parsing for H.264/H.265 annex B (keyframe detection without JS intervention)

**Architecture**:

```
┌─────────────────────────────────────────┐
│              Browser JS                  │
│  ┌──────────────────────────────────┐   │
│  │    WASM Client (Go or Rust)      │   │
│  │  ┌──────┐ ┌──────┐ ┌─────────┐  │   │
│  │  │TURN  │ │SDP   │ │ Opus    │  │   │
│  │  │STUN  │ │Parse │ │ Decode  │  │   │
│  │  └──────┘ └──────┘ └─────────┘  │   │
│  │  ┌──────────────────────────┐   │   │
│  │  │  WebTransport Shim       │   │   │
│  │  └──────────────────────────┘   │   │
│  └──────────────────────────────────┘   │
└─────────────────────────────────────────┘
```

**Implementation**:
- Build WASM module in Go (`GOOS=js GOARCH=wasm`) or Rust via `wasm-pack`
- Expose via `Promise`-based API from JS
- Lazy-load on first streaming session (not on page load)

---

## 2. Predictive Client-Side Frame Rendering

**What**: Use frame interpolation and dead-reckoning on the client to mask network jitter and packet loss. The client predicts the next N frames based on motion vectors and renders them locally while waiting for the next keyframe.

**How it works**:

```
Time ──────────────────────────────────────────────►

[Keyframe] [Delta] [Delta] [LOST] [Delta] [Delta]
    │          │       │              │       │
    ▼          ▼       ▼              ▼       ▼
 Render    Render   Render   ┌────────────┐  Render
                              │ Predict    │
                              │ Frame 3    │
                              │ from 1→2   │
                              │ motion     │
                              └────────────┘
```

**Client-side** (in WASM or WebCodecs):
1. Track last two decoded frames and their motion vectors
2. On packet loss or jitter spike → run lightweight interpolation
3. Display interpolated frame instead of freezing/stuttering
4. When next real frame arrives, snap back or blend

**Requirements**:
- WebCodecs API for low-level frame access (Chrome 86+, Firefox 130+)
- Motion vector extraction from H.264/VP9 bitstream
- Simple linear interpolation for mouse/keyboard input prediction

**Acceptable**: Predictive frames are visually acceptable for desktop and gaming sessions where motion is somewhat predictable (scrolling, camera pan, mouse movement). UI elements with sudden changes (new window opens) require real frames.

---

## 3. ML-Based Bitrate Prediction

**What**: Train a lightweight ML model (or use a decision forest) on historical connection quality data to predict optimal bitrate settings before congestion occurs.

**Training features**:
- Last 30 s of RTT (min, max, mean, variance)
- Last 30 s of packet loss (rate, burst pattern)
- Last 30 s of received bitrate
- Client platform (browser, OS)
- Network type (WiFi, Ethernet, 4G, 5G)
- Time of day
- Geographic region pair

**Inference** (runs on gateway per session):

```go
type BitratePredictor struct {
    model *onnx.RuntimePredictor  // ONNX model
}

func (p *BitratePredictor) Predict(snapshot QualityHistory) BitrateRecommendation {
    features := p.extractFeatures(snapshot)
    output := p.model.Infer(features)
    return BitrateRecommendation{
        TargetBitrate: int(output[0]),
        MaxBitrate:    int(output[1]),
        MinBitrate:    int(output[2]),
        Confidence:    output[3],
    }
}
```

**Fallback**: Rule-based adaptive bitrate (see NETWORK_ACCELERATION.md §4) when ML model is unavailable or confidence < 0.7.

**Training pipeline**: Collect telemetry from production → label with user satisfaction signals (session duration, reconnects, manual quality changes) → train weekly model → A/B test vs baseline.

---

## 4. Preemptive Session Preconnect

**What**: When a user shows intent to start a session (clicks "Launch Desktop", opens WebOS, or navigates to gaming mode), the gateway pre-allocates resources and partially establishes the streaming path before the user completes the action.

**Trigger events**:
1. User hovers over "Launch" button → pre-warm
2. WebOS boot sequence starts (BIOS screen) → pre-allocate session slot
3. User selects a VM image → start agent tunnel pre-connect
4. User fills in connection settings → begin WebRTC port allocation

**Pre-warm pipeline**:

```
User intent detected
        │
        ▼
  Allocate session ID (no expiry)
        │
        ▼
  Reserve encoder slot on gateway
        │
        ▼
  Pre-dial host agent (agent pool take)
        │
        ▼
  Pre-allocate UDP ports for media relay
        │
        ▼
  Generate and cache SDP offer
        │
        ▼
  User clicks Go →  SDP exchange + ICE
        │                 (200 ms vs 3 s)
        ▼
  Streaming in <500 ms
```

**Gateway API**: Add `POST /sessions/prewarm` that returns a session token but does not start billing. The token is consumed by the real `POST /sessions/` call. Unused pre-warps expire after 60 s.

---

## 5. Shared Memory UDP Gateway (AF_VSOCK / UNIX DGRAM)

**What**: For host agents running on the same physical machine as the gateway (co-located deployment), replace TCP tunnels with shared memory or UNIX domain datagram sockets for zero-copy inter-process communication.

**When**: Gateway and host agent are deployed on the same host (typical for single-tenant or dedicated hosting).

**Architecture**:

```
┌─────────────────────┐
│     Gateway          │
│  ┌───────────────┐   │
│  │  UNIX DGRAM   │──┼──► /tmp/fc-gateway.sock
│  │  Listener     │   │
│  └───────────────┘   │
└──────────┬──────────┘
           │ splice() / sendmsg()
           ▼
┌─────────────────────┐
│   Host Agent         │
│  ┌───────────────┐   │
│  │  UNIX DGRAM   │◄─┼──► /tmp/fc-agent.sock
│  │  Reader       │   │
│  └───────────────┘   │
└─────────────────────┘
```

**Benefits over TCP loopback**:
- No TCP stack overhead (no checksum, no segmentation, no congestion control)
- `splice(2)` works on UNIX sockets on Linux 5.11+
- File descriptor passing (pass QEMU monitor socket to gateway)
- Lower latency: ~2-5 µs vs ~20-50 µs for TCP loopback

**Detection**: Auto-detect co-located deployment via `/proc/self/cgroup` or env var `FREECOMPUTE_CO_LOCATED=true`.

---

## 6. Persistent QUIC Connection Migration

**What**: Maintain streaming sessions across network changes (WiFi ↔ cellular, VPN on/off, NAT rebinding) using QUIC connection migration. The client never sees a disconnect; the stream continues seamlessly.

**Host-side**: Gateway QUIC listener supports connection migration natively (migrating `quic.Connection` to a new `io.Writer`/`io.Reader` pair when the client's IP:port changes).

**Client-side**: In the WebTransport client, handle the `quic.ConnectionState` event for path degradation and use 0-RTT to re-establish on the new path.

**Use case**: A user on a laptop walks from office to meeting room. The WiFi AP changes, the client gets a new IP, but the WebRTC video/audio stream continues without interruption. The gateway sees a new `quic.Connection` from the new IP, matches it to the existing session via the session token in the ALPN or early data.

---

## 7. WebRTC Simulcast with Receiver-Side Layering

**What**: Encode video in 3 spatial layers (low/medium/high resolution) and let the client request the appropriate layer based on viewport size and connection quality.

**Layers**:

| Layer | Resolution | Bitrate | When Used |
|-------|-----------|---------|-----------|
| Low | 480p | 500 Kbps | Thumbnail / poor connection |
| Medium | 720p | 2 Mbps | Default / good connection |
| High | 1080p | 8 Mbps | Fullscreen gaming / excellent connection |

**Transition**: Client requests layer switch via WebRTC `RTCRtpSender.setParameters()`. Gateway responds within 1 frame interval by adjusting encoder output. No renegotiation needed.

**Implementation**: Use Pion's `Track` with multiple `RTPCodec` instances, or use VP9 SVC (scalable video coding) which encodes layers in a single stream.

---

## 8. Frontend: Virtualized Desktop with Canvas Rendering

**What**: Replace the current DOM-based window manager with an HTML5 Canvas + OffscreenCanvas renderer. Each app paints to its own OffscreenCanvas, composited on a main Canvas at 60 FPS by the window manager.

**Benefits**:
- No DOM reflow/layout cost for window dragging/resizing
- Compositing via GPU (Canvas2D or WebGL)
- Window contents don't trigger browser layout passes
- Smoother animations at 60 FPS regardless of DOM complexity

**Architecture**:

```
┌─────────────────────────────────────┐
│         Main Canvas (WebGL)         │
│  ┌──────────┐  ┌──────────┐        │
│  │ Terminal │  │ Browser  │        │
│  │ Canvas   │  │ Canvas   │        │
│  └──────────┘  └──────────┘        │
│  ┌──────────┐                      │
│  │Settings  │                      │
│  │ Canvas   │                      │
│  └──────────┘                      │
└─────────────────────────────────────┘
```

**Implementation**: Each app component renders to an `OffscreenCanvas` (or regular Canvas for visibility). The `WindowManager` compositor runs a RAF loop that redraws only dirty regions. Window chrome (title bar, resize handles) are drawn on the compositor canvas, not DOM elements.

---

## 9. Input Event Compression and Batching

**What**: Compress and batch input events before sending over the network. A mouse move at 1000 Hz generates 1000 events/s — far more than the display refresh rate.

**Techniques**:
- **Dead reckoning**: Skip intermediate positions, send only endpoint + velocity vector
- **Delta compression**: Send relative movements instead of absolute coordinates
- **Temporal batching**: Accumulate events for 4 ms, send as batch
- **Priority queuing**: Keyboard/gamepad button events sent immediately (latency-critical); mouse movement batched (throughput-sensitive)

**Wire format**:

```protobuf
message InputBatch {
    repeated MouseEvent mouse = 1;     // batched mouse moves
    KeyboardEvent keyboard = 2;        // latest key state
    GamepadState gamepad = 3;          // current gamepad snapshot
    uint32 timestamp_ms = 4;           // batch creation time
    bool has_precise_mouse = 5;        // raw input mode
}
```

**Implementation**: Input events collected in a ring buffer on the client. Every 4 ms (250 Hz), serialize and send via WebRTC data channel or WebTransport datagram. The gateway applies events in order with interpolation between batches.

---

## 10. Gateway Replication via Raft

**What**: Run multiple gateway instances behind a load balancer, with session state replicated via Raft consensus. Enables zero-downtime upgrades and horizontal scaling.

**Session state to replicate**:
- Active WebRTC sessions and their ICE candidates
- Agent pool connections (route → agent mapping)
- Signal room state (in-flight messages)
- Auth tokens and rate limiter state

**Stateless vs Stateful split**:
- **Stateless**: HTTP proxy, health check, capabilities, storage (delegated to file-service)
- **Stateful**: WebRTC sessions, agent pool, signaling — replicated via Raft

**Implementation**: Use `hashicorp/raft` for consensus. Each gateway joins the Raft cluster on startup. Session writes go through Raft (majority commit), reads are served locally (linearizable reads via `LeaderCh`).

**Gateway discovery**: Gateways discover each other via `FREECOMPUTE_PEER_ADDRS` env var or DNS SRV records.

---

## Implementation Priority Matrix

| Feature | Effort | User Impact | Complexity | Phase |
|---------|--------|-------------|-----------|-------|
| Input compression & batching | 1 week | High (latency) | Low | Phase 2 |
| Preemptive session preconnect | 1 week | High (startup) | Medium | Phase 2 |
| WASM client core | 3 weeks | Medium | High | Phase 3 |
| ML bitrate prediction | 4 weeks | High (quality) | High | Phase 3 |
| Persistent QUIC migration | 3 weeks | Medium | High | Phase 3 |
| WebTransport tunnels | 3 weeks | High (latency) | High | Phase 3 |
| Predictive frame rendering | 6 weeks | High (smoothness) | Very High | Phase 4 |
| Simulcast layers | 3 weeks | Medium | High | Phase 4 |
| Virtualized Canvas desktop | 8 weeks | Medium | Very High | Phase 4 |
| Raft gateway replication | 6 weeks | High (reliability) | Very High | Phase 4 |
| Shared memory UDP gateway | 2 weeks | Medium (co-located) | Medium | Phase 4 |
