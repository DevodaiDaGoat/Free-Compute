#!/bin/bash
set -e

ROOT="$(cd "$(dirname "$0")" && pwd)"
echo "=== FreeCompute Website + Tunnels ==="
echo "Root: $ROOT"

# ------------------------------------------------------------------
# Gateway (includes tunnels)
# ------------------------------------------------------------------
export FREECOMPUTE_GATEWAY_ADDR="${FREECOMPUTE_GATEWAY_ADDR:-:8080}"
export FREECOMPUTE_TUNNEL_TOKEN="${FREECOMPUTE_TUNNEL_TOKEN:-dev-token}"

export FREECOMPUTE_TUNNEL_ROUTES="${FREECOMPUTE_TUNNEL_ROUTES:-[{\"id\":\"web\",\"protocol\":\"http\",\"target\":\"http://localhost:3000\"},{\"id\":\"api\",\"protocol\":\"http\",\"target\":\"http://localhost:8081\"},{\"id\":\"vm-http\",\"protocol\":\"http\",\"target\":\"http://localhost:8082\"},{\"id\":\"vm-ws\",\"protocol\":\"websocket\",\"target\":\"http://localhost:8082\"},{\"id\":\"vm-ssh\",\"protocol\":\"ssh\",\"target\":\"agent\"},{\"id\":\"vm-tcp\",\"protocol\":\"tcp\",\"target\":\"agent\"},{\"id\":\"vm-udp\",\"protocol\":\"udp\",\"target\":\"localhost:30000\"},{\"id\":\"rtc\",\"protocol\":\"webrtc\",\"listen\":\"0.0.0.0:8443\"},{\"id\":\"p2p-signal\",\"protocol\":\"p2p\",\"target\":\"\"},{\"id\":\"game-udp\",\"protocol\":\"udp\",\"listen\":\"0.0.0.0:30001\",\"target\":\"localhost:30001\"}]}"

export FREECOMPUTE_GATEWAY_SHUTDOWN_SECONDS="${FREECOMPUTE_GATEWAY_SHUTDOWN_SECONDS:-5}"
export FREECOMPUTE_TUNNEL_DIAL_SECONDS="${FREECOMPUTE_TUNNEL_DIAL_SECONDS:-3}"
export FREECOMPUTE_TUNNEL_AGENT_WAIT_SECONDS="${FREECOMPUTE_TUNNEL_AGENT_WAIT_SECONDS:-5}"
export FREECOMPUTE_GATEWAY_READ_HEADER_SECONDS="${FREECOMPUTE_GATEWAY_READ_HEADER_SECONDS:-3}"
export FREECOMPUTE_GATEWAY_IDLE_SECONDS="${FREECOMPUTE_GATEWAY_IDLE_SECONDS:-60}"
export FREECOMPUTE_PROXY_UPSTREAM_IDLE_SECONDS="${FREECOMPUTE_PROXY_UPSTREAM_IDLE_SECONDS:-30}"
export FREECOMPUTE_PROXY_RESPONSE_HEADER_SECONDS="${FREECOMPUTE_PROXY_RESPONSE_HEADER_SECONDS:-5}"
export FREECOMPUTE_PROXY_EXPECT_CONTINUE_SECONDS="${FREECOMPUTE_PROXY_EXPECT_CONTINUE_SECONDS:-1}"
export FREECOMPUTE_PROXY_MAX_IDLE_CONNS="${FREECOMPUTE_PROXY_MAX_IDLE_CONNS:-8192}"
export FREECOMPUTE_PROXY_MAX_IDLE_CONNS_PER_HOST="${FREECOMPUTE_PROXY_MAX_IDLE_CONNS_PER_HOST:-1024}"
export FREECOMPUTE_PROXY_FLUSH_MS="${FREECOMPUTE_PROXY_FLUSH_MS:--1}"

# Security & rate limiting
export FREECOMPUTE_RATE_LIMIT_RPM="${FREECOMPUTE_RATE_LIMIT_RPM:-2000}"
export FREECOMPUTE_MAX_CONNS_PER_USER="${FREECOMPUTE_MAX_CONNS_PER_USER:-100}"
export FREECOMPUTE_DEFAULT_BROWSING_MODE="${FREECOMPUTE_DEFAULT_BROWSING_MODE:-casual}"
export FREECOMPUTE_ADMIN_EMAIL="${FREECOMPUTE_ADMIN_EMAIL:-admin}"
export FREECOMPUTE_ADMIN_ROLE="${FREECOMPUTE_ADMIN_ROLE:-admin}"

export FREECOMPUTE_CDN_HOSTNAME="${FREECOMPUTE_CDN_HOSTNAME:-cdn.freecompute.io}"
export FREECOMPUTE_EDGE_HOSTNAME="${FREECOMPUTE_EDGE_HOSTNAME:-edge.freecompute.io}"
export FREECOMPUTE_API_HOSTNAME="${FREECOMPUTE_API_HOSTNAME:-api.freecompute.io}"

