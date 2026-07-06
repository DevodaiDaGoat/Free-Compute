# Remote Access, Game Streaming, and Universal Proxy

FreeCompute treats remote desktops, development machines, support sessions, and gaming sessions as first-class workloads. The default path is browser-accessible WebRTC with a low-latency edge control plane and host agents close to the user.

## Product Modes

| Mode | Primary use | Default session type | Scheduling priority |
| --- | --- | --- | --- |
| Desktop Mode | General cloud desktop access | Desktop Session | Stability, compatibility, fair cost |
| Development Mode | Browser IDEs, terminals, SSH, app preview ports | Desktop Session | CPU, RAM, storage, persistent networking |
| Gaming Mode | Fullscreen game streaming with controller input | Gaming Session | Lowest latency, GPU, encoder, network quality |
| Remote Support Mode | Temporary access to owned or approved systems | Remote Support Session | Approval, audit logging, timeout controls |

Host Session is reserved for host maintenance, diagnostics, encoder tests, GPU checks, and network measurements.

## Streaming Path

```
Browser / WebOS app / Native client
        |
        | HTTPS + WebSocket control, WebRTC media/data
        v
BunnyCDN edge hostname
        |
        | Fastest public ingress for signaling, auth, static client assets,
        | and relay discovery
        v
FreeCompute gateway + signaling service
        |
        | signed host control channel
        v
Host agent + VM/desktop/game process
        |
        | GPU capture + hardware encoder when available
        v
WebRTC stream back to the client
```

The CDN edge should front static assets, auth, API traffic, WebSocket signaling, and short-lived proxy ingress hostnames. Real-time media should prefer direct WebRTC UDP between client and host or an edge relay/TURN path when NAT, firewall, or policy prevents direct traffic.

## Safe Mode and Fast Mode

Safe Mode prioritizes compatibility:

- WebRTC with H.264 baseline/main or VP8 fallback.
- Conservative bitrate ramp-up.
- Adaptive resolution enabled early.
- Strong packet loss recovery and lower CPU pressure.
- Browser-first controls with keyboard, mouse, touch, clipboard, file transfer, and audio.

Fast Mode prioritizes latency:

- WebRTC UDP preferred, relay only when required.
- H.264 low-latency hardware encoding first.
- Higher start bitrate and faster bitrate adaptation.
- Fullscreen capture, high refresh rate targets, and controller polling.
- Gamepad events sent over the lowest-latency control channel available.

H.265 and AV1 are future codec targets and should be enabled per host only after encoder support, browser support, and licensing/runtime constraints are known.

## Session Capabilities

Every remote session should declare:

- Mode: `desktop`, `development`, `gaming`, or `remote-support`.
- Type: `desktop`, `gaming`, `remote-support`, or `host`.
- Resource class: `basic`, `standard`, `gaming`, or `workstation`.
- Stream preset: `safe` or `fast`.
- Supported inputs: keyboard, mouse, touch, Xbox controllers, PlayStation controllers, and generic gamepads.
- Permissions: remote control, clipboard read/write, file upload/download, audio forwarding, recording, max duration, and idle timeout.
- Network telemetry: RTT, jitter, packet loss, throughput, and a normalized score.

Remote Support sessions must require user approval for remote control, temporary access links, expiration, connection logs, audit events, and device verification when available.

## Gaming Mode Requirements

Gaming-capable hosts report:

- GPU model, vendor, VRAM, and driver version.
- Encoder support for H.264 now, H.265 and AV1 later.
- Active encoder sessions and encoder utilization.
- Network uplink/downlink, UDP availability, P2P availability, p50 latency, and p95 latency.
- Current CPU, RAM, GPU, VRAM, storage, stream, and proxy-route load.

The scheduler ranks gaming sessions by:

1. User-to-host latency and jitter.
2. Available hardware encoder and GPU allocation.
3. Host load and current stream pressure.
4. Network quality and UDP/P2P reachability.
5. Requested region and resource class.

