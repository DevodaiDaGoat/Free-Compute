# LAN Tunneling and Multiplayer Network Optimization

## Goal

Allow a hosted VM to expose services on its local area network through the FreeCompute universal proxy, and to carry multiplayer/network game traffic with low latency, UDP support, NAT traversal, and session-aware relay grouping. Clients outside the VM's LAN should reach LAN hosts and game servers as if they were on the same network, while the gateway picks the cheapest transport (P2P, Tailscale direct, or edge relay) for each flow.

## Current State

The gateway already has the building blocks for this feature. They are not wired together for general LAN exposure or game-session optimization yet.

### What exists

- **UDP relay listener** (`apps/gateway/internal/tunnel/udp.go`): `serveUDP` binds a public `Listen` address, creates a per-client upstream via `dialUDP(route)`, and demultiplexes packets with `udpClientMap`. Each client gets a dedicated `net.UDPConn` to the route's `Target`. Idle clients are swept by `sweepLoop` after `udpClientIdleTimeout` (60s). Socket buffers are 4 MiB (`udpSocketBufferSize`), and `setUDSocketOptions(listener, route.QoS)` applies DSCP marking through `buffer_pool.go`.
- **TCP listener** (`apps/gateway/internal/tunnel/tcp.go`): `serveTCP` accepts connections and calls `relayTCP` -> `openTCP` -> `dialTCP`. `openTCP` already supports agent-tunneled routes (`route.UsesAgentTunnel()`); it tries `dialViaTailscale` first, then falls back to `agents.take(waitCtx, route.ID)` from the agent pool.
- **Agent tunnel** (`apps/gateway/internal/tunnel/agent_pool.go`, `agent_handler.go`): `handleAgentTunnel` hijacks an HTTP CONNECT on `/agent/{routeID}` and registers the resulting `net.Conn` in `agentPool.add`. Consumers call `agentPool.take` to get a pooled connection. The pool supports waiters, idle eviction, and liveness checks via `liveAgentConn` in `buffer_pool.go`.
- **Host-agent local target** (`host-agent/cmd/host-agent/main.go`): `runTunnelOnce` dials the local target with `dialLocalTarget(ctx, cfg, route)` (currently TCP only), opens a CONNECT tunnel to the gateway with `connectToGateway` (`/agent/{routeID}`), and bridges both sides with `bridge`. Routes are driven by `FREECOMPUTE_AGENT_ROUTES`.
- **P2P signaling** (`apps/gateway/internal/tunnel/signaling.go`): `handleSignal` exposes `/signal/{routeID}/rooms/{roomID}` with long-poll. Rooms cap at 256 messages and TTL 10 minutes. The store is scoped to `ProtocolWebRTC` and `ProtocolP2P` routes.
- **Tailscale direct** (`apps/gateway/internal/tunnel/server.go`): `dialViaTailscale` races TCP dials to registered Tailscale hosts. `handleTailscaleRegister` / `handleTailscaleHosts` track hosts with a 5-minute freshness window. `handleTailscaleProxy` returns a synthetic route for a target IP:port.
- **QoS plumbing** (`apps/gateway/internal/tunnel/buffer_pool.go`): `setUDSocketOptions` and `applyTCPSocketOptions` set SO_RCVBUF/SO_SNDBUF and honor `QoSConfig.DSCP` via `IP_TOS` / `IPV6_TCLASS`. The `RouteConfig.QoS` field already carries this through the config layer.
- **Route config** (`apps/gateway/internal/tunnel/config.go`): `RouteConfig` supports `Protocol`, `Listen`, `Target`, `IdleTimeoutSeconds`, and `QoS *QoSConfig`. `UsesAgentTunnel()` currently gates on `ProtocolTCP`/`ProtocolSSH` with `Target == "agent"`.

### Gaps

