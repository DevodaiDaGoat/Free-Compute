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

## Phase 3 — Performance Optimization

High-ROI backend optimizations from codebase audit (see [PERFORMANCE_OPTIMIZATION.md](./PERFORMANCE_OPTIMIZATION.md)):

- **Buffer pool adoption**: Route all 32 KB TCP/WebSocket buffers through `sync.Pool` — eliminates ~75% of per-connection allocations.
- **Audio ring buffer rewrite**: Byte-by-byte copy → bulk `copy()` with two-stage wrap — 50-100x audio throughput improvement.
- **Priority queue → binary heap**: Replace O(n) slice insertion with `container/heap` for scheduler queue — critical at scale.
- **Agent pool O(1) removal**: Replace slice scans with `container/list` + map for idle connection management.
- **UDP client expiration**: Per-client idle timeout + periodic sweep — fixes memory leak from abandoned UDP sessions.
- **Stats collector parsing**: Parse `webrtc.StatsReport` to populate real metrics (RTT, jitter, packet loss, bitrate).
- **Fix dual HostAllocator bug**: Scheduler and SessionManager currently create separate allocator instances — hosts registered with one are invisible to the other.
- **WebSocket frame pooling**: Reuse frame buffers within connections instead of per-frame allocation.
- **Host allocator indexing**: Pre-index hosts by region/resource-class to avoid full-map scan on every allocation.

## Phase 4 — Connection Quality

Low-latency streaming enhancements (see [CONNECTION_QUALITY.md](./CONNECTION_QUALITY.md)):

- **WebRTC Bandwidth Estimation**: Real-time adaptive bitrate using GCC hybrid algorithm (loss-based + delay-based). Static bitrate → dynamic 100 Kbps–50 Mbps range.
- **Adaptive Codec Switching**: Runtime codec switching based on network conditions. H.265 at low jitter → VP8 at high jitter.
- **WebTransport / QUIC**: HTTP/3 datagram support for gamepad/mouse/telemetry — 30-50% lower p50 latency vs WebSocket. QUIC connection migration survives network changes without ICE restart.
- **Edge Relay Optimization**: Dynamic relay selection with STUN-like probes, health metrics, and sub-second failover.
- **Multi-Channel Data Layer**: 7 WebRTC data channels with per-channel reliability/priority — gamepad input never blocked by file transfers.
- **Jitter Buffer**: 20ms adaptive jitter buffer on gateway — smooths variable encode times.
- **Connection Prefetch**: Pre-warm ICE gathering, encoder processes, and host selection before user clicks "Connect."

## Phase 5 — New Features

Product features beyond the current scope (see [NEW_FEATURES.md](./NEW_FEATURES.md)):

- **Cloud Save & Session Persistence**: Persistent home directories via NFS/S3FS. Game saves survive reboots. Quick resume on disconnect.
- **Audio Pipeline**: Full Opus encode/decode, WebRTC APM for AEC/NS/AGC, mixer for multi-user collaboration.
- **Collaborative Sessions**: Multi-pointer, multi-cursor remote collaboration with role-based access (viewer/editor/co-owner).
- **Connection Quality Dashboard**: Real-time and historical charts for latency, jitter, bitrate, packet loss. Global heatmap for admins.
- **AI-Assisted Session Management**: LLM-based quality troubleshooting, adaptive scheduler tuning, anomaly detection.
- **WebOS App Ecosystem**: App store with installable cloud-native apps. Code editor, dual-pane file manager, network monitor.
- **Public Embed API**: REST + WebSocket API for embedding remote desktops in third-party apps. Iframe/JS SDK for LMS, support, CI/CD.
- **Headless Mode**: API-only deployment without WebOS frontend — programmatic session creation via curl.

## Phase 6 — Beta

Public beta with community onboarding, feedback loops, and stability improvements.

- Persistent game storage, cloud saves, snapshots, LAN tunneling, and launcher integrations.
- Usage-based billing and credit system.
- WebOS desktop polish and app ecosystem.
- Horizontal scaling and multi-region deployment.