## Universal Proxy

FreeCompute uses a universal proxy contract for app connections from WebOS, browser apps, SSH terminals, hosted apps, remote support, and game/network experiments.

The first gateway implementation lives in `apps/gateway` and provides HTTP(S)/WebSocket reverse proxying, HTTP CONNECT, WebSocket-to-TCP/UDP/SSH tunneling for browser clients, raw TCP listeners, UDP relay listeners, and WebRTC/P2P long-poll signaling.

Clients can discover the supported proxy surface with `GET /capabilities`. The response advertises browser, WebOS app, native client, host-agent, and edge-worker paths so the WebOS shell and normal browser clients can choose `/proxy`, `/ws`, `/connect`, raw listener, or `/signal` flows without hardcoding deployment details.

Supported protocol classes:

- `http` and `https` for browser apps, previews, dashboards, and hosted services.
- `websocket` for control channels, app sockets, and terminal streams.
- `webrtc` for media, input, data channels, and browser-friendly P2P.
- `ssh` for browser terminal, WebOS terminal, and native SSH style access.
- `tcp` for native clients, host agents, and tunneled app connections.
- `udp` for game networking, voice, discovery, and future low-latency relays.
- `p2p` for direct browser/agent routes when NAT traversal succeeds.

Browsers cannot open arbitrary raw TCP or UDP sockets directly. Browser-based TCP, UDP, and SSH must be encapsulated through WebRTC data channels, WebSocket tunnels, WebTransport/QUIC where available, or a FreeCompute edge relay. Native and WebOS agents can use direct TCP/UDP where the OS allows it.

## Proxy Route Modes

| Mode | Best for | Behavior |
| --- | --- | --- |
| `edge-relay` | Browser access, NAT traversal, BunnyCDN-fronted ingress | Client connects to an edge hostname, edge relays to gateway or host agent |
| `direct-p2p` | Lowest latency WebRTC | Signaling service introduces peers, media/data flows directly when possible |
| `host-tunnel` | SSH, private services, TCP/UDP from host | Host agent keeps an outbound tunnel and exposes only authorized routes |

Proxy routes must be short-lived by default, authenticated, logged, and bound to a session or user. Private network targets should be blocked by default unless a verified host agent owns the target and policy grants access.

## BunnyCDN-Oriented Deployment

Use BunnyCDN-facing hostnames for:

- Static frontend and WebOS assets.
- API gateway and auth endpoints.
- WebSocket signaling endpoints when the selected Bunny product path supports it.
- Relay discovery documents.
- Short-lived HTTPS proxy ingress names.

Keep these outside normal cache behavior:

- WebRTC media.
- SSH tunnels.
- TCP and UDP relays.
- File transfer chunks.
- Session recordings until they are finalized and immutable.

Recommended edge behavior:

- Cache immutable static assets aggressively.
- Disable buffering on streaming/control endpoints.
- Preserve upgrade headers for WebSocket routes.
- Use nearest-region routing for signaling and relay discovery.
- Keep connection tokens short-lived and scoped to one route/session.
- Bypass cache for `/proxy/*`, `/ws/*`, `/connect/*`, `/agent/*`, and `/signal/*`; these are live control or tunnel paths.

## API Contracts

The shared TypeScript contracts live in `packages/api-types/src`:

- `remote.ts` defines sessions, streaming, permissions, temporary links, audit logs, and WebRTC signaling.
- `proxy.ts` defines universal proxy protocols, route modes, target addresses, ingress endpoints, policies, metrics, and SSH profiles.
- `host.ts` includes GPU, encoder, network, proxy, and WebRTC capabilities.
- `queue.ts` and `vm.ts` include session mode, resource class, GPU, latency, and stream hints.
- `websocket.ts` includes mouse, keyboard, touch, gamepad, clipboard, file transfer, session control, and network telemetry messages.
