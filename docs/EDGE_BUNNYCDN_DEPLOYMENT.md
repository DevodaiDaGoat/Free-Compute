# BunnyCDN Edge Deployment

## Goal

Deploy FreeCompute behind BunnyCDN to minimize ingress latency for browser clients, WebOS desktop sessions, and native clients. BunnyCDN should serve as the global edge entry point for static assets, API/auth, WebSocket signaling, and relay discovery. Real-time media, tunnel paths, and control channels must bypass the CDN or traverse it with zero buffering so that interactive latency is not degraded.

## Topology

```
Client (browser / WebOS / native)
        |
        | HTTPS / WSS / HTTP/3
        v
  BunnyCDN edge
        |
        |--- Static assets (immutable WebOS shell, JS/CSS bundles)
        |--- API + auth (login, register, capabilities)
        |--- WebSocket signaling (/signal, /ws)
        |--- Relay discovery documents
        |
        |--- [bypass cache / preserve upgrade] ---
        |       /proxy/*, /connect/*, /agent/*, /webrtc/*, /sessions/*
        v
  FreeCompute gateway (origin)
        |
        | signed host control channel
        v
  Host agent / VM / desktop process
        |
        | WebRTC UDP (direct P2P preferred) or TURN relay
        v
  Media + input + clipboard back to client
```

## Hostname Plan

| Purpose | Hostname | Env var | CDN product |
|---------|----------|---------|-------------|
| WebOS shell + static assets | `cdn.freecompute.io` | `FREECOMPUTE_CDN_HOSTNAME` | BunnyCDN Pull Zone (file storage, aggressive cache) |
| Edge / API / signaling ingress | `edge.freecompute.io` | `FREECOMPUTE_EDGE_HOSTNAME` | BunnyCDN Edge (proxy/acceleration, WebSocket allowed) |
| Auth + REST API | `api.freecompute.io` | `FREECOMPUTE_API_HOSTNAME` | BunnyCDN Edge or direct origin (short cache, strict auth) |
| Relay discovery | served under `edge` or `cdn` path | derived from above | Edge path with Bypass Cache |

The gateway advertises these values in `handleCapabilities` (`apps/gateway/internal/tunnel/server.go:532-549`) inside the `bunnyCdn` block. Clients use `CDNHostname` for static asset origins, `EdgeHostname` for signaling/relay, and `APIHostname` for auth. Do not hardcode hostnames in the frontend; read them from `/capabilities`.

## Cache Rules

### Aggressive caching

- Immutable WebOS assets (JS/CSS bundles with content hashes): `Cache-Control: public, max-age=31536000, immutable`
- WebOS app shell icons and splash images: `public, max-age=86400, immutable`
- Relay discovery documents (candidate updates): `public, max-age=60, stale-while-revalidate=30`

These are served from the BunnyCDN Pull Zone (`cdn.freecompute.io`). Set BunnyCDN origin rules to cache these paths without query strings.

### Bypass / no-store

Every tunnel or control path must bypass CDN buffering and never be cached by BunnyCDN or intermediate proxies. Set `Cache-Control: no-store` on the following routes, matching the `bypassCache` list already advertised by `handleCapabilities`:

- `/proxy/*`
- `/ws/*`
- `/connect/*`
- `/agent/*`
- `/signal/*`
- `/webrtc/*`
- `/sessions/*`

In BunnyCDN configuration, create "Cache Bypass" rules for these prefixes. Also disable BunnyCDN's request/response buffering on these paths so latency-sensitive frames are not held.

### WebSocket upgrades

BunnyCDN supports WebSocket for Edge and Accelerator products. Configure the edge to preserve the `Connection: Upgrade` and `Upgrade: websocket` headers. Do not let the CDN compress or buffer WebSocket frames. The gateway already returns `withCommonHeaders` (`apps/gateway/internal/tunnel/server.go:416-436`) with CORS headers when `CDNHostname` is set; keep this intact.

### Prewarm

`handlePrewarm` (`apps/gateway/internal/tunnel/server.go:444-453`) returns `Cache-Control: no-store` and a small warm JSON body. BunnyCDN should not cache this endpoint; it is intended for edge-to-origin keepalive only.

## TLS, 0-RTT, and HTTP/3

BunnyCDN terminates TLS at the edge. For repeat clients, enable 0-RTT on the BunnyCDN origin where supported so the TLS handshake completes in one RTT. BunnyCDN also offers HTTP/3 (QUIC) on compatible plans; enable it for the `edge.freecompute.io` hostname.

