# Fastest Path: Minimal-Latency Connection Design

## Goal

Achieve the lowest possible connection setup latency and the lowest per-byte latency for browser, WebOS, and native clients accessing the FreeCompute gateway behind BunnyCDN. This document builds on the existing transport primitives (`buffer_pool.go`, `connection_fusion.go`, `dns_cache.go`, `agent_pool.go`, `server.go`) and closes the remaining gaps that prevent the codebase from hitting sub-RTT cold-connect and near-zero warm-reuse targets.

For the full acceleration landscape, see `NETWORK_ACCELERATION.md`. For code-level micro-optimizations already identified, see `CONNECTION_FAST_TRACK.md`. This doc focuses on transport selection, connection coalescing, TCP tuning, DNS caching, edge/QUIC ingress, and session resumption — ordered by implementation priority.

---

## 1. Transport Auto-Negotiation

### Problem

Clients today pick a single transport upfront (HTTP proxy, WebSocket tunnel, or WebRTC) without comparing alternatives. Under BunnyCDN, the edge may terminate TCP, so wasted RTT is paid before the gateway even sees the request.

### Solution

Use the existing `/capabilities` handler (`server.go:472`) as the source of truth for transport negotiation. Extend the response with a `preferredTransports` array ranked by client class:

| Client | Rank 1 | Rank 2 | Rank 3 |
|--------|--------|--------|--------|
| Browser (modern) | `webtransport` | `webrtc-data-channel` | `websocket` |
| Browser (legacy) | `websocket` | `theora-connect` | `http-connect` |
| WebOS | `websocket` | `http-connect` | `tcp` |
| Native | `tcp` / `quic` | `websocket` | `http-connect` |

### Where to add

- **`server.go:472` (`handleCapabilities`)** — add `preferredTransports` map to the JSON response. This is the only negotiation surface the client needs to query.
- **Client side (`apps/frontend`)** — fetch `/capabilities` at startup, cache for session, pick the highest-ranked transport the client platform supports.
- **BunnyCDN edge config** — ensure `Upgrade` and `Connection` headers are forwarded for WebSocket/WebTransport upgrades, and that QUIC (HTTP/3) is enabled on the BunnyCDN pull zone so 0-RTT is possible for repeat connections.

### Transport fallback chain

```
attempt 1 — WebTransport (QUIC, 0-RTT if session ticket cached)
    ↓ not supported / handshake failure
attempt 2 — WebRTC data channel (via /signal, browser only)
    ↓ ICE failure / no STUN/TURN
attempt 3 — WebSocket tunnel (/ws/{routeID})
    ↓ proxy/upgrade block
attempt 4 — HTTP CONNECT (/connect/{routeID})
    ↓ not available
attempt 5 — HTTP proxy (/proxy/{routeID}/...)
```

The chain should be evaluated per-request with a hot-path fast failure: if WebTransport is not advertised, skip straight to WebSocket or HTTP CONNECT without probing.

---

## 2. Connection Coalescing / Multiplexing

### Problem

A single session opens N separate transport connections (video, audio, input, clipboard, file transfer). Each connection pays a TCP+TLS (or QUIC) handshake round trip. Under BunnyCDN, the edge multiplexes at the TLS layer, but multiple connections still compete for congestion windows and increase head-of-line blocking.

### Existing capabilities

- `connection_fusion.go` — `sessionMultiplexer` already implements WebSocket-level frame multiplexing with typed streams (`StreamType`), sequence numbers, and checksums. The `wsFusedTransport` implementation wraps `webSocketBridge`.
- `server.go` — `/prewarm` handler (`handlePrewarm`, line 444) returns a keep-alive HTTP response that keeps a socket open for 15 seconds.

### What needs to be done

1. **Upgrade `sessionMultiplexer` to support QUIC streams** — add a `quicFusedTransport` implementation alongside `wsFusedTransport`. The `fusedTransport` interface (`ReadFrame`/`WriteFrame`/`Close`) maps cleanly to QUIC unidirectional streams. When the transport is QUIC, each `StreamType` gets its own QUIC stream; when it is WebSocket, the existing frame encapsulation is used.

2. **Pre-connect the multiplexed transport at session creation** — once `/sessions/` is created, open one multiplexed transport (WebSocket or QUIC) and keep it warm. Subsequent per-feature paths (`/input/`, `/audio/`, `/clipboard/`, `/transfer/`) should upgrade or reuse the existing transport instead of negotiating a new one.

