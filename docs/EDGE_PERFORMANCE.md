# Edge Performance & New Features

Low-latency edge architecture, P2P mesh distribution, and client-side optimizations.

## Edge Caching Architecture

```
                     ┌──────────────────┐
Client ──► BunnyCDN ──► Gateway ──► Host Agent
              │
              ├── Static assets (immutable, long cache)
              ├── API responses (short cache with ETag)
              ├── WebSocket upgrade (bypass, pass-through)
              └── Proxy routes (bypass or edge-relay)
```

### Cache Strategy
- **Immutable assets** — hash-based filenames, `Cache-Control: public, max-age=31536000, immutable`
- **API responses** — `Cache-Control: no-cache` with `ETag` for validation
- **Tunnel routes** — `Bypass` cache for all `/proxy/*`, `/ws/*`, `/connect/*`, `/agent/*`
- **Signaling** — SSE bypass cache, never buffer
- **Relay discovery** — Short TTL (60s) for P2P candidate updates

## P2P Mesh for Streaming Distribution

```
Host Agent ◄──► Peer A ◄──► Peer B
    │                        │
    └───── Gateway ──────────┘
```

When multiple clients access the same stream (collaborative viewing, spectator mode):
- Host agent sends one upstream to gateway
- Gateway fans out via WebRTC P2P mesh between peers
- Reduces host agent upload bandwidth by N-1
- Signalling uses existing `/signal/{route}/rooms/{room}` infrastructure

## Distributed Gateway Mesh

```
          ┌── Gateway A (us-east) ── Host Agent 1
Client ── ┤── Gateway B (eu-west) ── Host Agent 2
          └── Gateway C (ap-southeast) ── Host Agent 3
                  │
                  └── Shared state via NATS/Redis
```

- Multiple gateway instances share session and route state
- Client routes to nearest gateway via DNS geo-routing
- Host agent connects to nearest gateway for lowest registration latency
- Session migration across gateways for persistent connections

## WebTransport / QUIC Support

### WebTransport (HTTP/3)
- Browser API for low-latency, unordered, unreliable data
- Lower overhead than WebSocket for game input and real-time data
- Native multiplexing without head-of-line blocking
- Unidirectional streams for gamepad input, bidirectional for reliable data

### QUIC Forwarding
- Gateway terminates QUIC from client, forwards to upstream via TCP or QUIC
- 0-RTT connection establishment for repeat clients
- Connection migration — session survives IP/network change

## Predictive Prefetching

- Client-side heuristic predicts next likely route or resource
- Pre-connect to predicted proxy endpoints via `<link rel="preconnect">`
- Prefetch DNS, TLS, and HTTP/2 connection pool
- Reduces perceived latency by 100-200ms on navigation

## Client-Side Connection Health

### Real-Time Dashboard
- WebSocket-based live metrics feed replaces 5s polling
- Metrics streamed at 1s intervals:
  - Gateway health + component status
  - Active sessions + connection quality
  - Per-route throughput and error rate
  - Host agent availability and load
- Powered by Server-Sent Events from `/events` endpoint

```typescript
// Client subscribes to live metrics
const events = new EventSource('/events');
events.onmessage = (event) => {
  const { type, data } = JSON.parse(event.data);
  updateDashboard(type, data);
};
```

### Connection Quality Scoring
- Composite score (0.0 — 1.0) based on:
  - RTT (weight: 30%)
  - Jitter (weight: 20%)
  - Packet loss (weight: 25%)
  - Throughput relative to codec requirement (weight: 25%)
- Visual indicator in console: green (≥0.8), yellow (≥0.5), red (<0.5)
- Triggers adaptive bitrate adjustment when score drops

## Session Persistence & Migration

### Session Handoff
- Active WebRTC session migrates from one host agent to another
- Use case: host maintenance, load balancing, hardware failure
- Flow:
  1. Gateway signals migration to client and target host
  2. Client opens new peer connection to target host
  3. Target host pre-loaded with session state (encoder config, codec, bitrate)
  4. Old session closes after migration complete
  5. Session ID remains stable — client transparent to change

### Persistent Session Storage
- Session metadata persisted to database (session ID, client ID, codec, preset)
- Recoverable after gateway restart
- 24-hour session TTL with periodic refresh

## Protocol-Level Compression

### Per-Protocol Compression Strategy

| Protocol | Compression | Method |
|----------|-------------|--------|
| WebSocket text (signaling) | Enabled | permessage-deflate |
| WebSocket binary (tunnel) | Optional | zstd per-frame |
| HTTP proxy | Configurable | gzip/brotli |
| WebRTC data channel | Enabled | LZ4 for small messages |
| File transfer | Chunked | zstd at application level |

### zstd for Tunnel Data
- Higher compression ratio than gzip at similar speed
- Dictionary-based compression for repeated protocol patterns
- Negotiated during tunnel setup via `X-FreeCompute-Compression` header

## Frontier Technologies

### WebGPU Encoder Offload
- Client-side WebGPU compute shaders for video post-processing
- Reduce host agent encoder load by 20-30%
- Sharpening, color correction, HDR tonemapping on client

### WASM Tunnel Client
- WebAssembly-based WebSocket frame processor in browser
- Zero-copy frame parsing and reassembly
- Lower GC pressure and jitter compared to JavaScript implementation
- Targeted for WebOS desktop environment

### Machine Learning Bandwidth Estimation
- Lightweight model predicts optimal bitrate from packet timing patterns
- Replaces heuristic-based adaptation with learned model
- ~100KB model delivered with WebOS client
- Updates via background model fetch
