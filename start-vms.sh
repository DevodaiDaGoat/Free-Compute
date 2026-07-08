#!/bin/bash
set -e

ROOT="$(cd "$(dirname "$0")" && pwd)"
echo "=== FreeCompute VM Host Agent ==="
echo "Root: $ROOT"

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
    echo "Stopping host-agent..."
    pkill -f "freecompute-host-agent" 2>/dev/null || true
    sleep 1
    echo "Host agent stopped."
}
trap cleanup EXIT INT TERM

# ------------------------------------------------------------------
# Check gateway is reachable
# ------------------------------------------------------------------
echo ""
echo "[1] Checking gateway at $FREECOMPUTE_AGENT_GATEWAY_URL..."
if curl -sf "${FREECOMPUTE_AGENT_GATEWAY_URL}/healthz" > /dev/null 2>&1; then
    echo "  Gateway reachable"
else
    echo "  WARNING: gateway not reachable at $FREECOMPUTE_AGENT_GATEWAY_URL"
    echo "  Start the gateway first with: ./start-website.sh"
    echo "  Retrying in 3 seconds..."
    sleep 3
    if ! curl -sf "${FREECOMPUTE_AGENT_GATEWAY_URL}/healthz" > /dev/null 2>&1; then
        echo "  ERROR: gateway still not reachable"
        echo "  Run ./start-website.sh in another terminal first"
        exit 1
    fi
fi

# ------------------------------------------------------------------
# Build host-agent
# ------------------------------------------------------------------
echo "[2] Building host-agent..."
(cd "$ROOT/host-agent" && go build -buildvcs=false -o /tmp/freecompute-host-agent ./cmd/host-agent)

# ------------------------------------------------------------------
# Start host-agent
# ------------------------------------------------------------------
echo "[3] Starting host-agent connecting to ${FREECOMPUTE_AGENT_GATEWAY_URL}..."
/tmp/freecompute-host-agent &
AGENT_PID=$!
echo "  Agent started (PID $AGENT_PID)"

# ------------------------------------------------------------------
# Summary
# ------------------------------------------------------------------
echo ""
echo "=== VM Host Agent Running ==="
echo ""
echo "Gateway:     ${FREECOMPUTE_AGENT_GATEWAY_URL}"
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
