#!/bin/bash
# FreeCompute Tunnels
#
# Tunnels are part of the gateway — they start automatically with:
#   ./start-website.sh
#
# The gateway handles all tunnel protocols:
#   HTTP, WebSocket, SSH, TCP, UDP, WebRTC, P2P signaling
#
# The host-agent (./start-vms.sh) connects to the gateway
# to tunnel VM traffic through the gateway.
#
# Usage:
#   ./start-website.sh   # Starts gateway (tunnels) + frontend
#   ./start-vms.sh       # Starts host-agent (connects VMs to gateway)

echo "Tunnels are built into the gateway."
echo ""
echo "To start everything:"
echo "  Terminal 1: ./start-website.sh    # gateway + frontend + tunnels"
echo "  Terminal 2: ./start-vms.sh        # host-agent (VM connectivity)"
echo ""
echo "The gateway listens on :8080 (configurable via FREECOMPUTE_GATEWAY_ADDR)"
echo "Tunnel routes are configured via FREECOMPUTE_TUNNEL_ROUTES env var."
