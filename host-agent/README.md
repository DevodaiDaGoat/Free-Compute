# FreeCompute Host Agent

The host agent currently includes a reverse TCP tunnel client. It lets a machine behind NAT create outbound tunnel streams to the gateway, then the gateway can attach browser, CONNECT, or raw TCP clients to those streams.

## Gateway Route

Configure a gateway route with `"target": "agent"`:

```bash
FREECOMPUTE_GATEWAY_ADDR=:8080 \
FREECOMPUTE_TUNNEL_TOKEN=dev-token \
FREECOMPUTE_TUNNEL_ROUTES='[
  {
    "id": "home-ssh",
    "protocol": "ssh",
    "target": "agent"
  }
]' \
go run ./apps/gateway/cmd/gateway
```

## Host Agent

Run this on the machine that owns the private service:

```bash
FREECOMPUTE_AGENT_GATEWAY_URL=http://127.0.0.1:8080 \
FREECOMPUTE_AGENT_TOKEN=dev-token \
FREECOMPUTE_AGENT_ROUTES='[
  {
    "id": "home-ssh",
    "target": "127.0.0.1:22",
    "poolSize": 4
  }
]' \
go run ./host-agent/cmd/host-agent
```

Each pool slot opens the local target first, then advertises one outbound tunnel to the gateway. When a client connects to `/ws/home-ssh`, `/connect/home-ssh`, or a raw TCP listener for that route, the gateway consumes one ready agent stream and bridges it to that pre-checked local connection.

Use HTTPS for the gateway URL in production. `FREECOMPUTE_AGENT_INSECURE_SKIP_TLS=1` exists only for local testing against self-signed certificates.
