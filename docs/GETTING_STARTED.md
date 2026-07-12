# Getting Started

## Prerequisites

- Go 1.22+
- Node.js 20+
- Git

Optional (for real VM functionality):
- `ffmpeg` — video encoding for streaming
- `qemu-system-x86_64` — actual VM launch
- `tailscale` — peer-to-peer mesh networking

## Quick Start (Dev)

### Terminal 1 — Website + Gateway
```bash
./start-website.sh
```
Builds and starts:
- **Gateway** on `:8080` (API, tunnels, auth, WebRTC, everything)
- **Frontend** dev server on `:3000` (WebOS + console)
- **VM-setup agent** (registers this machine)

### Terminal 2 — VM Host Agent
```bash
./start-vms.sh
```
Builds and starts:
- **Host agent** — reverse tunnel to gateway
- **VM-setup** — registers VM, starts encoding pipeline

50% of host resources allocated by default:
```bash
# Change allocation percentage:
FREECOMPUTE_RESOURCE_PCT=75 ./start-vms.sh

# Or set manually:
FREECOMPUTE_VM_CPUCORES=8 FREECOMPUTE_VM_RAMGB=6 ./start-vms.sh
```

### Access
| URL | What |
|-----|------|
| http://localhost:3000 | Landing page |
| http://localhost:3000/webos | WebOS desktop (main app) |
| http://localhost:8080 | Gateway API |
| http://localhost:8080/healthz | Health check |
| http://localhost:8080/capabilities | Transport capabilities |

## First Login

Admin password is auto-generated on first start — check the gateway log:
```
gateway: generated admin password: <RANDOM_16_CHARS>
```

Or register a new account at http://localhost:3000/webos → "Register".

Dev tunnel token: `dev-token`

## Verify VM Agent Config
```bash
cd host-agent && go run ./cmd/vm-setup --dry-run
```
Prints the resolved config and QEMU args without launching anything.

## Key Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `FREECOMPUTE_GATEWAY_ADDR` | `:8080` | Gateway listen address |
| `FREECOMPUTE_TUNNEL_TOKEN` | `dev-token` | Auth token for host agents |
| `FREECOMPUTE_DB_PATH` | `/tmp/freecompute.db` | SQLite database path |
| `FREECOMPUTE_ADMIN_EMAIL` | `admin` | Admin user email |
| `FREECOMPUTE_ADMIN_PASSWORD` | (auto-generated) | Admin password |
| `FREECOMPUTE_RESOURCE_PCT` | `50` | VM resource % of host (50 = half) |
| `FREECOMPUTE_VM_CPUCORES` | auto (50%) | Override VM CPU cores |
| `FREECOMPUTE_VM_RAMGB` | auto (50%) | Override VM RAM GB |
| `FREECOMPUTE_VM_STORAGEGB` | auto (50%) | Override VM disk GB |
| `FREECOMPUTE_CDN_HOSTNAME` | — | Production CDN hostname (CORS allowlist) |
| `FREECOMPUTE_MODERATION_LLM_URL` | — | Optional LLM for AI moderation |

## Where Things Live

```
todo.txt            Quick change index → points to docs/
docs/ARCHITECTURE.md     Full service architecture
docs/ROADMAP.md          What's done, in progress, planned
docs/TODO.md             Detailed work items + bug tracker
docs/SECURITY_AUTH.md    Security model
docs/CONNECTION_QUALITY.md  Latency targets
docs/OPTIMIZATION_TIPS.md  Performance tips
```

## Running Tests

```bash
# Gateway (with race detector)
cd apps/gateway && go test -race ./...

# Frontend typecheck
cd apps/frontend && npx tsc --noEmit

# VM agent
cd host-agent && go build ./...
```

## Production Checklist

- [ ] Change `FREECOMPUTE_TUNNEL_TOKEN` from `dev-token` to a random secret
- [ ] Set `FREECOMPUTE_ADMIN_EMAIL` + `FREECOMPUTE_ADMIN_PASSWORD`
- [ ] Put gateway behind TLS reverse proxy (nginx / Cloudflare Tunnel)
- [ ] Set `FREECOMPUTE_CDN_HOSTNAME` for CORS allowlist
- [ ] Set `FREECOMPUTE_DB_PATH` to a persistent directory (not /tmp)
- [ ] Install `ffmpeg` on host machines for video encoding
- [ ] Install `qemu-system-x86_64` for actual VM launch
- [ ] Install `tailscale` for peer-to-peer mesh