- LAN discovery is not implemented. The host-agent has no mechanism to enumerate reachable LAN hosts or advertise them to the gateway.
- Port-range exposure is unsupported. A route maps exactly one `Listen:Target` pair; there is no wildcard or port-range mapping for "expose everything on 10.0.0.0/24".
- Multiplayer session grouping is absent. UDP clients are isolated per `clientAddr`; there is no concept of a "game room" where multiple players share relay state or a jitter buffer.
- DSCP marking is config-stubbed but rarely used. No route in `run-backend.sh` sets `qos`, so game traffic and best-effort traffic share the same DSCP class.
- Jitter buffers and frame reordering are not implemented in `udp.go`; packets are forwarded in arrival order.
- Agent-side UDP dialing does not exist. `dialLocalTarget` in the host-agent is TCP-only, so a VM cannot proxy UDP LAN services through an agent tunnel today.
- NAT traversal policy is not explicit. The gateway has P2P signaling, WebRTC relay, and Tailscale, but there is no router that chooses among them based on client NAT type or route mode.

## LAN Service Exposure

### Model

A host-agent route acts as the bridge between the gateway and the VM's LAN. When a client sends traffic to the gateway's public listener for a route, the gateway forwards it to the agent tunnel; the agent dials a LAN target and relays bytes.

Reuse the existing `agentPool` and `openTCP` path instead of inventing a new tunnel type. Extend the agent tunnel so it can carry UDP in addition to TCP:

1. **Gateway side** — extend `agentPool` to transport `net.Conn`-shaped wrappers for both TCP and UDP. The existing `pooledAgentConn` interface is sufficient because `net.Conn` covers `Read`/`Write`; for UDP, wrap a `net.PacketConn`-style flow inside a `net.Conn` shim or add a parallel `udpAgentPool`. The simpler path is to keep the agent pool as a TCP control channel and run UDP relay end-to-end on the agent, similar to how `serveUDP` runs on the gateway.
2. **Agent side** — extend `dialLocalTarget` to accept a protocol parameter and return either a `net.Conn` (TCP) or a UDP socket wrapper. `runTunnelOnce` already bridges two `net.Conn`s; for UDP, the bridge becomes two goroutines that read from the local UDP socket and write to the agent tunnel (and reverse).

### Config shape

```jsonc
// FREECOMPUTE_TUNNEL_ROUTES (gateway side)
{
  "id": "lan-http",
  "protocol": "http",
  "target": "agent",
  "listen": "0.0.0.0:8080"
},
{
  "id": "lan-game",
  "protocol": "udp",
  "target": "agent",
  "listen": "0.0.0.0:30001",
  "qos": { "dscp": 46 }   // EF for game traffic
}
```

```jsonc
// FREECOMPUTE_AGENT_ROUTES (host-agent side)
{
  "id": "lan-http",
  "target": "192.168.1.10:80",
  "poolSize": 2
},
{
  "id": "lan-game",
  "target": "192.168.1.20:27015",
  "poolSize": 4
}
```

Route validation in `config.go` should allow `ProtocolUDP` with `Target == "agent"` and treat it the same way `ProtocolTCP`/`ProtocolSSH` currently do in `UsesAgentTunnel()`. The agent-side `RouteConfig` validation should also permit UDP targets.

### Reuse points

- `agent_pool.go`: `add` / `take` / `removeIdle` already handle lifecycle. No structural change needed for TCP; for UDP, use a separate `udpAgentPool` keyed by route ID, or multiplex a single agent tunnel and tag frames with a route discriminator.
- `agent_handler.go`: HTTP CONNECT hijack on `/agent/{routeID}` is the control-channel entrypoint. For UDP, either reuse CONNECT as a control channel and run UDP on a sub-path (`/agent/{routeID}/udp` upgraded to WebSocket/WebTransport) or run a raw UDP listener behind the agent and map it to the pool.
- `tcp.go:openTCP` — the `dialViaTailscale` fallback should remain available for agent-routed TCP. For UDP, a similar fallback can be added to `dialUDP` in `udp.go`.

## Multiplayer Optimization

### UDP relay tuning

`udp.go` already sets generous socket buffers (4 MiB) and a 60-second idle timeout. For game traffic, expose per-route tuning:

- **Idle timeout**: Lower `udpClientIdleTimeout` for known game ports (e.g., 5s) to free stale players faster; keep 60s as the default.
- **Sweep interval**: `udpClientSweepInterval` is 30s. Consider making it configurable or adaptive (e.g., sweep more aggressively when client count exceeds a threshold).
- **Packet budget**: `maxUDPPacketSize` is 64 KiB. Game payloads are usually < 1.2 KiB; cap application writes to 1.5 KiB to avoid pathological fragmentation on the public listener.

