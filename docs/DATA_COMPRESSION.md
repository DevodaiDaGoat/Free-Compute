# Data Compression & Encoding Strategies

## Overview

Strategic compression at multiple layers reduces bandwidth usage by 40-80% without adding perceptible latency. Different data types use different compression strategies based on their latency and loss tolerance.

## Compression Layers

```
┌──────────────────────────────────────────┐
│            Application Layer             │
│  ┌─────────┐ ┌──────────┐ ┌──────────┐  │
│  │ Control  │ │ Metadata │ │  JSON    │  │
│  │  Msgs    │ │  (CBOR)  │ │ (Bro-   │  │
│  │ (no-     │ │          │ │  tli)   │  │
│  │  compress)│ │          │ │         │  │
│  └─────────┘ └──────────┘ └──────────┘  │
├──────────────────────────────────────────┤
│            Transport Layer               │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ │
│  │  QUIC    │ │WebSocket │ │   HTTP   │ │
│  │ (QPACK)  │ │(per-msg  │ │ (gzip/   │ │
│  │          │ │ compress)│ │  brotli) │ │
│  └──────────┘ └──────────┘ └──────────┘ │
├──────────────────────────────────────────┤
│               Payload Layer              │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ │
│  │  Video   │ │  Audio   │ │  Input   │ │
│  │ (codec)  │ │ (Opus)   │ │ (delta)  │ │
│  └──────────┘ └──────────┘ └──────────┘ │
└──────────────────────────────────────────┘
```

## Per-Data-Type Strategy

| Data Type | Strategy | Ratio | Latency Impact |
|-----------|----------|-------|---------------|
| Video frames | H.264/H.265/AV1 codec | 100:1 - 500:1 | <1ms (HW) |
| Audio (mic) | Opus @ 16-32kbps | 10:1 | 2.5ms (frame) |
| Audio (system) | Opus @ 64-128kbps | 6:1 | 10ms (frame) |
| Input events | Delta encoding | 100:1 | ~0ms |
| Clipboard text | Brotli (quality 4) | 3:1 - 10:1 | <1ms |
| Signaling JSON | CBOR (binary JSON) | 1.5:1 | ~0.2ms |
| File transfer | None (passthrough) | 1:1 | N/A |
| Desktop frame diff | Run-length + zlib | 10:1 - 50:1 | 2-5ms |
| Session checkpoints | zstd (level 3) | 3:1 - 8:1 | <5ms |

## Input Event Compression

Input events (mouse, keyboard, gamepad) are the most latency-sensitive data. They use delta encoding:

```protobuf
message InputFrame {
  uint32 sequence = 1;
  uint64 timestamp_us = 2;

  // Delta from previous frame
  sint32 delta_x = 3;    // mouse X delta
  sint32 delta_y = 4;    // mouse Y delta
  uint32 buttons = 5;    // bitmask of pressed buttons

  // Key events (only when state changes)
  repeated KeyEvent keys = 6;

  // Gamepad state (full state, not delta)
  repeated GamepadState gamepads = 7;
}
```

A typical mouse move at 1000Hz generates ~28 bytes/frame instead of ~200 bytes as JSON.

## Binary Protocol for Signaling

Replace JSON WebSocket signaling with CBOR (RFC 7049):

| Metric | JSON | CBOR | Benefit |
|--------|------|------|---------|
| SDP offer | 4200 bytes | 2800 bytes | 33% smaller |
| ICE candidate | 280 bytes | 140 bytes | 50% smaller |
| Parse time | ~0.5ms | ~0.05ms | 10x faster |
| Session config | 1200 bytes | 680 bytes | 43% smaller |

## WebSocket Message Compression

For WebSocket tunnels carrying binary tunnel data (not text signaling), disable per-message compression to save CPU:

```go
// tunnel/websocket.go
var compressor = flate.NewWriter(someWriter, flate.BestSpeed)

// Only compress signaling messages (type 0x00-0x0F)
// Never compress tunnel data (type 0x10+)
func shouldCompress(msgType byte) bool {
    return msgType < 0x10
}
```

For tunnel data where compression helps (e.g., desktop frame diffs), use zstd at level 1 (fastest):

```go
import "github.com/klauspost/compress/zstd"

var zstdEncoder, _ = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedFastest))
compressed := zstdEncoder.EncodeAll(plain, nil)
```

## HTTP Payload Compression

All JSON API responses are compressed with Brotli (preferred) or gzip (fallback):

```go
// gateway middleware
func compressionMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if strings.Contains(r.Header.Get("Accept-Encoding"), "br") {
            w.Header().Set("Content-Encoding", "br")
            w = &brotliWriter{ResponseWriter: w}
        } else if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
            w.Header().Set("Content-Encoding", "gzip")
            w = &gzipWriter{ResponseWriter: w}
        }
        next.ServeHTTP(w, r)
    })
}
```

## Adaptive Compression

The system dynamically adjusts compression based on available bandwidth:

```go
type AdaptiveCompressor struct {
    bandwidthEstimate int // bps
    packetLoss        float64
    currentLevel      CompressionLevel
}

func (a *AdaptiveCompressor) selectLevel() CompressionLevel {
    switch {
    case a.bandwidthEstimate < 5_000_000: // <5 Mbps
        return CompressionHigh     // sacrifice quality for bandwidth
    case a.packetLoss > 0.05:     // >5% loss
        return CompressionMedium   // reduce packet size
    default:
        return CompressionLow      // preserve quality
    }
}
```

## Performance Benchmarks

| Scenario | Uncompressed | Compressed | CPU Overhead |
|----------|-------------|------------|--------------|
| Session create (JSON → CBOR) | 1.2KB | 680B | +0.02ms |
| Input stream (1min @ 1000Hz) | 12MB | 168KB | +3% CPU |
| Desktop diff (1080p @ 30Hz) | 45 Mbps | 8 Mbps | +5% CPU |
| Signaling (1000 messages) | 4.2MB | 2.8MB | +0.5ms |
| Checkpoint upload | 8MB | 2.3MB | +8ms |

## Implementation Phases

1. **Phase 1** — HTTP Brotli compression for API responses
2. **Phase 2** — CBOR signaling protocol for WebSocket
3. **Phase 3** — Input delta encoding + protobuf
4. **Phase 4** — Adaptive compression based on bandwidth estimates
