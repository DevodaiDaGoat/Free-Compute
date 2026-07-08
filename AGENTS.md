# AGENTS.md

**FreeCompute** — Community-powered cloud computing platform. Pre-alpha.

## Project structure

Monorepo with two language ecosystems, orchestrated at the top level:

```
apps/
  frontend/        Next.js 15 + React 19 + TypeScript   (npm)
  gateway/         Go 1.22 HTTP/WebRTC/tunnel server     (Go module)
  scheduler/       Go — stub (no code yet)               (Go module)
  file-service/    Go — stub (go.mod only)               (Go module)
host-agent/        Go 1.22 VM lifecycle agent             (Go module)
packages/          Shared npm packages (api-types, ui, utils, ...)
tests/unit/        Empty — no tests yet
```

The `go.work` at root wires the four Go modules.

## Real code lives in

| Where | What |
|-------|------|
| `apps/gateway/internal/tunnel/` | HTTP/TCP/UDP/WebSocket proxy, agent pool, signaling |
| `apps/gateway/internal/webrtc/` | WebRTC session logic |
| `host-agent/cmd/host-agent/` | Agent main loop |
| `apps/frontend/` | Next.js console UI |

Everything else is scaffolding or stub (`scheduler/src/`, `file-service/`, `tests/`, several `packages/`).

## Commands

```bash
npm run build       # turbo run build (TS packages → dist/)
npm run dev         # turbo run dev   (persistent, no cache)
npm run lint        # turbo run lint  (no lint config exists yet)
npm run typecheck   # turbo run typecheck (tsc --noEmit per package)

# Go services (run from service directory)
go run ./cmd/gateway          # starts gateway on :8080
go run ./cmd/host-agent       # starts host agent

# Full backend (from repo root)
./run-backend.sh              # starts gateway + agent, sets FREECOMPUTE_* env
```

## Testing

```bash
go test ./...                 # 2 tests in gateway/internal/tunnel/ — the only tests
```

No JS test framework is configured. No CI workflows exist.

## Architecture (what exists)

`gateway` is an env-driven tunnel server (`FREECOMPUTE_GATEWAY_ADDR`, `FREECOMPUTE_TUNNEL_*`, etc. — see `run-backend.sh` for defaults). `host-agent` registers with the gateway via WebSocket and tunnels routes (HTTP, TCP, UDP, WebRTC, SSH).

Key endpoints (from `run-backend.sh`):
- `GET /healthz` — health check
- `GET /capabilities` — gateway capabilities
- `GET /routes` — tunnel routes
- `POST /webrtc/`, `/sessions/`, `/gaming/`, `/input/`, `/audio/`, `/transfer/` — session endpoints

## Important notes

- **Pre-alpha**: many directories are scaffolding. Verify code exists before editing.
- **No ESLint/Prettier config** — no JS lint or format tooling is set up.
- **No Dockerfiles** — described in docs but not present.
- **Host agent `vm-setup.go`** is commented out in `run-backend.sh` (needs gateway endpoints that don't exist yet).
- Config is **env-driven** (`FREECOMPUTE_*`), not file-based.
- Gateway tunnel config is JSON in `FREECOMPUTE_TUNNEL_ROUTES` env var.
