# Architecture Overview

FreeCompute is a community-powered cloud computing platform that provides browser-accessible virtual desktop environments backed by donated hardware.

## Service Architecture

```
Browser (Next.js WebOS)
    │  HTTP / WebSocket / WebRTC
    ▼
┌─────────────────────────────────────────────────────┐
│  API Gateway  :8080  (Go)                           │
│                                                     │
│  ├── auth          JWT register/login/roles         │
│  ├── tunnel        HTTP/TCP/UDP/WS/SSH proxy        │
│  ├── webrtc        PeerConnection, codec, ICE       │
│  ├── session       Desktop/gaming/support sessions  │
│  ├── security      Threat detection, AI moderation  │
│  ├── admin         Dashboard, user/VM management    │
│  ├── storage       Per-user file storage (10 GB)    │
│  ├── usage         Credits, quotas, invoices        │
│  ├── images        VM image catalog, snapshots      │
│  ├── keys          SSH key management               │
│  ├── firewall      Rules engine, security groups    │
│  ├── monitoring    Prometheus metrics, health       │
│  ├── ratelimit     Token bucket per IP + per user   │
│  └── moderation    Heuristic + optional LLM triage  │
└──────────────────┬──────────────────────────────────┘
                   │ Reverse tunnel (CONNECT + WebSocket)
                   ▼
┌─────────────────────────────────────────────────────┐
│  Host Agent  (Go)                                   │
│  ├── Registers capability with gateway              │
│  ├── Opens reverse tunnel per route                 │
│  ├── Reports CPU/RAM/GPU metrics                    │
│  └── Relays SSH / TCP / HTTP to local VM            │
└──────────────────┬──────────────────────────────────┘
                   │ localhost
                   ▼
┌─────────────────────────────────────────────────────┐
│  VM Agent  (Go — cmd/vm-setup)                      │
│  ├── Registers VM with gateway                      │
│  ├── Launches FFmpeg encoder (H.264/H.265)          │
│  ├── Starts WebRTC media session                    │
│  └── Handles audio capture + gaming mode            │
└─────────────────────────────────────────────────────┘
```

## Service Roles

| Service | Port | Role |
|---------|------|------|
| **Gateway** | `:8080` | Central API, tunnel proxy, WebRTC signaling, all business logic |
| **Host Agent** | — | Reverse tunnel client, VM lifecycle, capability reporting |
| **VM Agent** | — | Encoding, WebRTC session, gaming mode per VM |
| **Frontend** | `:3000` | Console dashboard (Next.js) + WebOS desktop environment |

## Monorepo Structure (actual)

```
apps/
  frontend/           Next.js 15 + React 19 + TypeScript
    app/
      page.tsx         Landing page
      connect/         Connection space
      gateway/         Admin console
      webos/           Full WebOS desktop
        boot/          BIOS → login sequence
        desktop/       macOS-style desktop + dock + menu bar
        taskbar/       (legacy, replaced by Dock in Desktop)
        window-manager/ Window chrome, resize, traffic lights
        apps/
          terminal/    SSH-over-WebSocket terminal
          browser/     In-WebOS browser
          files/       File manager
          settings/    Account + connection settings
          remote-desktop/  Stream + SSH + Tailscale tabs
          host-monitor/    Live host CPU/RAM/GPU dashboard
          ai-monitor/      Heuristic log threat analysis
          credits/         Credits, crypto payments, earn
          store/           App Player (install/run packages)
          task-manager/    Process list
          admin/           Admin panel
          calculator/      Calculator
        system/
          types.ts         Shared TypeScript types
          api/websocket.ts Gateway API client (fetch + retry)
          hooks/
            useTunnelConnection.ts  Binary multiplexed WebSocket

  gateway/            Go — all backend logic
    cmd/gateway/       Entry point
    internal/
      tunnel/          HTTP mux, proxy, agent pool, WebSocket
      webrtc/          PeerConnection, bandwidth estimator
      session/         Session manager, host allocator
      auth/            JWT, register, login, preferences, roles
      database/        SQLite, migrations, all DB queries
      security/        Threat detection, crypto signatures
      admin/           Dashboard, user/VM ops
      storage/         Per-user file storage
      usage/           Credits, quotas, invoices
      moderation/      Heuristic + LLM triage
      monitoring/      Prometheus, health checker
      ratelimit/       Token bucket per IP + per user
      images/          VM image catalog
      keys/            SSH key management
      firewall/        Rules engine

host-agent/           Go — host/VM agent
  cmd/
    host-agent/        Reverse tunnel client
    vm-setup/          VM registration + encoding
  internal/
    vmagent/           VMAgent, encoder manager, config loader

docs/                 All planning and architecture docs
scripts/              Helper scripts
```

## Security Model

```
Internet request
  → CORS origin allowlist check (only allowed origins get credentials)
  → Rate limiter (token bucket per IP, per user)
  → Auth middleware (JWT Bearer token validation)
  → Role check (user=0, moderator=1, admin=2)
  → Handler
      Storage: userId forced from JWT (not query param)
      Usage:   userId forced from JWT (IDOR prevention)
      Hosts:   requires tunnel token OR admin JWT
  → Response (internal errors → generic message, never stack trace)
```

## Transport Stack

| Path | Protocol | Use |
|------|----------|-----|
| `/proxy/{id}/` | HTTP reverse proxy | Browser HTTP traffic |
| `/ws/{id}` | WebSocket tunnel | TCP/SSH over WS |
| `/connect/{id}` | HTTP CONNECT | Raw TCP tunnel |
| `/ssh/{id}` | SSH-over-WebSocket | Terminal access |
| `/agent/{id}` | HTTP CONNECT (agent) | VM reverse tunnel |
| `/signal/{id}/rooms/{room}` | WebSocket | WebRTC signaling |
| `/webrtc/` | REST + WebRTC | Session creation |
| `/wt/` | QUIC (stub) | WebTransport (Phase 4) |

## VM Resource Allocation

By default, the VM agent takes **50% of the host machine's resources**.
Override with `FREECOMPUTE_RESOURCE_PCT` or set values individually.

```
Host: 12 CPUs, 8 GB RAM, 688 GB disk
VM (50%): 6 CPUs, 4 GB RAM, 344 GB disk
```