3. **Use `/prewarm` as a reservation mechanism** — before session creation, `GET /prewarm` establishes the TCP+TLS state and holds the connection. The client then reuses that socket for the first real request. This is particularly valuable behind BunnyCDN where the edge-to-origin TLS handshake dominates cold latency.

4. **Reference `CONNECTION_FUSION.md`** — the framework document describes the full vision. This implementation phase focuses on the gateway-side multiplexer and the QUIC transport shim.

---

## 3. TCP Tuning Fix: Wire `TCPBufferSize` into `applyTCPSocketOptions`

### Current bug

`config.go:154` loads `FREECOMPUTE_TCP_BUFFER_SIZE` (default 2 MB) into `Config.TCPBufferSize`, but `buffer_pool.go:115-116` ignores it:

```go
unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_SNDBUF, 2_097_152)
unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_RCVBUF, 2_097_152)
```

The hardcoded `2_097_152` value silently shadows the config knob. Operators tuning `FREECOMPUTE_TCP_BUFFER_SIZE` see no effect.

### Fix

Change the two lines to read from a parameter:

```go
// In applyTCPSocketOptions, accept a bufferSize int parameter
unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_SNDBUF, bufferSize)
unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_RCVBUF, bufferSize)
```

Callers pass `cfg.TCPBufferSize`:

- `setTCPKeepaliveAggressive` (`buffer_pool.go:124`) — used by `dialViaTailscale` (`server.go:1273`) and `tailscaleHosts` paths.
- `applyTCPListenerOptions` (`buffer_pool.go:134`) — listener options do not need send/receive buffer tuning; leave as-is.
- Future: `applyTCPSocketOptions` should also be called from `dialTCP` and `bridgeTCP` (see `CONNECTION_FAST_TRACK.md` §1e for full list).

### Keep existing tuning

- **BBR** (`TCP_CONGESTION` auto-select, `buffer_pool.go:97-113`)
- **TCP_NOTSENT_LOWAT** (`16_384`, line 95)
- **NODELAY + QUICKACK** (lines 89, 90)
- **Aggressive keepalive** (`TCP_KEEPIDLE=5`, `TCP_KEEPINTVL=1`, `TCP_KEEPCNT=3`, lines 91-93)

### Gap table

| Path | Buffer source | Status |
|------|--------------|--------|
| TCP outbound (proxy) | `2_097_152` hardcoded | **Gap** — should use `cfg.TCPBufferSize` |
| TCP listener | `2_097_152` hardcoded via `SO_SNDBUF`/`SO_RCVBUF` in `applyTCPListenerOptions` | Minor gap — listener tuning is less critical |
| UDP outbound | `s.cfg.UDPBufferSize` (`udp.go:157-158`) | ✅ Wired |
| UDP listener | `udpSocketBufferSize = 4 MB` hardcoded (`udp.go:33-35`) | Minor gap — should also use `cfg.UDPBufferSize` |

---

## 4. DNS Caching for Proxy Targets

### Current state

`dns_cache.go` implements `DNSCache` with LRU eviction, TTL, and a `dnsCacheDialer` that wraps `net.Dialer`. However, `newProxyTransport` (`http_proxy.go:12-30`) ignores it entirely:

```go
dialer := &net.Dialer{
    Timeout:   cfg.DialTimeout,
    KeepAlive: 30 * time.Second,
}
// ...
DialContext: dialer.DialContext,
```

This means every upstream proxy request pays a full DNS resolve on the first connection, and every new idle connection pays again after `IdleConnTimeout`.

### Fix

Wire `DNSCache` into the proxy transport:

1. **Create a singleton `DNSCache` in `NewServer`** (`server.go:101`) with TTL from env (e.g., `FREECOMPUTE_DNS_TTL_SECONDS`, default 60s) and max entries from env (e.g., `FREECOMPUTE_DNS_MAX_ENTRIES`, default 1024).

2. **Replace the dialer in `newProxyTransport`** with `newDNSCacheDialer(dialer, cache)`:

```go
func newProxyTransport(cfg Config, dnsCache *DNSCache) *http.Transport {
    dialer := &net.Dialer{
        Timeout:   cfg.DialTimeout,
        KeepAlive: 30 * time.Second,
    }

    return &http.Transport{
        Proxy:                 http.ProxyFromEnvironment,
        DialContext:           newDNSCacheDialer(dialer, dnsCache).DialContext,
        // ...
    }
}
```

3. **Also use it in `dialViaTailscale`** — `net.Dialer` is currently created inline (`server.go:1270`). Pass a `dnsCacheDialer` there so Tailscale direct routes also benefit from cached DNS.

4. **Expose cache stats** via `/capabilities` or `/metrics` so BunnyCDN edge cache hit ratio can be observed.

### BunnyCDN interaction

BunnyCDN resolves the origin hostname at the edge. If the edge has stale DNS, it will route to the wrong origin. The gateway-side DNS cache does not help here — that is a separate `FREECOMPUTE_EDGE_HOSTNAME` concern. What the gateway DNS cache helps is the **proxy upstream** (`/proxy/{routeID}`) and Tailscale direct routes, where the gateway dials backends internally.

---

## 5. Edge / QUIC: HTTP/3 for `/proxy` and WebTransport Ingress

### What exists

- `config.go:70` — `QUICAddr` field loaded from `FREECOMPUTE_GATEWAY_QUIC_ADDR` (default `:8084`).
- `server.go:298-300` — `/wt/` is registered but returns `501 Not Implemented` because `quic-go` is not imported.
- `handleCapabilities` (`server.go:485-498`) advertises `webtransport` as a transport endpoint.

### What needs to happen

1. **Add `github.com/quic-go/quic-go` and `github.com/quic-go/quic-go/http3`** to `apps/gateway/go.mod` (or use the existing `WebTransportServer` placeholder).

2. **Implement the QUIC listener** in `server.go:Start` — alongside `ListenAndServe`, start a goroutine that uses `quic.ListenAddr` on `cfg.QUICAddr` with TLS config supporting `h3`, `h2`, `http/1.1` ALPN. Serve HTTP/3 for `/proxy/`, `/capabilities`, and `/healthz`, and WebTransport streams for `/wt/`.

3. **HTTP/3 for `/proxy`** — `http3.ServeConn` can handle HTTP requests directly. This gives BunnyCDN clients 0-RTT resumption on repeat visits and removes TCP head-of-line blocking from the proxy path.

4. **WebTransport ingress for `/wt/`** — the existing `fusedTransport` interface in `connection_fusion.go` map directly to WebTransport streams. Add a `quicFusedTransport` implementation that reads/writes QUIC streams. The `sessionMultiplexer` gains a second transport backend with no changes to stream framing.

5. **BunnyCDN config** — enable HTTP/3 on the pull zone, set `QUICAddr` publicly reachable, and advertise `h3` in `/capabilities.transportEndpoints`.

### Future fast path

Once QUIC is in place:
- Gateway-to-agent traffic (currently WebSocket over TCP) can move to a single QUIC connection with multiplexed streams, replacing the `agentPool` model. Each route becomes a QUIC stream. This is the long-term path described in `NETWORK_ACCELERATION.md` §3 (Connection Coalescing).

---

## 6. TLS 0-RTT and Session Resumption

### What exists

Standard Go `crypto/tls` supports session resumption via session tickets and TLS 1.3 0-RTT data. The gateway currently uses Go's default `http.Server` TLS behavior (or no TLS at all if terminated at BunnyCDN).

### What to do

1. **If TLS terminates at the gateway** (not behind BunnyCDN termination) — enable `tls.Config.Certificates` with a config that sets `ClientAuth` and enables session tickets. Go enables session tickets by default in 1.21+. Use `tls.Config.SessionTicketsDisabled = false`.

2. **0-RTT for QUIC** — `quic-go` handles 0-RTT automatically when session tickets are available. No additional code is needed beyond enabling the QUIC listener.

3. **If BunnyCDN terminates TLS** (recommended for BunnyCDN) — the edge already does TLS. The gateway sees plain HTTP/1.1 or HTTP/2 from the edge. In this case, 0-RTT is a BunnyCDN-side concern; the gateway should focus on connection pooling at the edge-to-origin hop. Enable `IdleTimeout` on the HTTP server (`HTTPIdleTimeout`, default 120s) and set `MaxIdleConnsPerHost` high enough to keep a pool of edge-origin connections warm.