### DSCP marking

`RouteConfig.QoS` carries a `DSCP` value. `setUDSocketOptions` and `applyTCPSocketOptions` already write `IP_TOS` / `IPV6_TCLASS`. For multiplayer:

- Game UDP routes should set `qos.dscp = 46` (Expedited Forwarding).
- Best-effort LAN discovery or lobby traffic should use `qos.dscp = 0` or `8` (CS1).
- The agent-side `dialLocalTarget` and the agent tunnel socket should also apply DSCP so marking is end-to-end, not just at the gateway's public listener.

### Session grouping via `/signal`

`/signal/{routeID}/rooms/{roomID}` is already available. A multiplayer lobby can use one room per match:

1. Lobby creates a room under the game route ID.
2. All peers long-poll the same room; ICE candidates and session descriptions flow through `signalStore`.
3. Once P2P is established, peers migrate out of the relay. The gateway keeps the room for renegotiation only.

To support this, the `signalStore` append path should allow a higher message cap during active sessions (e.g., 1024 transient messages) or clear the room after a match starts.

### P2P where possible

The gateway already has `ProtocolP2P` and `ProtocolWebRTC`. For multiplayer:

- If both peers are on the same LAN subnet behind the same VM, the agent should prefer local loopback and skip the gateway entirely.
- If both peers have Tailscale and the gateway knows their Tailscale IPs (`tailscaleHosts`), prefer `dialViaTailscale` over relay.
- Otherwise, use WebRTC data-channel relay (`/signal`) with the gateway acting as a TURN-like edge relay.

### Jitter buffers

`copyUDPToClient` forwards packets in arrival order. For game traffic, add an optional jitter buffer on the read path:

- Buffer N packets (e.g., 4) sorted by sequence number.
- Deliver at a fixed tick rate (e.g., 60 Hz) to smooth variance.
- This is a later optimization; the initial implementation can forward immediately and add buffering as a `JitterBufferMs` route option.

## NAT Traversal

### Decision tree

1. **Same LAN / same agent VM**: agent dials local target directly; no gateway relay needed beyond the control channel.
2. **Tailscale direct**: if both client and target host have registered Tailscale IPs within the last 5 minutes (`tailscaleHosts` freshness in `server.go`), `dialViaTailscale` races a direct TCP dial. For UDP, a similar race can be added in `dialUDP`.
3. **WebRTC P2P**: if both sides are browsers or WebRTC-capable native clients, exchange ICE via `/signal/{routeID}/rooms/{roomID}` and prefer a direct DTLS/SCTP data channel.
4. **Edge relay**: if P2P fails (symmetric NAT, firewall), the gateway or an edge relay carries the traffic. The existing `webrtc.RelayMesh` and BunnyCDN edge hostnames are the fallback path.

### Route mode advertisement

`handleCapabilities` already lists `edge-relay`, `direct-p2p`, and `host-tunnel`. For LAN game routes, advertise all three and let the client pick based on its own NAT measurement.

## Implementation Steps

1. **Allow UDP agent routes** (`apps/gateway/internal/tunnel/config.go`):
   - Update `UsesAgentTunnel()` to include `ProtocolUDP` when `Target == "agent"`.
   - Update `Validate()` to accept UDP agent targets.
   - Add `udpAgentPool` struct similar to `agentPool` but keyed by route ID and holding `*net.UDPConn` + client address mappings.

2. **Agent-side UDP dial** (`host-agent/cmd/host-agent/main.go`):
   - Add a `protocol` field to `RouteConfig` (or infer from route ID / new env var).
   - Implement `dialLocalUDPTarget(ctx, route) (*net.UDPConn, error)` mirroring `dialLocalTarget`.
   - In `runTunnelOnce`, branch on protocol: TCP uses the existing `bridge`; UDP runs `copyUDP` loops between the local UDP socket and the agent tunnel `net.Conn`.

