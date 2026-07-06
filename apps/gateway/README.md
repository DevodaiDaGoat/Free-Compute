# FreeCompute Gateway

The gateway currently includes a lightweight tunnel service for development and early edge deployments. It is stdlib-only Go so it can build without downloading dependencies.

## Run

```bash
FREECOMPUTE_GATEWAY_ADDR=:8080 \
FREECOMPUTE_TUNNEL_TOKEN=dev-token \
FREECOMPUTE_TUNNEL_ROUTES='[
  {
    "id": "web",
    "protocol": "http",
    "target": "http://127.0.0.1:3000"
  },
  {
    "id": "ssh",
    "protocol": "ssh",
    "listen": "127.0.0.1:2222",
    "target": "127.0.0.1:22"
  },
  {
    "id": "game-udp",
    "protocol": "udp",
    "listen": "127.0.0.1:27015",
    "target": "127.0.0.1:27015"
  },
  {
    "id": "rtc",
    "protocol": "webrtc"
  }
]' \
go run ./apps/gateway/cmd/gateway
```

Use `GOCACHE=/tmp/go-build` in locked-down environments where the default Go build cache is not writable.

## Endpoints

- `GET /healthz` returns gateway health.
- `GET /capabilities` returns the universal proxy capability document for browser, WebOS, native, host-agent, and edge-worker clients.
- `GET /routes` returns configured routes and requires `FREECOMPUTE_TUNNEL_TOKEN` when set.
- `ANY /proxy/{routeID}/{path}` reverse-proxies `http`, `https`, and upstream WebSocket routes.
- `CONNECT /connect/{routeID}` opens an HTTP CONNECT stream to a `tcp` or `ssh` target.
- `GET /ws/{routeID}` upgrades to WebSocket and bridges browser frames to a `tcp`, `udp`, or `ssh` target.
- `POST /signal/{routeID}/rooms/{roomID}` publishes a WebRTC/P2P signaling message.
- `GET /signal/{routeID}/rooms/{roomID}?after={seq}` long-polls signaling messages.

Raw TCP and UDP listeners are enabled per route with the `listen` field.

## Performance Knobs

The gateway is tuned for low-latency streaming by default while staying stdlib-only:

- `FREECOMPUTE_PROXY_FLUSH_MS=-1` flushes streamed proxy responses immediately.
- `FREECOMPUTE_PROXY_MAX_IDLE_CONNS=1024` keeps a large upstream connection pool ready.
- `FREECOMPUTE_PROXY_MAX_IDLE_CONNS_PER_HOST=128` avoids reconnect churn for busy route targets.
- `FREECOMPUTE_PROXY_UPSTREAM_IDLE_SECONDS=90` controls idle upstream keepalive lifetime.
- `FREECOMPUTE_TUNNEL_DIAL_SECONDS=10` caps outbound route dial latency.
- `FREECOMPUTE_TUNNEL_AGENT_WAIT_SECONDS=15` caps how long client connections wait for a ready host-agent tunnel.
- `FREECOMPUTE_GATEWAY_READ_HEADER_SECONDS=5` limits slow client header reads.
- `FREECOMPUTE_GATEWAY_IDLE_SECONDS=120` controls HTTP keepalive lifetime.

TCP sockets enable keepalive and `TCP_NODELAY`; UDP relay sockets request larger OS buffers for game and voice traffic bursts. Tune OS socket buffers and regional placement separately for production gaming hosts.

Agent-backed routes skip stale closed tunnel slots before assigning a client and preserve any already-sent upstream bytes, which matters for protocols such as SSH that may send a banner immediately.

## Route Types

| Protocol | Browser path | Native/WebOS path | Notes |
| --- | --- | --- | --- |
| `http` / `https` | `/proxy/{routeID}` | `/proxy/{routeID}` | Works for app previews and hosted web apps |
| `websocket` | `/proxy/{routeID}` | `/proxy/{routeID}` | Uses Go reverse proxy upgrade support |
| `ssh` | `/ws/{routeID}` | raw TCP listener or `CONNECT` | Browser terminal can speak SSH over WebSocket |
| `tcp` | `/ws/{routeID}` | raw TCP listener or `CONNECT` | Browser path encapsulates TCP in WebSocket |
| `udp` | `/ws/{routeID}` datagrams or future WebRTC data/WebTransport/QUIC relay | raw UDP listener | Useful for game networking and voice relay experiments |
| `webrtc` / `p2p` | `/signal/{routeID}/rooms/{roomID}` | same | Signaling only; media/data should flow direct or through TURN/relay |

## Auth

Set `FREECOMPUTE_TUNNEL_TOKEN` to require either:

```text
Authorization: Bearer <token>
```

or:

```text
X-FreeCompute-Tunnel-Token: <token>
```

Routes inherit token auth by default. A route can set `"requireAuth": false` for local-only development, but production routes should stay authenticated and short-lived.

## BunnyCDN Placement

Put BunnyCDN-facing hostnames in front of:

- Static frontend assets.
- API and auth endpoints.
- `/proxy/*` app routes that are HTTP(S)/WebSocket compatible.
- `/signal/*` signaling routes.

Keep raw TCP and UDP listeners on infrastructure that supports those transports directly. Browser access to TCP/SSH/UDP can use `/ws/{routeID}` through HTTPS, while direct game UDP should prefer the raw listener, a future QUIC/WebTransport edge path, or WebRTC/TURN relay when available.

Bypass cache and buffering for `/proxy/*`, `/ws/*`, `/connect/*`, `/agent/*`, and `/signal/*`. Cache immutable frontend/WebOS assets aggressively, but keep tokens and route documents short-lived.
