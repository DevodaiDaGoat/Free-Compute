# FreeCompute — TODO

> See `todo.txt` in the repo root for the quick-reference change index.
> This file tracks detailed work items.

## Done (July 2026)

### Backend
- [x] Gateway: JWT auth, register, login, preferences, roles (user/moderator/admin)
- [x] Gateway: SQLite DB with WAL, idempotent migrations, indexes
- [x] Gateway: Session manager, host allocator, Galaxy scheduler
- [x] Gateway: WebRTC pipeline — H.264/H.265/VP8/VP9/AV1, adaptive bitrate (GCC)
- [x] Gateway: Multiplexed binary WebSocket tunnel (typed channels, ping/pong RTT)
- [x] Gateway: Security detector — crypto-miner signatures, traffic anomaly, AI moderation flag
- [x] Gateway: Report system (POST /reports, moderator action, conditional LLM triage)
- [x] Gateway: Storage with per-user quota, path traversal protection
- [x] Gateway: Usage tracking, quota enforcement, invoice generation
- [x] Gateway: SSH key management, VM image catalog, firewall rules engine
- [x] Gateway: Rate limiter (token bucket per IP + per user), connection limit per user
- [x] Gateway: Prometheus metrics, health checker, DNS cache, proxy cache
- [x] Host Agent: Reverse tunnel client, capability reporting, reconnect with backoff
- [x] Host Agent: VM-setup agent — env-driven config, --dry-run, --self-test, 50% resource default
- [x] Security hardening:
  - CORS origin allowlist (no wildcard + credentials)
  - /hosts/register + /hosts/metrics require tunnel token (was public)
  - Usage IDOR fixed (userId forced from JWT, not query param)
  - User enumeration fixed (same 401 for wrong email/password)
  - Error messages no longer leak internal paths/stack traces
  - Password minimum length enforced (8 chars)
  - Email format validation on register

### Frontend / WebOS
- [x] Landing page (macOS-style dark, features, architecture, contribute CTA)
- [x] Connection Space (/connect) — session manager, WebRTC stream viewer
- [x] WebOS boot sequence (BIOS POST → loading → login → desktop)
- [x] macOS-style desktop: menu bar, floating dock with magnification, desktop grid
- [x] Alt-Tab switcher (overlay, cycles on key release)
- [x] Spotlight launcher (Ctrl+Space, searchable)
- [x] Window manager: traffic lights, drag, resize, maximize, minimize
- [x] Apps: Terminal, Browser, Files, Settings, Calculator, Admin Panel, Task Manager
- [x] Apps: Remote Desktop (stream + SSH terminal + Tailscale tab)
- [x] Apps: App Player (install/run/remove packages, terminal console)
- [x] Apps: Host Monitor (live CPU/RAM/disk per host, active sessions)
- [x] Apps: Credits & Billing (buy credits, crypto deposits, earn free, history)
- [x] Apps: AI Log Monitor (real-time heuristic threat analysis, GPU detection)
- [x] Security headers (CSP, HSTS, CORP, COOP)
- [x] CORS fixed (was blocking all API calls)

## In Progress

- [ ] Phase 4 — Connection quality improvements (docs/CONNECTION_QUALITY.md)
  - WebTransport / QUIC (needs quic-go wired to /wt/ endpoint)
  - Adaptive jitter buffer
  - Multi-channel WebRTC data layer

## Needs Real Dependencies (install on host)

- [ ] QEMU VM launch — needs `qemu-system-x86_64` in PATH
- [ ] FFmpeg encoding — needs `ffmpeg` in PATH
- [ ] Tailscale mesh — needs `tailscale` installed and logged in
- [ ] Stripe payments — needs `STRIPE_SECRET_KEY`
- [ ] LLM moderation — needs `FREECOMPUTE_MODERATION_LLM_URL`

## Planned (Phase 5+)

- [ ] Cloud saves / persistent home directories (NFS or S3FS)
- [ ] Collaborative sessions (multi-cursor, role-based)
- [ ] Steam integration / game launcher
- [ ] Horizontal scaling (multiple gateway replicas)
- [ ] Public embed API (REST + WebSocket SDK)
- [ ] Global CDN + multi-region (BunnyCDN + edge workers)
- [ ] Real Stripe billing integration
- [ ] Mobile PWA (docs/MOBILE_PWA.md)
- [ ] WebOS app store with signed packages

## Bug Tracker

| ID | Status | Description |
|----|--------|-------------|
| BUG-01 | Fixed | BandwidthEstimator type collision (adaptive_bitrate.go vs bandwidth.go) |
| BUG-02 | Fixed | handleHostMetrics passed 0 instead of activeStreams |
| BUG-03 | Fixed | RouteRegistry interface returned Route value (should be *Route) |
| BUG-04 | Fixed | UpdateHostLoad marked idle hosts offline (zero load = offline) |
| BUG-05 | Fixed | bridgeWebSocket goroutine drain waited IdleTimeout on error |
| BUG-06 | Fixed | GenerateInvoice panic on short userID (slice bounds) |
| BUG-07 | Fixed | storage DeleteFile never decremented usage |
| BUG-08 | Fixed | usage Tracker.Track memory leak (never pruned old records) |
| BUG-09 | Fixed | storage WriteFile appended duplicate FileInfo on overwrite |
| BUG-10 | Fixed | Rate limiter bucket mutated without mutex (race condition) |
| BUG-11 | Fixed | dialViaTailscale leaked N-1 racing connections |
| BUG-12 | Fixed | WriteVideoRTP/WriteAudioRTP held write-lock per RTP packet |
| BUG-13 | Fixed | CORS blocked all API calls (missing Allow-Origin header) |
| BUG-14 | Fixed | page.tsx added duplicate nav bar over Desktop menu bar |
| BUG-15 | Fixed | /hosts/register and /hosts/metrics were unauthenticated |
| BUG-16 | Fixed | Usage IDOR — any user could query other user's usage data |
| BUG-17 | Fixed | User enumeration — different errors for wrong email vs wrong password |
| BUG-18 | Fixed | openApp in Desktop captured stale windows state (moved to setWindows updater) |
