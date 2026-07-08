#!/bin/bash

# FreeCompute Backend Startup Script
# Starts gateway + host agent + VM setup agent with universal proxy

set -e

ROOT="$(cd "$(dirname "$0")" && pwd)"
echo "=== FreeCompute Backend ==="
echo "Root: $ROOT"

# ------------------------------------------------------------------
# System tuning (requires root; best-effort otherwise)
# ------------------------------------------------------------------
if [ "$(id -u)" -eq 0 ]; then
    sysctl -w net.core.rmem_max=16777216 2>/dev/null || true
    sysctl -w net.core.wmem_max=16777216 2>/dev/null || true
    sysctl -w net.ipv4.tcp_rmem='4096 87380 16777216' 2>/dev/null || true
    sysctl -w net.ipv4.tcp_wmem='4096 16384 16777216' 2>/dev/null || true
    sysctl -w net.ipv4.tcp_congestion_control=bbr 2>/dev/null || true
    sysctl -w net.ipv4.tcp_slow_start_after_idle=0 2>/dev/null || true
    sysctl -w net.core.netdev_max_backlog=5000 2>/dev/null || true
    sysctl -w net.core.somaxconn=65535 2>/dev/null || true
    sysctl -w net.ipv4.tcp_max_syn_backlog=4096 2>/dev/null || true
    sysctl -w net.ipv4.tcp_fin_timeout=10 2>/dev/null || true
    sysctl -w net.ipv4.tcp_tw_reuse=1 2>/dev/null || true
    sysctl -w net.core.rmem_default=262144 2>/dev/null || true
    sysctl -w net.core.wmem_default=262144 2>/dev/null || true
fi

# Aggressive file descriptor limit for high connection counts
ulimit -n 1000000 2>/dev/null || true

# ------------------------------------------------------------------
# Gateway
# ------------------------------------------------------------------
export FREECOMPUTE_GATEWAY_ADDR="${FREECOMPUTE_GATEWAY_ADDR:-:8080}"
export FREECOMPUTE_TUNNEL_TOKEN="${FREECOMPUTE_TUNNEL_TOKEN:-dev-token}"

