# FreeCompute Backend Implementation

This document describes the complete backend implementation for remote access, game streaming, and universal proxy functionality.

## Architecture Overview

The backend consists of several key components:

### 1. Gateway Service (`apps/gateway`)
The main gateway service that handles all incoming connections and coordinates remote sessions.

**Key Features:**
- Universal proxy supporting HTTP/HTTPS, TCP, UDP, SSH, WebSocket, WebRTC, and P2P
- WebRTC streaming with H.264/H.265/AV1 codec support
- Session management for desktop, gaming, and remote support modes
- Input handling for keyboard, mouse, touch, and gamepads
- Audio streaming with Opus/AAC codecs
- File transfer and clipboard synchronization
- Gaming mode with controller support and performance optimization
- GPU-aware scheduling and resource allocation

**Components:**
- `internal/tunnel/` - Universal proxy implementation
- `internal/webrtc/` - WebRTC streaming service
- `internal/session/` - Session management and scheduling
- `internal/input/` - Input device handling
- `internal/audio/` - Audio streaming
- `internal/transfer/` - File transfer and clipboard
- `internal/gaming/` - Gaming mode optimization

### 2. Host Agent (`host-agent`)
Runs on remote machines to establish outbound tunnels and handle VM/desktop operations.

**Key Features:**
- Reverse TCP tunneling
- VM/desktop integration
- GPU encoder initialization
- Screen capture and streaming
- Audio capture
- Controller input handling
- Metrics reporting

## Configuration

### Gateway Environment Variables

```bash
# Basic Configuration
FREECOMPUTE_GATEWAY_ADDR=":8080"
FREECOMPUTE_TUNNEL_TOKEN="your-token"

# Proxy Configuration
FREECOMPUTE_TUNNEL_ROUTES='[...]'
FREECOMPUTE_PROXY_MAX_IDLE_CONNS="1024"
FREECOMPUTE_PROXY_MAX_IDLE_CONNS_PER_HOST="128"

# Timeouts
FREECOMPUTE_GATEWAY_SHUTDOWN_SECONDS="30"
FREECOMPUTE_TUNNEL_DIAL_SECONDS="10"
FREECOMPUTE_TUNNEL_AGENT_WAIT_SECONDS="15"
```

### Host Agent Environment Variables

```bash
FREECOMPUTE_AGENT_GATEWAY_URL="http://gateway:8080"
FREECOMPUTE_AGENT_TOKEN="your-token"
FREECOMPUTE_AGENT_ROUTES='[...]'
FREECOMPUTE_AGENT_DIAL_SECONDS="10"
FREECOMPUTE_AGENT_RECONNECT_SECONDS="1"
```

## API Endpoints

### Health & Capabilities
- `GET /healthz` - Health check
- `GET /capabilities` - Gateway capabilities and supported protocols
- `GET /routes` - Available proxy routes

### Session Management
- `POST /sessions/` - Create remote session
- `GET /sessions/{id}` - Get session details
- `DELETE /sessions/{id}` - End session

### WebRTC Streaming
- `POST /webrtc/` - Create WebRTC session
- `GET /signal/{id}` - WebRTC signaling (WebSocket)

### Gaming
- `POST /gaming/{id}` - Create gaming session
- `PUT /gaming/{id}` - Update gaming state
- `GET /gaming/{id}` - Get gaming state

### Input
- `POST /input/{id}` - Send input events

### Audio
- `POST /audio/{id}` - Send audio frames

### File Transfer
- `POST /transfer/` - Create transfer
- `PUT /transfer/` - Send chunk

### Clipboard
- `POST /clipboard/{id}` - Write clipboard
- `GET /clipboard/{id}` - Read clipboard

### Proxy Routes
- `GET /proxy/{route}/{path}` - HTTP/HTTPS proxy
- `CONNECT /connect/{route}` - HTTP CONNECT tunnel
- `GET /ws/{route}` - WebSocket tunnel
- `GET /agent/{route}` - Agent tunnel

## BunnyCDN Optimization

The implementation is optimized for BunnyCDN deployment with:

1. **Static Asset Caching**
   - Long TTL (1 year) for immutable assets
   - Cache rules for JS, CSS, images, fonts

2. **Streaming Endpoint Bypass**
   - No caching for real-time endpoints
   - Preserved WebSocket upgrade headers
   - Disabled buffering for streaming