3. **Gateway-side UDP relay through agent** (`apps/gateway/internal/tunnel/udp.go`):
   - In `serveUDP`, when `route.UsesAgentTunnel()`, instead of `dialUDP(route)`, call `s.agents.takeUDP(ctx, route.ID)`.
   - The agent side already forwards UDP from the LAN to the gateway tunnel; the gateway writes to the pooled agent connection and reads back.

4. **DSCP propagation** (`apps/gateway/internal/tunnel/udp.go`, `buffer_pool.go`, `host-agent/cmd/host-agent/main.go`):
   - Apply `route.QoS` in `dialUDP` (upstream socket) and in the agent tunnel socket.
   - In the host-agent, apply QoS to the locally dialed UDP socket using the same `unix.IP_TOS` / `IPV6_TCLASS` pattern.

5. **Signal room reuse for multiplayer** (`apps/gateway/internal/tunnel/signaling.go`):
   - Raise `maxSignalMessagesPerRoom` transiently for active game sessions, or add a `signalRoom.SetTTL(duration)` method.
   - Document that `/signal` is the canonical way to bootstrap P2P for a LAN game session exposed through the gateway.

6. **Tailscale UDP fallback** (`apps/gateway/internal/tunnel/server.go`, `udp.go`):
   - Add `dialUDPViaTailscale(route)` racing UDP dials to known Tailscale hosts.
   - Call it from `serveUDP` before falling back to the default `dialUDP`.

7. **Route examples** (`run-backend.sh`):
   - Add a LAN game route with QoS:
     ```json
     {"id":"lan-game","protocol":"udp","target":"agent","listen":"0.0.0.0:30001","qos":{"dscp":46}}
     ```
   - Add the matching agent route:
     ```json
     {"id":"lan-game","target":"192.168.1.20:27015","poolSize":4}
     ```

8. **Frontend contract** (no code, but update `packages/api-types/src/proxy.ts`):
   - Add `lanTarget` to the route definition so the UI can show "this route exposes a LAN host through the VM".
   - Expose `qos.dscp` in the route metadata.

## Acceptance Criteria

1. **LAN reachability**: two external clients can reach a LAN game server (TCP and UDP) by connecting to the gateway's public listener for the same route. The gateway forwards through the agent tunnel to the LAN target and back.
2. **p95 latency**: for a UDP game route with `qos.dscp = 46`, p95 one-way latency (gateway egress to client) stays under 20 ms on a 1 ms RTT LAN, measured through the gateway's metrics endpoint.
3. **P2P preference**: when both peers have valid Tailscale IPs or can establish WebRTC P2P, traffic bypasses the gateway relay. The fallback order is P2P -> Tailscale direct -> edge relay.
4. **DSCP honored**: `tcpdump` or `ss` on the gateway shows `IP_TOS = 0xb8` (DSCP 46 << 2) for packets on routes with `qos.dscp = 46`.
5. **Agent resilience**: killing and restarting the host-agent preserves the route; `runTunnelLoop` backoff reconnects and the agent pool refills within `AgentWaitTimeout`.

## Risks

- **UDP amplification**: `serveUDP` reflects upstream packets to the client address without validating that the client actually sent a request. An attacker can spoof source addresses and amplify traffic to arbitrary targets. Mitigation: require auth on game routes (`requireAuth: true`), rate-limit per-client packet bytes, and cap `maxUDPPacketSize` for untrusted routes.
- **Port scanning**: exposing a route with `listen` on `0.0.0.0` opens that port to the internet. If the agent tunnel is down, the gateway still binds the listener. Mitigation: return `503 Service Unavailable` when no agent is available instead of accepting and resetting; add a firewall rule in `firewall.Manager` for LAN-exposed routes.
- **Fairness**: a single UDP client can monopolize `udpClientMap` entries if it sends from many source ports. Mitigation: add a per-remote-address client cap (e.g., 16) and a global client cap per route.
- **Agent pool starvation**: if all pooled agent connections die simultaneously, new requests wait up to `AgentWaitTimeout`. Mitigation: keep-alive the agent tunnel with a small probe, and pre-warm connections via `/prewarm`.
- **DSCP trust**: middleboxes may ignore or rewrite DSCP. Do not rely on DSCP alone for QoS; use it as a hint combined with socket buffer sizing and TCP BBR / UDP buffer tuning already present in `buffer_pool.go`.