Origin-to-edge TLS should use valid certificates (Let's Encrypt or BunnyCDN managed certs). If the gateway is exposed directly (not recommended), enable `FREECOMPUTE_GATEWAY_QUIC_ADDR` (default `:8084`) for QUIC fallback; see `config.go:141`.

## Tunneling Through CDN

### HTTP CONNECT / WebSocket / tunnel paths

- **Browser clients** access `/proxy/{routeID}/*`, `/ws/{routeID}`, `/connect/{routeID}` through `edge.freecompute.io`.
- BunnyCDN forwards these to the gateway origin without buffering.
- The gateway's `handleConnect` and `handleWebSocketTunnel` (`apps/gateway/internal/tunnel/server.go`) then forward to the host agent or target.
- For SSH-over-WebSocket (`/ssh/{routeID}`), BunnyCDN must pass the upgrade headers through to the gateway.

### Direct P2P vs edge relay / TURN

WebRTC media should prefer direct UDP between client and host. When direct paths fail (symmetric NAT, firewall), fall back to:

1. **Edge relay (TURN)** — traffic traverses BunnyCDN edge + gateway relay mesh.
2. **Host-agent relay** — host agent forwards via TCP if the gateway is reachable.

The gateway advertises `routeModes` (`edge-relay`, `direct-p2p`, `host-tunnel`) in `/capabilities`. Clients pick the mode based on ICE candidate results. The `webrtc` package in `apps/gateway/internal/webrtc/` manages relay mesh peers via `RelayMesh` and `MeshPeers` config.

## Gateway Config Checklist

Set these environment variables in production before starting the gateway. They are already wired in `config.go` and `run-backend.sh`; the task is to assign real values.

| Variable | Purpose | Production value example |
|----------|---------|--------------------------|
| `FREECOMPUTE_GATEWAY_ADDR` | Gateway listen address | `:8080` (internal only; do not expose directly) |
| `FREECOMPUTE_TUNNEL_TOKEN` | Shared tunnel auth token | strong random string (64+ chars) |
| `FREECOMPUTE_CDN_HOSTNAME` | Pull Zone hostname | `cdn.freecompute.io` |
| `FREECOMPUTE_EDGE_HOSTNAME` | Edge hostname | `edge.freecompute.io` |
| `FREECOMPUTE_API_HOSTNAME` | API hostname | `api.freecompute.io` |
| `FREECOMPUTE_GATEWAY_SHUTDOWN_SECONDS` | Graceful shutdown | `5` |
| `FREECOMPUTE_TUNNEL_DIAL_SECONDS` | Dial timeout | `3` |
| `FREECOMPUTE_TUNNEL_AGENT_WAIT_SECONDS` | Agent wait timeout | `5` |
| `FREECOMPUTE_GATEWAY_READ_HEADER_SECONDS` | Read header timeout | `3` |
| `FREECOMPUTE_GATEWAY_IDLE_SECONDS` | HTTP idle timeout | `60` |
| `FREECOMPUTE_PROXY_UPSTREAM_IDLE_SECONDS` | Upstream idle timeout | `30` |
| `FREECOMPUTE_PROXY_RESPONSE_HEADER_SECONDS` | Response header timeout | `5` |
| `FREECOMPUTE_PROXY_EXPECT_CONTINUE_SECONDS` | Expect continue timeout | `1` |
| `FREECOMPUTE_PROXY_MAX_IDLE_CONNS` | Max idle conns | `2048` |
| `FREECOMPUTE_PROXY_MAX_IDLE_CONNS_PER_HOST` | Max idle conns per host | `256` |
| `FREECOMPUTE_PROXY_FLUSH_MS` | Proxy flush interval | `-1` (streaming) |
| `FREECOMPUTE_TCP_CC_ALGO` | TCP congestion control | `auto` or `bbr` |
| `FREECOMPUTE_TCP_BUFFER_SIZE` | TCP buffer size | `2097152` |
| `FREECOMPUTE_UDP_BUFFER_SIZE` | UDP buffer size | `4194304` |
| `FREECOMPUTE_MESH_PEERS` | Relay mesh peers (comma-separated) | internal relay IPs or empty |
| `FREECOMPUTE_QUALITY_*` | WebRTC quality thresholds | tune per region |
| `FREECOMPUTE_GATEWAY_QUIC_ADDR` | QUIC listener (if exposed) | `:8084` |
| `FREECOMPUTE_ENABLE_COMPRESSION` | Enable response compression | `true` |
| `FREECOMPUTE_DISABLE_PROXY_CACHE` | Disable in-memory proxy cache | `true` in edge production |
| `FREECOMPUTE_PROXY_CACHE_MAX_SIZE_MB` | Max proxy cache size | `256` (local only) |

## Implementation Steps

### 1. Gateway headers and capabilities (`apps/gateway/internal/tunnel/server.go`)

- `withCommonHeaders` already sets CORS headers when `CDNHostname` is non-empty. Ensure the BunnyCDN edge hostname is set so CORS is enabled for cross-origin browser/WebOS clients (`server.go:422-427`).
- `handleCapabilities` already advertises `cdnHostname`, `edgeHost`, `apiHost`, `cacheable`, and `bypassCache` lists (`server.go:532-549`). Verify these match your BunnyCDN topology.
- `handlePrewarm` sends `Cache-Control: no-store` (`server.go:447`). This is correct for edge keepalive.

### 2. Config wiring (`apps/gateway/internal/tunnel/config.go`)

- `Config` struct already has `CDNHostname`, `EdgeHostname`, `APIHostname` fields (`config.go:67-69`).
- `LoadConfigFromEnv` reads `FREECOMPUTE_CDN_HOSTNAME`, `FREECOMPUTE_EDGE_HOSTNAME`, `FREECOMPUTE_API_HOSTNAME` (`config.go:138-140`). No code changes required; populate these env vars.
- Verify `EnableProxyCache` and `MaxProxyCacheMB` are set appropriately when running behind BunnyCDN. BunnyCDN should handle static asset caching; the gateway's local proxy cache is secondary.

### 3. Startup scripts (`run-backend.sh`, `start-website.sh`)

- Both scripts set the BunnyCDN hostnames with defaults: `cdn.freecompute.io`, `edge.freecompute.io`, `api.freecompute.io` (`run-backend.sh:51-53`, `start-website.sh:39-41`).
- Override them in production:

  ```bash
  export FREECOMPUTE_CDN_HOSTNAME="cdn.freecompute.io"
  export FREECOMPUTE_EDGE_HOSTNAME="edge.freecompute.io"
  export FREECOMPUTE_API_HOSTNAME="api.freecompute.io"
  ```

- `run-backend.sh` already contains the note "BunnyCDN config: override with actual values in production" (`run-backend.sh:50`). Replace the defaults with real BunnyCDN hostnames and ensure the gateway is only reachable from the CDN origin or private network.

### 4. BunnyCDN configuration

- Create a Pull Zone for `cdn.freecompute.io` pointing to the gateway origin. Enable aggressive caching for `/`, `/webos/*`, `/static/*`, `/assets/*`.
- Create an Accelerator / Edge hostname for `edge.freecompute.io` pointing to the same origin. Enable WebSocket support.
- Add Cache Bypass rules for: `/proxy/*`, `/ws/*`, `/connect/*`, `/agent/*`, `/signal/*`, `/webrtc/*`, `/sessions/*`.
- Enable HTTP/3 and 0-RTT on the edge hostname if your BunnyCDN plan supports it.
- Ensure `Upgrade: websocket` and `Connection: Upgrade` are preserved (BunnyCDN does this by default for WebSocket-enabled zones).

## Acceptance Criteria

1. **Static assets served from BunnyCDN edge**: WebOS shell and bundles load from `cdn.freecompute.io` with `Cache-Control: public, max-age=31536000, immutable` and `age` header > 0.
2. **Tunnel paths bypass cache**: Requests to `/proxy/*`, `/ws/*`, `/connect/*`, `/agent/*`, `/signal/*`, `/webrtc/*`, `/sessions/*` return `Cache-Control: no-store` and are never served from BunnyCDN cache.
3. **WebSocket upgrades preserved**: `/ws/{routeID}` and `/signal/{routeID}/rooms/{roomID}` upgrade successfully through BunnyCDN with no 502/504.
4. **API + auth reachable**: `/auth/login`, `/auth/register`, `/capabilities`, `/healthz` respond through `api.freecompute.io` or `edge.freecompute.io` with correct CORS headers.
5. **Edge latency measured**: `curl -o /dev/null -w "%%{time_total}" https://edge.freecompute.io/healthz` completes in < 100 ms from major regions.
6. **WebRTC session creation**: `POST /webrtc/` through the edge returns `201` with valid session JSON and SDP candidates.
7. **No reconnect storms**: Under sustained load, client reconnect rate stays below 1% and cache bypass rules prevent stale cached redirects.

## Risks

| Risk | Mitigation |
|------|-----------|
| BunnyCDN WebSocket timeout or buffer limits | Disable CDN buffering on tunnel paths; keep-alive via `/prewarm`; tune `FREECOMPUTE_GATEWAY_IDLE_SECONDS` |
| Cached stale API responses | Ensure `bypassCache` includes all auth/control paths; set `Cache-Control: no-store` on `/auth/*`, `/routes`, `/sessions/*` |
| CDN origin sees only proxy IPs | Use BunnyCDN real IP header (`X-BunnyCDN-Real-IP`) or enable CDN geo headers if gateway needs client IP |
| 0-RTT replay attacks | BunnyCDN handles TLS 0-RTT at the edge; origin does not need 0-RTT because tunnel paths are authenticated by `FREECOMPUTE_TUNNEL_TOKEN` |
| Reconnect storms if tunnel paths cached | Strict `no-store` on bypass list; monitor CDN cache hit ratio on `/proxy` and `/ws` paths |
| Edge hostname DNS misconfiguration | Verify CNAMEs point to BunnyCDN; validate with `dig` and `curl -I` before production cutover |

## Reference

- `apps/gateway/internal/tunnel/server.go` — `handleCapabilities` (`cdn` block + `bypassCache`), `withCommonHeaders`, `handlePrewarm`
- `apps/gateway/internal/tunnel/config.go` — `FREECOMPUTE_CDN_HOSTNAME`, `FREECOMPUTE_EDGE_HOSTNAME`, `FREECOMPUTE_API_HOSTNAME` env wiring
- `run-backend.sh` / `start-website.sh` — hostname defaults and tunnel route configs
- `docs/REMOTE_ACCESS_STREAMING.md` — "BunnyCDN-Oriented Deployment" section
- `docs/EDGE_PERFORMANCE.md` — edge caching strategy and bypass rules
- `docs/NETWORK_ACCELERATION.md` — QUIC / 0-RTT / DSCP details