# ------------------------------------------------------------------
# Cleanup
# ------------------------------------------------------------------
cleanup() {
    echo ""
    echo "Stopping services..."
    pkill -f "freecompute-gateway" 2>/dev/null || true
    pkill -f "freecompute-vm-setup" 2>/dev/null || true
    pkill -f "next dev" 2>/dev/null || true
    pkill -f "next start" 2>/dev/null || true
    sleep 1
    echo "All services stopped."
}
trap cleanup EXIT INT TERM

# ------------------------------------------------------------------
# Build gateway in background while preparing the frontend
# ------------------------------------------------------------------
echo ""
echo "[1] Building gateway (includes tunnels) in background..."
(cd "$ROOT/apps/gateway" && go build -buildvcs=false -o ${TMPDIR:-/tmp}/freecompute-gateway ./cmd/gateway) &
BUILD_PID=$!

# ------------------------------------------------------------------
# Start gateway (after build completes)
# ------------------------------------------------------------------
wait $BUILD_PID
echo "[2] Starting gateway on $FREECOMPUTE_GATEWAY_ADDR..."
${TMPDIR:-/tmp}/freecompute-gateway &
GATEWAY_PID=$!
sleep 0.5

if curl -sf "http://localhost${FREECOMPUTE_GATEWAY_ADDR}/healthz" > /dev/null 2>&1; then
    echo "  Gateway healthy (PID $GATEWAY_PID)"
else
    echo "  WARNING: health check failed, retrying..."
    sleep 1
    if curl -sf "http://localhost${FREECOMPUTE_GATEWAY_ADDR}/healthz" > /dev/null 2>&1; then
        echo "  Gateway healthy (PID $GATEWAY_PID)"
    else
        echo "  ERROR: gateway failed to start"
        exit 1
    fi
fi

# ------------------------------------------------------------------
# Storage directory
# ------------------------------------------------------------------
mkdir -p ${TMPDIR:-/tmp}/freecompute-storage
echo "  Storage: ${TMPDIR:-/tmp}/freecompute-storage (10GB per user)"

# ------------------------------------------------------------------
# Build vm-setup in background
# ------------------------------------------------------------------
echo ""
echo "[3a] Building vm-setup in background..."
(cd "$ROOT/host-agent" && go build -buildvcs=false -o ${TMPDIR:-/tmp}/freecompute-vm-setup ./cmd/vm-setup) &
VM_SETUP_BUILD_PID=$!

# ------------------------------------------------------------------
# Start frontend
# ------------------------------------------------------------------
echo "[3] Starting frontend dev server..."
(cd "$ROOT/apps/frontend" && npm run dev) &
FRONTEND_PID=$!
echo "  Frontend starting (PID $FRONTEND_PID)"

wait $VM_SETUP_BUILD_PID

# ------------------------------------------------------------------
# Start vm-setup
# ------------------------------------------------------------------
echo "[4] Starting vm-setup agent..."
export FREECOMPUTE_GATEWAY_URL="http://localhost${FREECOMPUTE_GATEWAY_ADDR}"
export FREECOMPUTE_AGENT_TOKEN="${FREECOMPUTE_TUNNEL_TOKEN:-dev-token}"
export FREECOMPUTE_VM_ID="local-vm-1"
export FREECOMPUTE_VM_REGION="local"
export FREECOMPUTE_VM_STORAGEGB="10"
${TMPDIR:-/tmp}/freecompute-vm-setup &
VM_SETUP_PID=$!
echo "  vm-setup started (PID $VM_SETUP_PID)"

# ------------------------------------------------------------------
# Summary
# ------------------------------------------------------------------
echo ""
echo "=== Website + Tunnels Running ==="
echo ""
echo "Gateway (tunnels): http://localhost${FREECOMPUTE_GATEWAY_ADDR}"
echo "  Health:          http://localhost${FREECOMPUTE_GATEWAY_ADDR}/healthz"
echo "  Routes:          http://localhost${FREECOMPUTE_GATEWAY_ADDR}/routes"
echo "Frontend:          http://localhost:3000"
echo "vm-setup:          PID $VM_SETUP_PID (provisions VMs)"
echo "Storage:           ${TMPDIR:-/tmp}/freecompute-storage/ (10GB per user)"
echo ""
echo "Tunnel routes are managed by the gateway:"
echo "  HTTP/WS:   /proxy/{routeID}/{path}"
echo "  SSH:       /ssh/{routeID}"
echo "  TCP:       /connect/{routeID}"
echo "  UDP:       Direct UDP listeners on configured ports"
echo "  WebRTC:    POST /webrtc/"
echo "  P2P:       /signal/{routeID}/rooms/{roomID}"
echo "  Agent:     /agent/{routeID}  (VM connections)"
echo ""
echo "To start VMs: ./start-vms.sh"
echo ""
echo "Press Ctrl+C to stop all services."
echo ""

wait