3. **Edge Rules**
   - Nearest region routing for signaling
   - Short-lived connection tokens (5 min)
   - Protocol-aware routing

4. **Performance**
   - HTTP/2 and HTTP/3 support
   - Brotli compression
   - Modern TLS configuration

5. **Security**
   - DDoS protection
   - Rate limiting
   - CORS configuration

## Session Types

### Desktop Session
- **Use:** General cloud desktop access
- **Scheduling:** Stability, compatibility, fair cost
- **Features:** Browser-based, clipboard sync, file transfer

### Gaming Session
- **Use:** Game streaming with low latency
- **Scheduling:** Lowest latency, GPU priority, network quality
- **Features:** Controller support, rumble, performance optimization

### Remote Support Session
- **Use:** Temporary access to approved systems
- **Scheduling:** Approval required, audit logging
- **Features:** User approval, temporary links, session recording

### Host Session
- **Use:** Host maintenance and diagnostics
- **Scheduling:** Reserved for host operations
- **Features:** Maintenance tools, diagnostics

## Resource Classes

### Basic
- 1 Core
- Shared RAM
- No GPU

### Standard
- 2-4 Cores
- More RAM
- Optional GPU

### Gaming
- Dedicated GPU allocation
- Hardware encoding
- High-performance network

### Workstation
- High-performance systems
- Content creation optimized
- Development workloads

## Stream Presets

### Safe Mode
- Maximum compatibility
- Conservative bitrate
- Adaptive resolution
- Strong packet loss recovery

### Fast Mode
- Prioritize latency
- Hardware encoding
- Higher bitrate
- Fullscreen capture

## Supported Input Devices

- Keyboard
- Mouse
- Touch
- Xbox controllers
- PlayStation controllers
- Generic gamepads
- Racing wheels (future)
- HOTAS (future)
- VR controllers (future)

## Running the Backend

### Quick Start

```bash
# Start all services
./run-backend.sh
```

### Manual Start

```bash
# Start Gateway
cd apps/gateway
go run ./cmd/gateway

# Start Host Agent
cd host-agent
go run ./cmd/host-agent

# Start VM Setup
go run ./vm-setup.go
```

### Testing

```bash
# Health check
curl http://localhost:8080/healthz

# Capabilities
curl http://localhost:8080/capabilities

# Create WebRTC session
curl -X POST http://localhost:8080/webrtc/ \
  -H "Content-Type: application/json" \
  -d '{
    "clientId": "test-client",
    "preset": "fast",
    "videoCodecs": ["h264", "vp8"],
    "audioCodecs": ["opus"],
    "resolution": {"width": 1920, "height": 1080, "refreshRate": 60}
  }'

# Create remote session
curl -X POST http://localhost:8080/sessions/ \
  -H "Content-Type: application/json" \
  -d '{
    "userId": "user-123",
    "type": "gaming",
    "mode": "gaming",
    "resourceClass": "gaming",
    "streamPreset": "fast",
    "gpuRequired": true
  }'
```

## Performance Optimization

### BunnyCDN Configuration
See `apps/gateway/bunnycdn-config.json` for complete CDN configuration.

### GPU Acceleration
- Hardware encoding (NVENC, AMF, Quick Sync)
- GPU memory management
- Encoder utilization tracking

### Network Optimization
- Adaptive bitrate
- Packet loss recovery
- Latency monitoring
- Quality scoring

### Gaming Optimization
- Mode-specific settings (competitive, standard, casual, VR)
- Performance metrics tracking
- Automatic quality adjustment
- Controller latency optimization

## Security Features

- Token-based authentication
- Session expiration
- Audit logging
- Temporary access links
- User approval requirements
- Device verification
- Rate limiting
- DDoS protection

## Monitoring

### Metrics Tracked
- CPU/GPU utilization
- Memory usage
- Network throughput
- Stream quality
- Latency measurements
- Packet loss
- Active connections

### Health Checks
- Gateway health endpoint
- Agent heartbeat
- Session state monitoring
- Resource availability

## Future Enhancements

- H.265 and AV1 codec support
- VR streaming
- Racing wheel and HOTAS support
- Cloud save synchronization
- Game launcher integration
- Steam integration
- LAN tunneling
- Multiplayer optimization
- Persistent game storage
- VM snapshots