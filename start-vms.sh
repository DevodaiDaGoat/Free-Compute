#!/bin/bash
set -e

ROOT="$(cd "$(dirname "$0")" && pwd)"
echo "=== FreeCompute VM Host Agent ==="
echo "Root: $ROOT"

# ------------------------------------------------------------------
# Resource allocation (default 50% of host; override with FREECOMPUTE_RESOURCE_PCT)
# ------------------------------------------------------------------
RESOURCE_PCT="${FREECOMPUTE_RESOURCE_PCT:-50}"
TOTAL_CPUS=$(python3 -c "import multiprocessing; print(multiprocessing.cpu_count())" 2>/dev/null || nproc 2>/dev/null || echo 4)
TOTAL_RAM_GB=$(python3 -c "
import subprocess
r = subprocess.check_output(['wmic','computersystem','get','totalphysicalmemory'],text=True,stderr=open('/dev/null'))
print(max(1, int([x for x in r.split() if x.isdigit()][0]) // (1024**3)))
" 2>/dev/null || free -g 2>/dev/null | awk '/^Mem:/{print $2}' || echo 8)
TOTAL_DISK_GB=$(python3 -c "
import shutil
t,_,_ = shutil.disk_usage('/' if __import__('os').name!='nt' else 'C:/')
print(max(10, t//(1024**3)))
" 2>/dev/null || df -BG / 2>/dev/null | awk 'NR==2{gsub(/G/,\"\",$2);print $2}' || echo 100)

VM_CPUS=$(python3 -c "print(max(1, int($TOTAL_CPUS * $RESOURCE_PCT / 100)))")
VM_RAM_GB=$(python3 -c "print(max(1, int($TOTAL_RAM_GB * $RESOURCE_PCT / 100)))")
VM_DISK_GB=$(python3 -c "print(max(10, int($TOTAL_DISK_GB * $RESOURCE_PCT / 100)))")

echo ""
echo "Resource allocation (${RESOURCE_PCT}% of host):"
echo "  Host: ${TOTAL_CPUS} CPUs, ${TOTAL_RAM_GB}GB RAM, ${TOTAL_DISK_GB}GB disk"
echo "  VM:   ${VM_CPUS} CPUs, ${VM_RAM_GB}GB RAM, ${VM_DISK_GB}GB disk"
echo ""

# ------------------------------------------------------------------
# Storage
# ------------------------------------------------------------------
mkdir -p ${TMPDIR:-/tmp}/freecompute-storage
echo "  Storage: ${TMPDIR:-/tmp}/freecompute-storage (${VM_DISK_GB}GB allocated)"

# ------------------------------------------------------------------
# vm-setup agent (provisions VMs)
# ------------------------------------------------------------------
export FREECOMPUTE_GATEWAY_URL="${FREECOMPUTE_GATEWAY_URL:-http://localhost:8080}"
export FREECOMPUTE_AGENT_TOKEN="${FREECOMPUTE_AGENT_TOKEN:-${FREECOMPUTE_TUNNEL_TOKEN:-dev-token}}"
export FREECOMPUTE_VM_ID="${FREECOMPUTE_VM_ID:-local-vm-1}"
export FREECOMPUTE_VM_REGION="${FREECOMPUTE_VM_REGION:-local}"
export FREECOMPUTE_VM_CPUCORES="${FREECOMPUTE_VM_CPUCORES:-$VM_CPUS}"
export FREECOMPUTE_VM_RAMGB="${FREECOMPUTE_VM_RAMGB:-$VM_RAM_GB}"
export FREECOMPUTE_VM_STORAGEGB="${FREECOMPUTE_VM_STORAGEGB:-$VM_DISK_GB}"
export FREECOMPUTE_VM_GPU_ENABLED="${FREECOMPUTE_VM_GPU_ENABLED:-false}"
export FREECOMPUTE_VM_ENABLE_WEBRTC="${FREECOMPUTE_VM_ENABLE_WEBRTC:-true}"
export FREECOMPUTE_VM_ENABLE_GAMING="${FREECOMPUTE_VM_ENABLE_GAMING:-true}"
export FREECOMPUTE_VM_AUDIO="${FREECOMPUTE_VM_AUDIO:-true}"

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
