# Roadmap

## Phase 1 — Architecture

Define system architecture, API contracts, and data models.

- Remote session, gaming mode, host capability, and proxy route contracts.
- Initial tunnel gateway for HTTP(S), WebSocket, TCP, UDP, SSH-over-WebSocket, HTTP CONNECT, and WebRTC/P2P signaling.

## Phase 2 — Implementation

Build core services (gateway, scheduler, host agent) and the WebOS frontend.

- Host-agent reverse tunnel registration and per-session authorization.
- WebRTC media pipeline with Safe Mode and Fast Mode presets.
- Gaming scheduler scoring for latency, GPU availability, encoder support, and host load.
- Browser terminal integration through SSH-over-WebSocket tunnels.

## Phase 3 — Alpha

Internal testing with a small set of host providers and users.

- Remote support approval flows, audit logs, temporary links, and session timeout enforcement.
- Controller input testing across Xbox, PlayStation, and generic gamepads.

## Phase 4 — Beta

Public beta with community onboarding, feedback loops, and stability improvements.

- Persistent game storage, cloud saves, snapshots, LAN tunneling, and launcher integrations.
