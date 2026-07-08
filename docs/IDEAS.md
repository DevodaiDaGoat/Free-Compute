# FreeCompute IDEAS — Code-Ready Backlog

This is a living idea index for FreeCompute. Detailed designs live in other docs (REMOTE_ACCESS_STREAMING.md, TAILSCALE_PER_USER.md, WEBOS_CLOUD_DRIVE.md, FASTEST_PATH.md). Each idea names a real file/function to start from.

## Connection & Proxy
- **Per-route proxy auth tiers** — gate `/proxy/` access by route ownership to stop cross-tenant leaks. `Start: apps/gateway/internal/tunnel/http_proxy.go`
- **WebSocket subprotocol negotiation** — pick binary vs JSON per client in `/ws/`. `Start: apps/gateway/internal/tunnel/websocket.go`
- **Connect pre-auth handshake** — require signed token before `/connect/` TCP upgrade. `Start: apps/gateway/internal/tunnel/tcp.go`
- **SSH agent forwarding** — expose agent socket through `/ssh/` tunnel. `Start: apps/gateway/internal/tunnel/server.go:handleSSHTunnel`
- **UDP QUIC fast path** — route `/udp` over QUIC when both ends support it. `Start: apps/gateway/internal/tunnel/udp.go`
- **Agent connection fusion** — merge parallel links into one logical stream. `Start: apps/gateway/internal/tunnel/connection_fusion.go`
- **BBR buffer backpressure** — tune `buffer_pool.go` for BBR-style pacing. `Start: apps/gateway/internal/tunnel/buffer_pool.go`
- **DNS cold-start cache** — prime `dns_cache.go` from provisioning config. `Start: apps/gateway/internal/tunnel/dns_cache.go`
- **Prewarm pool autoscaler** — scale idle agent connections off demand signal. `Start: apps/gateway/internal/tunnel/server.go:handlePrewarm`

## Tailscale & Networking
- **Per-user Tailscale IPs** — allocate one TS IP per user for clean routing. `Start: apps/gateway/internal/tunnel/server.go:authHandler.AllocateIP`
- **Tailscale proxy hardening** — restrict `/tailscale/proxy` to authorized nodes only. `Start: apps/gateway/internal/tunnel/server.go:handleTailscaleProxy`
- **MagicDNS route injection** — auto-add hostnames via `dialViaTailscale`. `Start: apps/gateway/internal/tunnel/server.go:dialViaTailscale`
- **LAN peer discovery** — advertise local LAN services over Tailscale. `Start: host-agent/cmd/host-agent/tailscale.go`
- **Tailscale exit-node toggle** — let a host act as user exit node. `Start: host-agent/cmd/host-agent/tailscale.go`
- **mTLS gateway↔agent** — mutual TLS for the agent control channel. `Start: apps/gateway/internal/tunnel/agent_handler.go`

## WebOS & Per-user Storage
- **Per-user cloud drive mount** — wire `/storage/*` into WebOS home. `Start: apps/gateway/internal/storage/storage.go`
- **Storage quota UI** — show usage from `CheckStorageQuota`. `Start: apps/gateway/internal/auth/auth.go:CheckStorageQuota`
- **WebOS PWA shell** — offline-capable app shell + install. `Start: apps/frontend`
- **Drag-drop upload** — call `/storage/upload` from file manager. `Start: apps/frontend`
- **Snapshot/restore drive** — checkpoint user storage to disk. `Start: apps/gateway/internal/tunnel/checkpoint.go`
- **Temp access share links** — time-boxed `/storage/download` tokens. `Start: apps/gateway/internal/storage/storage.go`

## Gaming & Streaming
- **Safe/fast quality presets** — expose `quality.go` presets in UI. `Start: apps/gateway/internal/tunnel/quality.go`
- **WebRTC adaptive bitrate** — hook encoder to network estimate. `Start: apps/gateway/internal/webrtc/adaptive_bitrate.go`
- **Multi-channel audio** — spatial/voice tracks via `audio.go`. `Start: apps/gateway/internal/audio/audio.go`
- **Input relay low-latency** — coalesce gamepad/mouse in `input.go`. `Start: apps/gateway/internal/input/input.go`
- **Session recording export** — save recordings to cloud drive. `Start: apps/gateway/internal/tunnel/session_recording.go`
- **Edge TURN relay** — fallback relay when P2P fails. `Start: apps/gateway/internal/webrtc/webrtc.go`

## Edge / BunnyCDN Performance
- **Edge cache for static** — put WebOS assets behind BunnyCDN. `Start: apps/frontend`
- **Geo-aware prewarm** — prewarm nearest region via `geo.go`. `Start: apps/gateway/internal/tunnel/geo.go`
- **Proxy cache TTL tuning** — cache warm proxied responses. `Start: apps/gateway/internal/tunnel/proxy_cache.go`
- **Compression negotiate** — choose brotli/zstd by client. `Start: apps/gateway/internal/tunnel/compression.go`
- **WebTransport fallback** — use WebTransport when WebRTC blocked. `Start: apps/gateway/internal/webrtc/webtransport.go`

## Security & Auth
- **Rate limiting per route** — throttle `/proxy` and `/connect`. `Start: apps/gateway/internal/firewall/firewall.go`
- **Anomaly detection alerts** — surface `detector.go` findings to admin. `Start: apps/gateway/internal/security/detector.go`
- **Audit log streaming** — pipe session audit to SIEM. `Start: apps/gateway/internal/session/audit.go`
- **Short-lived access tokens** — temp links for guest sessions. `Start: apps/gateway/internal/auth/auth.go`
- **Config hot-reload** — reload `config.go` without restart. `Start: apps/gateway/internal/tunnel/config.go`

## Ops & Observability
- **Health dashboard UI** — visualize `health.go`. `Start: apps/gateway/internal/monitoring/health.go`
- **Prometheus metrics export** — expose `metrics.go` at `/metrics`. `Start: apps/gateway/internal/monitoring/metrics.go`
- **Speed-check scheduler** — periodic path benchmarks. `Start: apps/gateway/internal/tunnel/speedcheck.go`
- **Admin console actions** — operator controls in `admin.go`. `Start: apps/gateway/internal/admin/admin.go`