4. **HTTP/2 connection coalescing** — `newProxyTransport` already sets `ForceAttemptHTTP2: true` (`http_proxy.go:21`). Ensure `MaxIdleConnsPerHost` (`cfg.MaxIdleConnsPerHost`, default 128) is used for the proxy transport so the edge, agent, and upstream all benefit from connection reuse.

---

## 7. Implementation Checklist

Ordered by latency impact and implementation effort.

### Phase 1 — Reduces cold connect by ~50% (1-2 days)

- [ ] **TCP buffer size wiring** — edit `buffer_pool.go:88` (`applyTCPSocketOptions`) to accept `bufferSize int`; replace hardcoded `2_097_152` with the parameter. Update callers (`setTCPKeepaliveAggressive`, `setTCPListenerOptions`, future `dialTCP`/`bridgeTCP`). Verify `FREECOMPUTE_TCP_BUFFER_SIZE` takes effect.
- [ ] **DNS cache wired to proxy transport** — edit `http_proxy.go:12` (`newProxyTransport`) to accept `*DNSCache` and use `newDNSCacheDialer`. Edit `NewServer` (`server.go:101`) to create the cache and pass it. Edit `server.go:1270` (`dialViaTailscale`) to use a cache-aware dialer.
- [ ] **Transport negotiation in `/capabilities`** — add `preferredTransports` map to `handleCapabilities` JSON response. Update client-side capability detection.

### Phase 2 — Reduces per-session latency by ~30% (2-3 days)

- [ ] **Multiplexed connection at session creation** — in `handleCreateSession`/`handleCreateWebRTCSession`, open one multiplexed transport (WebSocket via `sessionMultiplexer`) immediately and hold it warm. Reuse for `/input/`, `/audio/`, `/clipboard/`, `/transfer/`.
- [ ] **`/prewarm` used by client** — add client-side prefetch of `/prewarm` on app shell load, before the first `/sessions/` request.
- [ ] **WebTransport QUIC transport shim** — add `quicFusedTransport` to `connection_fusion.go`. When `QUICAddr` is configured and the client advertises `webtransport` support, use QUIC streams instead of WebSocket frames.

### Phase 3 — Edge/QUIC infra (3-5 days)

- [ ] **QUIC listener** — add `quic-go` dependency; implement `listenQUIC` in `server.go`; serve `/proxy/`, `/capabilities`, `/healthz` over HTTP/3; serve `/wt/` over WebTransport.
- [ ] **HTTP/3 on `/proxy`** — verify BunnyCDN forwards HTTP/3; benchmark cold connect with and without QUIC.
- [ ] **0-RTT enablement** — if not behind BunnyCDN TLS termination, configure `tls.Config` for session tickets. If behind BunnyCDN, tune edge pull-zone idle timeout and verify connection pooling to the gateway origin.

### Phase 4 — Hardening and observability (ongoing)

- [ ] **UDP buffer wiring** — in `udp.go:33-35`, use `s.cfg.UDPBufferSize` for listener buffers instead of `udpSocketBufferSize`.
- [ ] **Metrics** — instrument `DNSCache` hit/miss ratio, TCP buffer sizes in use, active QUIC connections, multiplexed stream count per session.
- [ ] **BBR tradeoff monitoring** — if buffer bloat is observed on lossy mobile paths, add per-route CC algo override (`cfg.TCPCCAlgo` per route, not global).

---

## 8. Acceptance / Benchmark Criteria

### Cold connect (no warm connection, no cached DNS)

| Client path | Target | Measurement method |
|-------------|--------|-------------------|
| Browser → BunnyCDN → Gateway → Agent (HTTP proxy) | < 300 ms p50 | Navigation Timing API, `responseStart - fetchStart` |
| Browser → BunnyCDN → Gateway → Agent (WebSocket tunnel) | < 250 ms p50 | WebSocket `onopen` timestamp minus request start |
| Native → Gateway → Agent (TCP CONNECT) | < 150 ms p50 | TCP RTT measured via `TCP_INFO` (`unix.TCP_INFO`) |

### Warm reuse (prewarmed / pooled connection)

| Scenario | Target |
|----------|--------|
| `/prewarm` hold → subsequent request on same socket | < 10 ms (effectively 0-RTT at app layer) |
| Multiplexed stream reuse within a session | < 1 ms (stream open over existing transport) |
| Agent pool `take()` with idle connection | < 50 ms p95 (`liveAgentConn` via `TCP_INFO`, zero syscall on hot path) |

