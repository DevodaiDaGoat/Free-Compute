# Rate Limiting & Quotas

Prevent abuse of gateway proxy/tunnel/signaling surfaces, cap per-user consumption, and surface metrics for throttling decisions. Current state: only coarse storage quota + naive in-memory `UsageTracker`.

## Current Coverage

| Scope | Mechanism | File |
|-------|-----------|------|
| Storage quota per user | `AuthManager.CheckStorageQuota` (10 GB default) | `apps/gateway/internal/auth/auth.go:267` |
| Concurrent WebRTC sessions | `sessionLimiter := make(chan struct{}, maxConcurrentSessions)` | `apps/gateway/internal/webrtc/webrtc.go:167` |
| Resource quotas (CPU/RAM/GPU/Network/Sessions) | `usage.Tracker` in-memory, but not enforced in handlers | `apps/gateway/internal/usage/usage.go:191` `CheckQuota` |
| Proxy transport tuning | `MaxIdleConns`, `MaxIdleConnsPerHost`, idle/read timeouts | `apps/gateway/internal/tunnel/config.go:135` |

**Gaps:** no per-IP or per-user request rate limits on proxy/tunnel/signaling, no bandwidth caps, no enforcement of current quota limits at request time, and no metrics for throttling.

## Rate Limit Tiers

| Tier | Limit | Scope |
|------|-------|-------|
| Unauthenticated health/capabilities/cdn | 100 req/s per IP | Global |
| Auth endpoints (`/auth/*`) | 10 req/s per IP per method | Per-IP |
| UI/API proxy (`/proxy/*`, `/connect/*`) | 50 req/s per user, burst 100 | Per-user |
| Tunnel/signaling (`/ws/*`, `/signal/*`, `/webrtc/*`) | 20 req/s per user, burst 40 | Per-user |
| Host-agent (`/agent/*`) | 100 req/s per IP | Per-IP |

## Token-Bucket Implementation

Add `internal/ratelimit/tokenbucket.go`:

```
TokenBucket struct:
    tokens       chan struct{}
    rate         float64    // tokens/sec
    burst        int
    mu           sync.Mutex
    lastTick     time.Time
```

Usage:
```go
func (b *TokenBucket) Allow() bool { ... }
func (b *TokenBucket) Wait(ctx context.Context) error { ... }
```

Map key per request: `remoteAddr` (unauth) or `userId` (auth). Clean up stale entries with a background sweep (evict after 1 min idle).

## Pluggable Middleware via `withCommonHeaders`

`withCommonHeaders` (`apps/gateway/internal/tunnel/server.go:416`) wraps the entire mux. Insert rate limiting *after* CORS but *before* route handlers:

```go
handler := withCommonHeaders(rateLimitMiddleware(s.compressMiddleware.Handler(mux)), cfg)
```

Per-route throttle selectors:

- `/proxy/*`, `/connect/*` → user + IP tier.
- `/ws/*`, `/signal/*` → user tier (signaling is session-bound, high frequency).
- `/webrtc/*` → user tier.
- `/agent/*` → IP tier (agents authenticate via mTLS/token; limit by agent IP).
- `/auth/login`, `/auth/register` → IP tier only.

## Bandwidth Caps

Encode in `Config` or `RouteConfig`:

```go
type RouteConfig struct {
    ...
    MaxDownlinkBps uint64 `json:"maxDownlinkBps,omitempty"`
    MaxUplinkBps   uint64 `json:"maxUplinkBps,omitempty"`
}
```

Enforcement points:
- TCP bridging `bridge()` (`host-agent/cmd/host-agent/main.go:318`) — count bytes in `copyConn` and deadline-shape.
- HTTP proxy transport — set `RoundTripper` with `http.MaxBytesHandler` or a `Trickle` response writer wrapping `ResponseWriter`.
- WebRTC `AdaptiveBitrate` already handles session bitrate (`apps/gateway/internal/webrtc/adaptive_bitrate.go:144`); unify `maxDownlinkBps` with the bitrate estimator.

## Concurrent Session / Tunnel Caps

- WebRTC sessions: hard cap at `maxConcurrentSessions` (channel semaphore) — already exists in `webrtc.go:167`.
- Agent tunnels: `agentPool` in `internal/tunnel/` tracks active connections; add a configurable `MaxAgentTunnels` (default 200).
- Gaming sessions: `GamingManager` (`apps/gateway/internal/gaming/gaming.go`) should enforce `maxConcurrentStreams` from `HostAllocator`.

## Quota Enforcement at Request Time

`usage.Tracker.CheckQuota` (`apps/gateway/internal/usage/usage.go:191`) is HTTP-safe but never called from handlers. Plug in:
- `HandleQuota` already exposes `GET/PUT`. Add a `CheckQuota` gate in compression middleware or route handler for every proxy request.
- On quota exceeded, return `999/Vendor: X-RateLimit-Remaining: 0` or HTTP `429` with JSON `{"error":"quota exceeded","resource":"network_gb"}`.

## Metrics for Throttling

Dashboard reads (Prometheus format):
- `freecompute_rate_limit_hits_total{route="/proxy/..."}`
- `freecompute_rate_limit_active{key="/proxy/..."}`
- `freecompute_quota_remaining{user="..."}` 
- `freecompute_bandwidth_bytes_total{route=..., direction=...}`

Emit via `monitoring.Metrics` in the rate-limit middleware and quota gate.

## Implementation Steps

| Step | Function / File | Action |
|------|-----------------|--------|
| 1 | Add `internal/ratelimit/tokenbucket.go` | Define `TokenBucket` with `Allow()` and `Wait()`. Use `time.Now()` + delta accumulation. |
| 2 | Extend `Config` / `RouteConfig` in `tunnel/config.go` | Add per-route `MaxDownlinkBps`, `MaxUplinkBps`, and rate-limit tiers. |
| 3 | Wire `rateLimitMiddleware` into `tunnel/server.go` | In `NewServer`, wrap mux via `withCommonHeaders`. Use `remoteAddr` from `r.RemoteAddr` + `X-Forwarded-For` under `CDNHostname`. |
| 4 | Enforce quotas in proxy handler | Call `usageTracker.CheckQuota(userID, ResourceNetwork, 1)` before proxying each request; short-circuit to `429`. |
| 5 | Add bandwidth meter in `bridge()` / `copyConn` | Count bytes copied per tunnel connection; close / throttle if cap exceeded. |
| 6 | Add `MaxAgentTunnels` guard | Wrap `handleAgentTunnel` with an in-flight counter in `apps/gateway/internal/tunnel/server.go`. |
| 7 | Add throttling metrics | Increment counters in rate-limit middleware; expose via existing `metricsHandler`. |

## Acceptance Criteria

- `/auth/login` and `/auth/register` return `429` after >10 req/s/IP for 5 seconds.
- `/proxy/*` returns `429` when a user exceeds 50 req/s or `network_gb` quota.
- `/agent/*` returns `503` when active agent tunnels exceed configured cap.
- WebRTC sessions hard-out at `maxConcurrentSessions` (existing behavior, verified by unit test).
- Prometheus scrapes (`GET /metrics`) show `freecompute_rate_limit_hits_total` and `freecompute_quota_remaining`.
- BunnyCDN proxy ingress respects `X-RateLimit-Remaining` and `Retry-After` headers.
