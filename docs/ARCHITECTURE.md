# Architecture Overview

FreeCompute is a community-powered cloud computing platform that provides browser-accessible, virtualized desktop environments.

## Service Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│   Browser    │────▶│   Gateway    │◀────│  Host Agent │
│  (Next.js)   │     │   :8080      │     │  (Go)       │
└─────────────┘     │  (Go/HTTP)   │     └─────────────┘
                    │              │
                    │  ┌───────────┴──────┐
                    │  │  File Service    │
                    │  │  :8082 (Go)      │
                    │  └──────────────────┘
                    │  ┌───────────┴──────┐
                    │  │   Scheduler      │
                    │  │  :8083 (Go)      │
                    │  └──────────────────┘
                    │
                    │  Internal Packages:
                    │  ├── Monitoring     (metrics, health, collector)
                    │  ├── Images         (VM images & snapshots)
                    │  ├── Keys           (SSH key management)
                    │  ├── Firewall       (rules & security groups)
                    │  └── Usage          (tracking & quotas)
                    └──────────────────────────────────
```

## Service Roles

| Service | Port | Role |
|---------|------|------|
| **Gateway** | `:8080` | Central API gateway, tunnel proxy, WebRTC signaling, all business logic |
| **File Service** | `:8082` | Dedicated file upload/download service with local and S3 backends |
| **Scheduler** | `:8083` | VM/desktop session scheduling, host ranking, resource allocation |
| **Host Agent** | — | Reverse tunnel client, VM lifecycle management, host capability reporting |
| **Frontend** | `:3000` | Console dashboard (Next.js) + WebOS desktop environment |

## Feature Modules (in Gateway)

| Module | Description |
|--------|-------------|
| **tunnel** | HTTP/TCP/UDP/WebSocket/SSH proxy, agent pool, signaling |
| **webrtc** | WebRTC PeerConnection, codec negotiation, ICE |
| **session** | Desktop/gaming/remote session lifecycle management |
| **auth** | JWT-based user auth, registration, login |
| **storage** | Filesystem-based file storage |
| **input** | Keyboard, mouse, touch, gamepad input handling |
| **audio** | Opus/AAC audio streaming with AEC/NS stubs |
| **gaming** | Gaming session controls, controller state, performance metrics |
| **transfer** | File transfer and clipboard sync |
| **security** | Threat detection, crypto mining detection, auto-pause |
| **admin** | User management, threat review, system settings |
| **monitoring** | Prometheus metrics, component health checking, system stats collection |
| **images** | VM image catalog, custom image creation, snapshot management |
| **keys** | SSH public key management with fingerprint validation |
| **firewall** | Firewall rules engine, security groups, VM assignment |
| **usage** | Resource usage tracking, quota enforcement, invoice generation |

## Data Flow

```
Browser ──HTTP/WS──▶ Gateway ──tunnel──▶ Host Agent ──QEMU──▶ VM
                        │                    │
                        ├── File Service ────┤
                        ├── Scheduler ───────┤
                        └── Monitoring ──────┘
```

For detailed technical specifications, see:

- [Backend Structure](./BACKEND_STRUCTURE.md) — full directory trees, API endpoints, database schema.
- [Desktop Structure](./DESKTOP_STRUCTURE.md) — WebOS frontend, window management, and streaming client.
- [Remote Access and Streaming](./REMOTE_ACCESS_STREAMING.md) — desktop sessions, gaming mode, remote support, WebRTC, and proxy architecture.
- [Performance Optimization](./PERFORMANCE_OPTIMIZATION.md) — buffer pooling, heap queue, ring buffer, UDP leak fix, and more.
- [Connection Quality](./CONNECTION_QUALITY.md) — bandwidth estimation, adaptive codec, WebTransport, session migration, edge relay optimization.
- [New Features](./NEW_FEATURES.md) — collaboration, cloud persistence, AI management, app ecosystem, audio pipeline.
- [WebTransport Strategy](./WEBTRANSPORT_STRATEGY.md) — QUIC/HTTP3 migration path, datagram input, multi-transport coexistence.
- [Performance Optimization](./PERFORMANCE_OPTIMIZATION.md) — zero-copy TCP, buffer pools, goroutine bounding, WebSocket rewrite, UDP relay GC, and WebRTC stats.
- [Network Acceleration](./NETWORK_ACCELERATION.md) — WebTransport, HTTP/3, connection coalescing, adaptive bitrate, edge caching, DSCP prioritization, and protocol negotiation.
- [Next-Gen Features](./NEXT_GEN_FEATURES.md) — WASM client, predictive rendering, ML bitrate, preemptive preconnect, QUIC migration, simulcast, and gateway replication.
