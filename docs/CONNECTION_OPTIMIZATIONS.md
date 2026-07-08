# Connection Optimizations

Performance improvements for the connection pipeline — lower latency, higher throughput, better resource utilization.

## Connection Multiplexing

### HTTP/2 Stream Multiplexing
- Share a single TCP connection for multiple concurrent proxy streams
- Eliminates head-of-line blocking for HTTP proxy routes
- Reduces connection establishment overhead (TCP + TLS handshakes per route)
- Gateway already has HTTP/2 support in `http.Transport` — enable for proxy routes

### WebRTC Data Channel Multiplexing
- Single peer connection carries multiple logical channels:
  - Input events (keyboard, mouse, gamepad) — high priority, low latency
  - Video/audio RTP streams — timed media
  - File transfer chunks — bulk throughput
  - Clipboard sync — small messages
- Eliminates need for separate HTTP POST calls for each data type

## Adaptive Bitrate Streaming

```
Client ──RTT/jitter/loss──▶ Gateway ──codec/bitrate/resolution──▶ Host Agent
                                │
                                └── Real-time quality adaptation loop
```

- Monitor per-packet RTT, jitter, and packet loss from WebRTC statistics
- Adjust video bitrate, resolution, and FPS dynamically based on:
  - Network congestion (rising RTT + packet loss → reduce bitrate)
  - Available bandwidth (estimated from throughput)
  - Encoder load and frame drop rate
- Presets map to adaptation strategies:
  - `safe` — conservative ramp-up, early resolution drop
  - `fast` — aggressive quality, late degradation
- Implement quality-of-service feedback via RTCP Receiver Reports

## Connection Pooling

### TCP/WebSocket Buffer Pool
- Reuse 32KB copy buffers via `sync.Pool` instead of per-goroutine allocation
- Already started with `byteBufferPool` — extend to:
  - `copyConnWithIdle()` in TCP bridging
  - `bridgeWebSocketToTCP()` and `bridgeWebSocketToUDP()` goroutines
  - WebRTC media ingest 1500-byte RTP buffers

### UDP Client Map GC
- Periodic sweep goroutine removes stale UDP client entries > idle timeout
- Prevents memory leak from disconnected clients

## WebSocket Compression
- Enable `permessage-deflate` extension for signaling WebSocket
- Reduces SDP offer/answer and ICE candidate message size by 60-80%
- Low CPU overhead with streaming compression

## Signaling Improvements

### Server-Sent Events (SSE) for Signaling
- Replace long-poll with SSE for push delivery
- Eliminates per-client goroutine blocking for 25-second poll windows
- Lower overhead under many concurrent signaling clients

| Metric | Long-Poll | SSE |
|--------|-----------|-----|
| Goroutines per client | 1 blocked ~25s | 0 blocked |
| Message latency | Up to poll interval | <10ms |
| Server memory | Notification channel per waiter | None |

### TTL-Based Room Expiry
- Background goroutine sweeps expired rooms at fixed interval
- No full map scan on every message append
- O(n) → O(1) append path

## Session ID Security
- Current: `time.Now().UnixNano()` — predictable, collision-prone
- Improved: `crypto/rand` 16-byte hex-encoded UUID
- Prevents session ID guessing and enumeration

## WebRTC Session Limiter
- Acquire semaphore slot only after validation passes
- Always release on error path to prevent slot leak
- Add context-based cancellation for pending session creations

## Connection Timeouts & Deadlines

| Connection Type | Current | Optimized |
|-----------------|---------|-----------|
| TCP idle timeout | Configurable | + aggressive keepalive (15s) |
| WebSocket frame | 1MB max | + per-frame deadline |
| HTTP proxy | Configurable | + per-request timeout |
| UDP client GC | None | 60s idle sweep |
| WebRTC session | 24h TTL | + inactivity timeout (5min) |

## Benchmark Targets

| Metric | Before | After |
|--------|--------|-------|
| WebSocket tunnel throughput | ~200 Mbps | ~400 Mbps |
| Concurrent UDP clients | Leaks | 100k+ stable |
| Signaling poll latency | 25s max | <10ms (SSE) |
| Session creation rate | ~100/s | ~1000/s |
| Per-connection memory | 32KB × goroutines | Pooled, reused |
