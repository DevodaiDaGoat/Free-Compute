# Connection Quality & Streaming Optimization

Strategies for faster connections, lower latency, and adaptive quality.

---

## 1. WebRTC Bandwidth Estimation (BWE)

**Current state:** Codec bitrate is static — set once at session creation and never adjusted.

**Target:** Real-time adaptive bitrate driven by the `NetworkMonitor` in `webrtc.go`.

### Architecture

```
NetworkMonitor (per-session)
    │
    ├── Reads: RTCP Receiver Reports (packet loss, jitter)
    ├── Reads: RTT from ICE candidate pair stats
    ├── Reads: Observed throughput from RTP stream bytes/timestamps
    │
    └── Emits: NetworkQualitySnapshot every 500ms
                    │
                    ▼
        BandwidthEstimator (per-session goroutine)
                    │
                    ├── Google GCC (send-side) hybrid algorithm
                    │   ├── Loss-based: if loss > 2%, reduce bitrate
                    │   ├── Delay-based: one-way delay gradient → overuse signal
                    │   └── Output: target bitrate every 200ms
                    │
                    └── Applies:
                        ├── Update encoder target bitrate
                        ├── Request keyframe on codec switch
                        └── Emit quality-change event to client
```

### Implementation Plan

1. Parse `webrtc.InboundRTPStreamStats` in the stats collector (`webrtc.go:startStatsCollector`) to extract `BytesReceived`, `PacketsLost`, `Jitter`, `FrameHeight`, `FrameWidth`.
2. Parse `webrtc.RemoteCandidateStats` for `RTT`.
3. Implement `BandwidthEstimator` struct with GCC-over-WebRTC logic (reference: `webrtc/gcc` or port from Chromium's `delay-based-bwe.cc`).
4. Expose `SetTargetBitrate(bps int)` on `EncoderConfig` / `Session`.
5. Plumb rate changes through to codec negotiation (renegotiation or RTCP REMB).
6. Emit `bitrate-change` event to client via data channel for UI feedback.

### Expected Improvement

- **Current:** Static 5 Mbps → 30 Mbps (gaming), no adaptation.
- **Target:** 100 Kbps (congested) → 50 Mbps (ideal), seamless transitions.
- **Latency under load:** Avoids 100%+ packet loss spirals.

---

## 2. WebTransport / QUIC Tunnel Path

**Problem:** Browser WebSocket-over-TCP is susceptible to head-of-line blocking. WebRTC datachannel (SCTP) has high per-message overhead for small control packets.

**Solution:** Add WebTransport (HTTP/3 over QUIC) as a third transport alongside WebSocket and WebRTC.

### When to use WebTransport

| Transport | Best for | Why |
|-----------|----------|-----|
| WebRTC | Media (video/audio) | Native codec integration, hardware acceleration |
| WebSocket | Control channel, file transfer | Ubiquitous browser support, simple framing |
| WebTransport | Gamepad input, cursor moves, chat | Unreliable/unordered mode, 0-RTT, no HOL blocking |

### Architecture

```
Browser                      Gateway                     Host Agent
  │                           │                           │
  ├── WebRTC (media) ─────────┤───────────────────────────┤
  ├── WebSocket (control) ────┤───────────────────────────┤
  └── WebTransport ───────────┤───────────────────────────┤
      (input, telemetry)      │                           │
          │ QUIC stream       │                           │
          ▼                   ▼                           ▼
     WT session          WT endpoint                  WT client
     (datagrams)         (quic-go)                    (quic-go)
```

### Implementation

1. **Gateway:** Embed `github.com/quic-go/quic-go` for QUIC listener on a separate port (e.g., :8084).
2. **WebTransport framing:** Implement HTTP/3 datagram (RFC 9221) for unreliable messages, WebTransport bidirectional streams for reliable control.
3. **Frontend:** Use `WebTransport` API (Chrome 97+, Firefox 114+) as a progressive enhancement — fallback to WebSocket when unavailable.
4. **Route mapping:** Expose `wt://{routeID}` in `/capabilities` response for client auto-discovery.

### Expected Improvement

- **Input latency:** Gamepad/keyboard/mouse over WebTransport datagrams reduces p50 latency by 30-50% vs WebSocket (no TCP head-of-line blocking).
- **Packet loss recovery:** QUIC's connection migration handles NAT rebinding transparently — sessions survive network changes.

---

## 3. Adaptive Codec Selection

**Current state:** Codec is chosen once at session creation (`safe` → H.264/VP8, `fast` → H.265/H.264). No runtime switching.

**Target:** Runtime codec/profiles switching based on network conditions:

```
NetworkCondition         Codec Profile
─────────────────────────────────────────────────
Excellent (<5ms jitter)  H.265 Main, 4:4:4, high bitrate
Good (<15ms jitter)      H.264 High, 4:2:0, adaptive bitrate
Fair (<30ms jitter)      VP9, 4:2:0, reduced resolution
Poor (>30ms jitter)      VP8, 4:2:0, low resolution, high keyframe interval
```

### Signaling Flow

1. BWE detects sustained condition change (e.g., jitter > 30ms for 2 seconds).
2. Gateway sends `codec-switch-proposal` to client via data channel.
3. Client responds with accepted codec + capabilities.
4. Gateway creates new `RTCRtpSender.replaceTrack()` or triggers renegotiation.
5. Gateway sends `codec-switch-confirmed` with new parameters.
6. Gateway requests keyframe from encoder for immediate recovery.

### Expected Improvement

- **Resilience:** Sessions survive 40%+ packet loss (switch to VP8 with high keyframe interval).
- **Quality:** Under ideal conditions, H.265 Main profile delivers 40% better compression than H.264 for the same bitrate.

---

## 4. Edge Relay Optimization

**Current state:** Relay path is CDN → Gateway → Host. No dynamic relay selection.

**Target:** Multi-path, anycast-aware relay selection.

### Component

| Component | Description |
|-----------|-------------|
| **Relay Discovery** | Client fetches `/relays` at session start; receives ranked list of relay nodes by latency |
| **Probe** | Client sends 3x STUN-like binding requests to top 5 relays; measures RTT |
| **Select** | Gateway selects relay with lowest combined client-to-relay + relay-to-host RTT |
| **Fallback** | If primary relay degrades, seamless switch to secondary (QUIC connection migration) |

### Relay Health Metrics

Gateway maintains per-relay:
- P50/P95 latency to each region
- Current active relay sessions
- Bandhead utilization
- Error rate (failed connections, timeouts)

### Expected Improvement

- **Client-to-host latency:** Reduced by 30-60ms by choosing nearest working relay.
- **Failover:** <1 second cutover without session disconnect.

---

## 5. Session Migration (Connection Survivability)

**Current state:** When a host goes offline, the session enters `Reconnecting` state permanently. User must start a new session.

**Target:** Transparent session migration to a new host.

### Migration Protocol

```
Session on Host A is running
        │ Host A sends graceful shutdown notice
        ▼
Gateway freezes session state (VM snapshot / container checkpoint)
        │ Scheduler selects Host B
        ▼
Gateway forwards session context to Host B
        │ Host B restores VM / container state
        ▼
Gateway sends `SDP renegotiate` to client
        │ Client ICE restarts to Host B
        ▼
Session resumes on Host B with same session ID
```

### Preconditions

- VM storage on shared volume (NFS/Ceph) or S3-backed snapshot.
- Host agents support checkpoint/restore (CRIU for containers, QEMU snapshots for VMs).
- Network state re-established via ICE restart.

### Expected Improvement

- **Uptime:** Zero-downtime host maintenance (patches, hardware swaps).
- **Resilience:** Unplanned host failure recovery in < 10 seconds.

---

## 6. Connection Prefetch & Pool Warmup

**Problem:** New sessions incur cold-start latency: WebRTC ICE checks, codec negotiation, encoder initialization.

**Solution:** Pre-warm resources before the user clicks "Connect."

### Pre-warm Pipeline

```
User opens WebOS desktop or settings panel
        │
        ▼
Gateway pre-allocates session slot (no host assignment yet)
        │
        ▼
Frontend pre-fetches:
  ├── /capabilities (cached 60s)
  ├── /relays (cached 30s)
  └── STUN/TURN credentials (cached 300s)
        │
        ▼
On "Connect" click:
  ├── ICE gathering already in progress
  ├── Encoder process already spawned (idle, minimal resource)
  └── Host pre-selected by region/resource-class
        │
        ▼
Session ready in < 1 second (vs. 3-5 seconds cold)
```

### Expected Improvement

- **Time-to-stream:** 5 seconds → under 1 second for repeat connections.
- **User-perceived latency:** Connection feels instant.

---

## 7. WebRTC Data Channel Optimization

**Current state:** Single data channel for all control messages (input, clipboard, file transfer, telemetry).

**Problem:** A large file transfer blocks low-latency gamepad input on the same SCTP stream.

### Solution: Multi-Channel Data Layer

```
DC ID | Purpose         | Reliability  | Priority | Ordered
──────┼─────────────────┼──────────────┼──────────┼────────
0     | Session control | Reliable     | Highest  | Yes
1     | Input events    | Unreliable   | High     | No
2     | Gamepad state   | Unreliable   | High     | No
3     | Audio metadata  | Reliable     | Medium   | Yes
4     | Clipboard       | Reliable     | Medium   | Yes
5     | File transfer   | Reliable     | Low      | Yes
6     | Telemetry       | Unreliable   | Low      | No
```

- **Unreliable + Unordered** for input events eliminates head-of-line blocking.
- **Low-priority** file transfers cannot starve gamepad input.
- **Per-channel flow control** prevents one channel from exhausting the SCTP send buffer.

### Expected Improvement

- **Input responsiveness:** Sub-millisecond variance even during concurrent file transfers.
- **Deterministic latency:** Gamepad inputs always delivered within the same RTT window.

---

## 8. Media Ingest Optimization

**Problem:** `HandleMediaIngest` allocates `make([]byte, 1500)` per HTTP read and processes frames sequentially.

### Improvements

1. **Zero-copy receive:** Use `splice()` or `sendfile()` syscalls where available for host-agent → gateway media path.
2. **Batched RTP write:** Buffer 3-5 ms of audio/video frames, write as a single RTP batch. Reduces syscall rate by 200x.
3. **Jitter buffer:** 20ms adaptive jitter buffer on gateway side to smooth out host-side encoding jitter.

### Expected Improvement

- **CPU usage:** Media ingest path CPU reduced by ~40%.
- **Frame delivery consistency:** Eliminates micro-bursts caused by variable encode times.
