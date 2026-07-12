# Roadmap

## Phase 1 — Architecture ✅

Define system architecture, API contracts, and data models.

- Remote session, gaming mode, host capability, and proxy route contracts.
- Initial tunnel gateway for HTTP(S), WebSocket, TCP, UDP, SSH-over-WebSocket, HTTP CONNECT, WebRTC/P2P signaling.
- Database schema for users, VMs, hosts, queue, sessions, proxy routes.

## Phase 2 — Implementation ✅

Build core services and the WebOS frontend.

- **Gateway**: Full tunnel proxy, WebRTC pipeline, session management, auth, storage, gaming, input, audio, transfer, security, admin dashboard, Tailscale integration.
- **Host Agent**: Reverse tunnel registration, QEMU VM lifecycle, Tailscale discovery, host capability reporting.
- **WebRTC**: Safe Mode and Fast Mode presets with H.264/H.265/VP8/VP9/AV1/Opus/AAC codecs.
- **Gaming Session**: Scheduler scoring for latency, GPU, encoder, load.
- **Browser Terminal**: SSH-over-WebSocket tunnel integration.
- **File Service**: Dedicated Go HTTP service with local and S3 backends.
- **Scheduler**: Priority-queue based scheduling with host ranking and resource allocation.
- **Monitoring**: Prometheus metrics endpoint, component health checking, system stats collector.
- **VM Images**: Image catalog with seeded defaults (Ubuntu, Debian, Windows), custom image creation, VM snapshots.
- **SSH Keys**: SSH public key management with fingerprint validation.
- **Firewall**: Firewall rules engine with security groups and VM assignment.
- **Usage Tracking**: Resource usage tracking, quota enforcement, invoice generation.

## Phase 3 — Performance Optimization ✅

High-ROI backend optimizations from codebase audit (see [PERFORMANCE_OPTIMIZATION.md](./PERFORMANCE_OPTIMIZATION.md)):

- ✅ Buffer pool adoption — `sync.Pool` for TCP/WebSocket buffers
- ✅ Audio ring buffer rewrite — bulk `copy()`, 50-100x throughput
- ✅ Priority queue → binary heap for O(log n) scheduling
- ✅ Agent pool O(1) removal via `container/list` + map
- ✅ UDP client expiration — idle timeout + periodic sweep
- ✅ WebSocket frame pooling for small frames
- ✅ Host allocator pre-indexed by region/resource-class
- ✅ Signal store context sweep for clean shutdown
- ✅ AudioBuffer deadlock fix
- ✅ Rate limiter race condition fixed (per-bucket mutex)
- ✅ dialViaTailscale connection leak fixed
- ✅ WriteVideoRTP/WriteAudioRTP write-lock eliminated (RLock + atomic)
- ✅ bridgeWebSocket goroutine drain delay fixed (close conns on first error)

## Phase 3b — Security Hardening ✅ (July 2026)

Hardening pass against open-source threat model:

- ✅ CORS origin allowlist — no wildcard + credentials, reflected origin validated
- ✅ `/hosts/register` + `/hosts/metrics` — require tunnel token (was unauthenticated)
- ✅ Usage IDOR — userId forced from JWT, not query param
- ✅ User enumeration — same 401 for wrong email and wrong password
- ✅ Error leakage — internal errors return generic messages
- ✅ Password minimum length (8 chars) + email format validation
- ✅ `UpdateHostLoad` — idle hosts no longer marked offline
- ✅ `GenerateInvoice` — no longer panics on short userID

## Phase 3c — WebOS Desktop Overhaul ✅ (July 2026)

Full WebOS redesign and new apps:

- ✅ macOS-style desktop: translucent menu bar, floating dock with magnification
- ✅ Alt-Tab app switcher (overlay, cycles on key release)
- ✅ Spotlight launcher (Ctrl+Space, searchable grid)
- ✅ Window manager: traffic lights, drag, resize, maximize, minimize, focus glow
- ✅ Host Monitor app — live CPU/RAM/disk bars per host, active sessions
- ✅ AI Log Monitor — heuristic threat detection (crypto-miner, bruteforce, flood)
- ✅ Credits & Billing — buy plans, crypto deposit addresses (BTC/ETH/SOL/USDC)
- ✅ Remote Desktop — stream tab + SSH terminal + Tailscale mesh info
- ✅ App Player — install/run/remove packages with terminal console
- ✅ Multiplexed binary tunnel hook — single WebSocket, 7 typed channels
- ✅ CORS fixed (was blocking all API calls in browser)
- ✅ Stale `.next` cache handling documented

## Phase 4 — Connection Quality (In Progress)

Low-latency streaming enhancements (see [CONNECTION_QUALITY.md](./CONNECTION_QUALITY.md)):

- ✅ GCC-style bandwidth estimation in bandwidth.go (loss + jitter tracking)
- ✅ Adaptive codec switching in WebRTC bandwidth estimator
- 🔲 WebTransport / QUIC — wired at `/wt/` but needs `quic-go` server started
- 🔲 Multi-channel WebRTC data layer (7 channels with priority)
- 🔲 Adaptive jitter buffer
- 🔲 Connection prefetch (pre-warm before user clicks Connect)

## Phase 5 — New Features (Planned)

Product features (see [NEW_FEATURES.md](./NEW_FEATURES.md)):

- 🔲 Cloud Save & Session Persistence — persistent home directories via NFS/S3FS
- 🔲 Collaborative Sessions — multi-pointer, multi-cursor, role-based
- 🔲 Steam integration / game launcher
- 🔲 Connection Quality Dashboard — real-time and historical charts
- 🔲 Public Embed API — REST + WebSocket SDK for third-party embedding
- 🔲 Headless Mode — API-only deployment, no WebOS required
- 🔲 Real payment processing (Stripe + crypto on-chain confirmation)

## Phase 6 — Beta

Public beta with community onboarding.

- 🔲 Persistent game storage, cloud saves, snapshots
- 🔲 LAN tunneling and launcher integrations
- 🔲 Horizontal scaling and multi-region deployment
- 🔲 Mobile PWA (see [MOBILE_PWA.md](./MOBILE_PWA.md))
- 🔲 WebOS app store with signed packages

## Real Dependencies Needed to Go Live

| Feature | Dependency |
|---------|-----------|
| VM launch | `qemu-system-x86_64` in PATH |
| Screen capture / encoding | `ffmpeg` in PATH |
| Peer-to-peer mesh | `tailscale` installed + auth |
| Payments | `STRIPE_SECRET_KEY` env |
| LLM moderation | `FREECOMPUTE_MODERATION_LLM_URL` env |
| HTTPS / edge | Cloudflare Tunnel or reverse proxy with TLS |

## Resource Allocation

VMs default to **50% of the host machine's resources**, configurable per host:

```bash
FREECOMPUTE_RESOURCE_PCT=50   # percentage to allocate (default)
# or set manually:
FREECOMPUTE_VM_CPUCORES=6
FREECOMPUTE_VM_RAMGB=4
FREECOMPUTE_VM_STORAGEGB=344
```

Current dev machine (50%): **6 CPUs, 4 GB RAM, 344 GB disk**