# Tunnel routes - configure for universal proxy
export FREECOMPUTE_TUNNEL_ROUTES="${FREECOMPUTE_TUNNEL_ROUTES:-[
  {\"id\":\"web\",\"protocol\":\"http\",\"target\":\"http://localhost:3000\"},
  {\"id\":\"api\",\"protocol\":\"http\",\"target\":\"http://localhost:8081\"},
  {\"id\":\"vm-http\",\"protocol\":\"http\",\"target\":\"http://localhost:8082\"},
  {\"id\":\"vm-ws\",\"protocol\":\"websocket\",\"target\":\"http://localhost:8082\"},
  {"id":"vm-ssh","protocol":"ssh","target":"agent"},
  {\"id\":\"vm-tcp\",\"protocol\":\"tcp\",\"target\":\"agent\"},
  {\"id\":\"vm-udp\",\"protocol\":\"udp\",\"target\":\"localhost:30000\"},
  {\"id\":\"rtc\",\"protocol\":\"webrtc\",\"listen\":\"0.0.0.0:8443\"},
  {\"id\":\"p2p-signal\",\"protocol\":\"p2p\",\"target\":\"\"},
  {\"id\":\"game-udp\",\"protocol\":\"udp\",\"listen\":\"0.0.0.0:30001\",\"target\":\"localhost:30001\"}
]}"

# Performance timeouts (optimized for BunnyCDN)
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

# TCP/UDP performance tuning
export FREECOMPUTE_TCP_CC_ALGO="${FREECOMPUTE_TCP_CC_ALGO:-auto}"
export FREECOMPUTE_TCP_BUFFER_SIZE="${FREECOMPUTE_TCP_BUFFER_SIZE:-8388608}"
export FREECOMPUTE_UDP_BUFFER_SIZE="${FREECOMPUTE_UDP_BUFFER_SIZE:-16777216}"

# Database
export FREECOMPUTE_DB_PATH="${FREECOMPUTE_DB_PATH:-/tmp/freecompute.db}"

# Optional LLM moderation (empty = heuristic only)
export FREECOMPUTE_MODERATION_LLM_URL="${FREECOMPUTE_MODERATION_LLM_URL:-}"
export FREECOMPUTE_MODERATION_LLM_KEY="${FREECOMPUTE_MODERATION_LLM_KEY:-}"

# DNS cache tuning
export FREECOMPUTE_DNS_TTL_SECONDS="${FREECOMPUTE_DNS_TTL_SECONDS:-600}"
export FREECOMPUTE_DNS_MAX_ENTRIES="${FREECOMPUTE_DNS_MAX_ENTRIES:-4096}"

# BunnyCDN edge config (override with actual values in production)
export FREECOMPUTE_CDN_HOSTNAME="${FREECOMPUTE_CDN_HOSTNAME:-cdn.freecompute.io}"
export FREECOMPUTE_EDGE_HOSTNAME="${FREECOMPUTE_EDGE_HOSTNAME:-edge.freecompute.io}"
export FREECOMPUTE_API_HOSTNAME="${FREECOMPUTE_API_HOSTNAME:-api.freecompute.io}"

# Go runtime tuning for throughput
export GOMAXPROCS="${GOMAXPROCS:-$(nproc)}"
export GOGC="${GOGC:-200}"
export GOMEMLIMIT="${GOMEMLIMIT:-4GiB}"

# ------------------------------------------------------------------
# Host Agent
# ------------------------------------------------------------------
export FREECOMPUTE_AGENT_GATEWAY_URL="${FREECOMPUTE_AGENT_GATEWAY_URL:-http://localhost:8080}"
export FREECOMPUTE_AGENT_TOKEN="${FREECOMPUTE_AGENT_TOKEN:-$FREECOMPUTE_TUNNEL_TOKEN}"
export FREECOMPUTE_AGENT_ROUTES="${FREECOMPUTE_AGENT_ROUTES:-[
  {\"id\":\"vm-ssh\",\"target\":\"localhost:22\"},
  {\"id\":\"vm-tcp\",\"target\":\"localhost:8082\"}
]}"
export FREECOMPUTE_AGENT_DIAL_SECONDS="${FREECOMPUTE_AGENT_DIAL_SECONDS:-5}"
export FREECOMPUTE_AGENT_RECONNECT_SECONDS="${FREECOMPUTE_AGENT_RECONNECT_SECONDS:-1}"
export FREECOMPUTE_AGENT_INSECURE_SKIP_TLS="${FREECOMPUTE_AGENT_INSECURE_SKIP_TLS:-0}"

# ------------------------------------------------------------------
# Cleanup trap
# ------------------------------------------------------------------
cleanup() {
    echo "Stopping services..."
    pkill -f "gateway" 2>/dev/null || true
    pkill -f "host-agent" 2>/dev/null || true
    pkill -f "vm-setup" 2>/dev/null || true
    sleep 1
    echo "All services stopped."
}
trap cleanup EXIT INT TERM

# ------------------------------------------------------------------
# Build gateway + host-agent in parallel
# ------------------------------------------------------------------
echo ""
echo "[1] Building gateway + host-agent in parallel..."
(cd "$ROOT/apps/gateway" && go build -buildvcs=false -o /tmp/freecompute-gateway ./cmd/gateway) &
(cd "$ROOT/host-agent" && go build -buildvcs=false -o /tmp/freecompute-host-agent ./cmd/host-agent) &
wait

# ------------------------------------------------------------------
# Build VM setup agent
# ------------------------------------------------------------------
echo "[2] Building vm-setup agent..."
(cd "$ROOT/host-agent" && go build -buildvcs=false -o /tmp/freecompute-vm-setup ./cmd/vm-setup) || echo "  vm-setup build skipped (optional)"

# ------------------------------------------------------------------
# Start Gateway
# ------------------------------------------------------------------
echo "[3] Starting gateway on $FREECOMPUTE_GATEWAY_ADDR..."
/tmp/freecompute-gateway &
GATEWAY_PID=$!
sleep 0.2

# Health check
if curl -sf "http://localhost${FREECOMPUTE_GATEWAY_ADDR}/healthz" > /dev/null 2>&1; then
    echo "  Gateway healthy (PID $GATEWAY_PID)"
else
    echo "  WARNING: gateway health check failed, checking again..."
    sleep 1
    if curl -sf "http://localhost${FREECOMPUTE_GATEWAY_ADDR}/healthz" > /dev/null 2>&1; then
        echo "  Gateway healthy (PID $GATEWAY_PID)"
    else
        echo "  ERROR: gateway health check failed"
        exit 1
    fi
fi

# ------------------------------------------------------------------
# Start Host Agent
# ------------------------------------------------------------------
echo "[4] Starting host-agent..."
/tmp/freecompute-host-agent &
AGENT_PID=$!
echo "  Agent started (PID $AGENT_PID)"

# ------------------------------------------------------------------
# Test endpoints
# ------------------------------------------------------------------
echo ""
echo "[5] Testing endpoints..."

echo -n "  Health: "; curl -sf "http://localhost${FREECOMPUTE_GATEWAY_ADDR}/healthz" && echo " OK" || echo " FAIL"

echo -n "  Capabilities: "; curl -sf "http://localhost${FREECOMPUTE_GATEWAY_ADDR}/capabilities" | head -c 100 && echo "..." || echo " FAIL"

echo -n "  Auth register: "
REG_RESP=$(curl -sf -X POST "http://localhost${FREECOMPUTE_GATEWAY_ADDR}/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"email":"demo@freecompute.io","password":"demo1234","displayName":"Demo User"}') && echo " OK" || echo " FAIL"

echo -n "  Auth login: "
TOKEN=$(curl -sf -X POST "http://localhost${FREECOMPUTE_GATEWAY_ADDR}/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"demo@freecompute.io","password":"demo1234"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['tokens']['accessToken'])" 2>/dev/null) && echo " OK" || echo " FAIL"

echo -n "  Storage upload: "
if [ -n "$TOKEN" ]; then
  echo "test content" > /tmp/test-upload.txt
  curl -sf -X POST "http://localhost${FREECOMPUTE_GATEWAY_ADDR}/storage/upload?userId=$TOKEN&path=test.txt" \
    -H "Content-Type: text/plain" \
    --data-binary @/tmp/test-upload.txt && echo " OK" || echo " FAIL"
fi

echo -n "  Proxy: "; curl -sf "http://localhost${FREECOMPUTE_GATEWAY_ADDR}/proxy/web/" && echo " OK" || echo " FAIL"

echo -n "  Tailscale hosts: "; curl -sf "http://localhost${FREECOMPUTE_GATEWAY_ADDR}/tailscale/hosts" | head -c 100 && echo "..." || echo " FAIL"

echo -n "  WebRTC session: "; curl -sf -X POST "http://localhost${FREECOMPUTE_GATEWAY_ADDR}/webrtc/" \
  -H "Content-Type: application/json" \
  -d '{"clientId":"test","preset":"safe"}' | head -c 80 && echo "..." || echo " FAIL"

# ------------------------------------------------------------------
# Summary
# ------------------------------------------------------------------
echo ""
echo "=== All services running ==="
echo ""
echo "Gateway:      http://localhost${FREECOMPUTE_GATEWAY_ADDR}"
echo "  Health:     http://localhost${FREECOMPUTE_GATEWAY_ADDR}/healthz"
echo "  Capabilities: http://localhost${FREECOMPUTE_GATEWAY_ADDR}/capabilities"
echo "  Routes:     http://localhost${FREECOMPUTE_GATEWAY_ADDR}/routes"
echo ""
echo "Host Agent:   PID $AGENT_PID (connected to gateway)"
echo ""
echo "Auth endpoints:"
echo "  POST /auth/register  | POST /auth/login"
echo "  GET  /auth/profile   | POST /auth/tailscale-ip"
echo ""
echo "Storage endpoints:"
echo "  GET    /storage/list   | POST /storage/upload"
echo "  GET    /storage/download  | DELETE /storage/delete"
echo ""
echo "Universal proxy endpoints:"
echo "  HTTP/WS:    /proxy/{routeID}/{path}"
echo "  CONNECT:    /connect/{routeID}"
echo "  WebSocket:  /ws/{routeID}"
echo "  SSH:        /ssh/{routeID}"
echo "  Agent:      /agent/{routeID}"
echo "  Signal:     /signal/{routeID}/rooms/{roomID}"
echo "  WebRTC:     POST /webrtc/"
echo "  UDP:        Direct UDP listener on configure routes"
echo "  P2P:        /signal/{routeID}/rooms/{roomID}"
echo ""
echo "Sessions / Streaming:"
echo "  POST /sessions/    | DELETE /sessions/{id}"
echo "  POST /gaming/{id}  | GET /gaming/{id}"
echo "  POST /input/{id}   | POST /audio/{id}"
echo "  POST /transfer/    | GET/POST /clipboard/{id}"
echo "  POST /media/{id}   | POST /keyframe/{id}"
echo ""
echo "Tailscale:"
echo "  POST /tailscale/register | GET /tailscale/hosts"
echo "  POST /tailscale/user     | GET /tailscale/user?userId=..."
echo "  POST /tailscale/proxy    (proxy fallback)"
echo ""
echo "WebOS (frontend):"
echo "  http://localhost:3000         - Console Dashboard"
echo "  http://localhost:3000/webos   - WebOS Desktop"
echo ""
echo "BunnyCDN config:"
echo "  CDN:  ${FREECOMPUTE_CDN_HOSTNAME}"
echo "  Edge: ${FREECOMPUTE_EDGE_HOSTNAME}"
echo "  API:  ${FREECOMPUTE_API_HOSTNAME}"
echo ""
echo "Storage: 100GB per user (local: /tmp/freecompute-storage/)"
echo ""
echo "Press Ctrl+C to stop all services."
echo ""

wait
