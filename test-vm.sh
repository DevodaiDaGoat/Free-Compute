#!/bin/bash
#
# test-vm.sh — Build the VM agent and exercise it safely.
#
#   1. Build the vm-setup binary.
#   2. Run --dry-run (must exit 0, no KVM/launch).
#   3. Run it live in the background pointed at the gateway.
#
# Safe by design: dry-run never connects or launches; the live run is
# backgrounded and killed at the end of the script (or on Ctrl-C).

set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
AGENT_DIR="$ROOT/host-agent"
BIN="$(mktemp 2>/dev/null || echo ${TMPDIR:-/tmp}/freecompute-vm-setup-XXXXXX)"

GATEWAY_URL="${FREECOMPUTE_GATEWAY_URL:-http://localhost:8080}"
TOKEN="${FREECOMPUTE_AGENT_TOKEN:-dev-token}"

echo "=== Building vm-setup binary ==="
( cd "$AGENT_DIR" && go build -buildvcs=false -o "$BIN" ./cmd/vm-setup )
echo "  built: $BIN"

echo ""
echo "=== Dry run (must exit 0, no KVM/launch) ==="
"$BIN" --dry-run
DRY_RC=$?
if [ "$DRY_RC" -ne 0 ]; then
  echo "ERROR: dry-run exited with code $DRY_RC" >&2
  exit 1
fi
echo "  dry-run OK (exit 0)"

echo ""
echo "=== Live run in background (gateway=$GATEWAY_URL) ==="
FREECOMPUTE_GATEWAY_URL="$GATEWAY_URL" \
FREECOMPUTE_AGENT_TOKEN="$TOKEN" \
FREECOMPUTE_VM_ID="test-vm-1" \
FREECOMPUTE_VM_REGION="local" \
FREECOMPUTE_VM_STORAGEGB="10" \
  "$BIN" &
AGENT_PID=$!
echo "  vm-agent started (PID $AGENT_PID)"

cleanup() {
  echo ""
  echo "=== Stopping vm-agent (PID $AGENT_PID) ==="
  kill "$AGENT_PID" 2>/dev/null || true
  wait "$AGENT_PID" 2>/dev/null || true
  echo "done."
}
trap cleanup EXIT INT TERM

# Give it a few seconds to register + report, then exit cleanly.
sleep 5
echo "  ran for 5s; see logs above. Stopping."