### Under load (steady state, 1000 concurrent sessions)

| Metric | Target |
|--------|--------|
| p95 tunnel latency (per-byte) | < 20 ms |
| Cold connect failure rate | < 1% |
| DNS cache hit ratio | > 90% |
| Proxy transport idle connection pool efficiency | > 80% (close-to-zero dial rate after warmup) |
| Multiplexed sessions reusing single transport | > 95% of new streams |

### Measurement location

- Gateway: `monitoring.Metrics` + `prometheus` endpoint (`server.go:438`).
- Client: browser `PerformanceObserver` for resource timing; native clients log `TCP_INFO` via `unix.GetsockoptTCPInfo`.

---

## 9. Risks and Tradeoffs

### BBR on all paths

- **Benefit**: Low latency, avoids CUBIC's slow start overshoot on tunnel workloads.
- **Risk**: BBR can cause buffer bloat on bandwidth-delay products over ~100 ms (e.g., inter-continental BunnyCDN edge to origin). If the queue at the bottleneck grows, latency spikes despite stable throughput.
- **Mitigation**: Keep BBR global but allow per-route override via `TCPCCAlgo` in `RouteConfig` (currently global only; see `config.go:74`). Consider `cubic` for high-BDP routes detected by `QualityTracker` (`server.go:93`).

### Hardcoded 2 MB buffers

- **Benefit**: Large `SO_SNDBUF`/`SO_RCVBUF` prevents stalls for bulk tunnel data.
- **Risk**: 2 MB per TCP connection is significant memory at scale. At 10,000 concurrent connections, that is 20 GB of socket buffer RAM.
- **Mitigation**: Wire `FREECOMPUTE_TCP_BUFFER_SIZE` as described in §3 so operators can size per deployment. Default should remain 2 MB but be tunable.

### DNS cache stale data

- **Benefit**: Eliminates repeated lookups for proxy upstreams.
- **Risk**: If the cache TTL is too long, hosts that change IP (e.g., dynamic backend) will route to dead ends.
- **Mitigation**: Default TTL 60 s (short). Expose `FREECOMPUTE_DNS_TTL_SECONDS` and `FREECOMPUTE_DNS_MAX_ENTRIES`. Watch for stale entries via `DNSCache.Refresh` called on 5xx errors.

### WebTransport / QUIC added surface area

- **Benefit**: 0-RTT, multiplexed streams, no head-of-line blocking.
- **Risk**: New dependency (`quic-go`) increases binary size and attack surface. QUIC listener needs careful resource limits (max connections, stream limits) to prevent memory exhaustion.
- **Mitigation**: Gate QUIC behind `cfg.QUICAddr` being non-empty. Set `quic.Config` limits (`MaxIncomingStreams`, `MaxIncomingUniStreams`) conservatively. Fail closed if `quic-go` fails to start.

### Connection multiplexing complexity

- **Benefit**: Fewer connections, less TLS handshake overhead, lower NAT state.
- **Risk**: A single fused transport becomes a single point of failure. A WebSocket or QUIC disruption drops all streams simultaneously.
- **Mitigation**: Keep stream framing tolerant of partial reads. On transport failure, fall back to per-feature connections (the current model) automatically. The `sessionMultiplexer` already reads frames independently per stream, so partial failure containment is built in.

---

## 10. Open Questions

1. **QUIC listener port exposure** — Should `QUICAddr` be publicly reachable, or only between BunnyCDN edge and gateway? If BunnyCDN terminates QUIC and forwards HTTP/3 to the gateway, the gateway may not need a public QUIC listener. This depends on BunnyCDN pull-zone configuration.
2. **Per-route TCP CC algo** — `TCPCCAlgo` is currently global (`SetTCPCCAlgo` in `buffer_pool.go:84`). Should we add `qos.tcpCcAlgo` per route or per destination, given that BBR behaves poorly on some high-BDP paths?
3. **DNSCache scope** — Should the cache be shared across all proxy transports, or per-route? Per-route caching prevents a large route from evicting entries for a high-frequency small route.

---

*This document is the implementation blueprint for the user. All file/line references above are exact in the current codebase and should be updated if the referenced files move.*
