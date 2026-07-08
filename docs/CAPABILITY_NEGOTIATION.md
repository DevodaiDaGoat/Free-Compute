# Capability Negotiation Protocol

## Overview

Capability negotiation allows clients and gateways to agree on the optimal set of protocols, codecs, transports, and features — adapting to network conditions, device capabilities, and user preferences in real-time.

## Protocol Flow

```
Client                          Gateway
  │                                │
  │── GET /capabilities ──────────►│
  │◄── 200 { protocols, codecs,   │
  │       transports, features,    │
  │       version, limits }        │
  │                                │
  │── POST /sessions/ ────────────►│
  │   { sessionConfig: {           │
  │     selectedProtocol,          │
  │     codec,                     │
  │     transport,                 │
  │     features: [...]            │
  │   }}                           │
  │                                │
  │── Server-side negotiation ────►│
  │   (intersect capabilities,     │
  │    apply preferences,          │
  │    check resources)            │
  │                                │
  │◄── 201 { sessionId,            │
  │       negotiatedConfig: {       │
  │         videoCodec: 'h265',     │
  │         transport: 'webrtc',   │
  │         features: [...]         │
  │       }}                        │
```

## Capabilities Response Schema

```json
{
  "protocols": ["http", "https", "tcp", "udp", "websocket", "webrtc", "p2p", "ssh"],
  "transports": ["webrtc", "websocket", "quic", "webtransport"],
  "videoCodecs": [
    { "name": "h264", "profiles": ["baseline", "main", "high"], "hwEnc": true, "maxRes": "4K", "maxFps": 240 },
    { "name": "h265", "profiles": ["main", "main10"], "hwEnc": true, "maxRes": "8K", "maxFps": 120 },
    { "name": "av1", "profiles": ["main"], "hwEnc": true, "maxRes": "8K", "maxFps": 60 },
    { "name": "vp8", "profiles": ["0"], "hwEnc": false, "maxRes": "4K", "maxFps": 60 },
    { "name": "vp9", "profiles": ["0", "2"], "hwEnc": false, "maxRes": "4K", "maxFps": 60 }
  ],
  "audioCodecs": [
    { "name": "opus", "bitrate": [6, 510], "channels": 2, "sampleRates": [8000, 48000] },
    { "name": "aac", "bitrate": [8, 320], "channels": 2, "sampleRates": [8000, 96000] }
  ],
  "features": {
    "clipboardSync": true,
    "fileTransfer": true,
    "sessionRecording": true,
    "multiMonitor": true,
    "controllerRumble": true,
    "hdr": true,
    "adaptiveBitrate": true,
    "connectionFusion": true,
    "quicMigration": true,
    "meshRouting": false
  },
  "limits": {
    "maxSessionsPerUser": 5,
    "maxSessionDuration": 28800,
    "maxResolution": "8K",
    "maxFps": 240,
    "maxBitrate": 200000
  },
  "version": "1.0.0"
}
```

## Client-side Negotiation

```typescript
function negotiateConfig(
  caps: Capabilities,
  prefs: ConnectionConfig,
): NegotiatedConfig {
  const videoCodec = selectBestVideoCodec(caps.videoCodecs, prefs);
  const transport = selectBestTransport(caps.transports, prefs);
  const features = intersectFeatures(caps.features, prefs);

  return { videoCodec, transport, features, ... };
}
```

### Preference Scoring

Each option is scored on a 100-point scale:

| Factor | Weight | Description |
|--------|--------|-------------|
| Preference match | 40 | User's explicit choice |
| Hardware support | 25 | GPU encoder available |
| Network conditions | 20 | Bandwidth, packet loss |
| Latency budget | 15 | User's latency target |

## Dynamic Re-negotiation

When network conditions change, the system can re-negotiate mid-session:

```
RTCStatsCollector
    │ (detects packet loss > 5%)
    ▼
NegotiationTrigger
    │
    ├── Downgrade codec (H.265 → H.264)
    ├── Reduce resolution (1440p → 1080p)
    ├── Lower bitrate (20Mbps → 8Mbps)
    └── Switch transport (WebRTC → WebSocket)
```

```typescript
interface RenegotiationEvent {
  reason: 'congestion' | 'packet-loss' | 'bandwidth-drop' | 'quality-improvement';
  suggestedChanges: Partial<NegotiatedConfig>;
  autoApply: boolean;
}
```

## Versioning & Compatibility

Capability responses include a semver version. Clients cache capabilities and use `If-None-Match` / ETag headers to minimize polling:

```
GET /capabilities
If-None-Match: "v1.0.0-abc123"
    │
    ├── 304 Not Modified → use cached
    └── 200 + new ETag → update cache
```

## Implementation Phases

1. **Phase 1** — Static capabilities response (current)
2. **Phase 2** — Client-side preference scoring + negotiation
3. **Phase 3** — Dynamic re-negotiation based on network stats
4. **Phase 4** — ETag-based caching + delta updates
