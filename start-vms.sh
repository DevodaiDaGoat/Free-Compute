#!/bin/bash
set -e

ROOT="$(cd "$(dirname "$0")" && pwd)"
echo "=== FreeCompute VM Host Agent ==="
echo "Root: $ROOT"

# ------------------------------------------------------------------
# Storage
# ------------------------------------------------------------------
mkdir -p ${TMPDIR:-/tmp}/freecompute-storage
echo "  Storage: ${TMPDIR:-/tmp}/freecompute-storage (10GB per user)"

# ------------------------------------------------------------------
# vm-setup agent (provisions VMs)
# ------------------------------------------------------------------
export FREECOMPUTE_GATEWAY_URL="${FREECOMPUTE_GATEWAY_URL:-http://localhost:8080}"
export FREECOMPUTE_AGENT_TOKEN="${FREECOMPUTE_AGENT_TOKEN:-${FREECOMPUTE_TUNNEL_TOKEN:-dev-token}}"
export FREECOMPUTE_VM_ID="${FREECOMPUTE_VM_ID:-local-vm-1}"
export FREECOMPUTE_VM_REGION="${FREECOMPUTE_VM_REGION:-local}"
export FREECOMPUTE_VM_STORAGEGB="${FREECOMPUTE_VM_STORAGEGB:-10}"

# ------------------------------------------------------------------
# Host Agent (connects VMs to gateway tunnels)
# ------------------------------------------------------------------
export FREECOMPUTE_AGENT_GATEWAY_URL="${FREECOMPUTE_AGENT_GATEWAY_URL:-http://localhost:8080}"
export FREECOMPUTE_AGENT_TOKEN="${FREECOMPUTE_AGENT_TOKEN:-${FREECOMPUTE_TUNNEL_TOKEN:-dev-token}}"

export FREECOMPUTE_AGENT_ROUTES="${FREECOMPUTE_AGENT_ROUTES:-[
  {\"id\":\"vm-ssh\",\"target\":\"localhost:22\"},
  {\"id\":\"vm-tcp\",\"target\":\"localhost:8082\"}
]}"
export FREECOMPUTE_AGENT_DIAL_SECONDS="${FREECOMPUTE_AGENT_DIAL_SECONDS:-5}"
export FREECOMPUTE_AGENT_RECONNECT_SECONDS="${FREECOMPUTE_AGENT_RECONNECT_SECONDS:-1}"
export FREECOMPUTE_AGENT_INSECURE_SKIP_TLS="${FREECOMPUTE_AGENT_INSECURE_SKIP_TLS:-0}"

# ------------------------------------------------------------------
# Cleanup
# ------------------------------------------------------------------
cleanup() {
    echo ""
    echo "Stopping services..."
    pkill -f "freecompute-vm-setup" 2>/dev/null || true
    pkill -f "freecompute-host-agent" 2>/dev/null || true
    sleep 1
    echo "Services stopped."
}
trap cleanup EXIT INT TERM

# ------------------------------------------------------------------
# Check gateway is reachable
# ------------------------------------------------------------------
echo ""
echo "[1] Checking gateway at $FREECOMPUTE_GATEWAY_URL..."
if curl -sf "${FREECOMPUTE_GATEWAY_URL}/healthz" > /dev/null 2>&1; then
    echo "  Gateway reachable"
else
    echo "  WARNING: gateway not reachable at $FREECOMPUTE_GATEWAY_URL"
    echo "  Start the gateway first with: ./start-website.sh"
    echo "  Retrying in 3 seconds..."
    sleep 3
    if ! curl -sf "${FREECOMPUTE_GATEWAY_URL}/healthz" > /dev/null 2>&1; then
        echo "  ERROR: gateway still not reachable"
        echo "  Run ./start-website.sh in another terminal first"
        exit 1
    fi
fi

# ------------------------------------------------------------------
# Build vm-setup + host-agent in parallel
# ------------------------------------------------------------------
echo "[2] Building vm-setup + host-agent..."
(cd "$ROOT/host-agent" && go build -buildvcs=false -o ${TMPDIR:-/tmp}/freecompute-vm-setup ./cmd/vm-setup) &
(cd "$ROOT/host-agent" && go build -buildvcs=false -o ${TMPDIR:-/tmp}/freecompute-host-agent ./cmd/host-agent) &
wait

# ------------------------------------------------------------------
# Start vm-setup
# ------------------------------------------------------------------
echo "[3] Starting vm-setup agent (provisions VMs)..."
${TMPDIR:-/tmp}/freecompute-vm-setup &
VM_SETUP_PID=$!
echo "  vm-setup started (PID $VM_SETUP_PID)"

# ------------------------------------------------------------------
# Start host-agent
# ------------------------------------------------------------------
echo "[4] Starting host-agent (tunnels to gateway)..."
${TMPDIR:-/tmp}/freecompute-host-agent &
AGENT_PID=$!
echo "  Host-agent started (PID $AGENT_PID)"

# ------------------------------------------------------------------
# Summary
# ------------------------------------------------------------------
echo ""
echo "=== VM Host Agent Running ==="
echo ""
echo "Gateway:     ${FREECOMPUTE_GATEWAY_URL}"
echo "vm-setup:    PID $VM_SETUP_PID"
echo "Host Agent:  PID $AGENT_PID"
echo "Routes:"
echo "  $(echo "$FREECOMPUTE_AGENT_ROUTES" | python3 -c "
import sys, json
routes = json.load(sys.stdin)
for r in routes:
    print(f'    {r[\"id\"]} -> {r[\"target\"]}')
" 2>/dev/null || echo '    see FREECOMPUTE_AGENT_ROUTES env')"
echo ""
echo "Press Ctrl+C to stop."
echo ""

wait
