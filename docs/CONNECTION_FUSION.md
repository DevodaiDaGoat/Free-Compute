# Connection Fusion & Stream Coalescing

## Overview

Connection Fusion merges multiple logical streams (video, audio, input, clipboard, file transfer) into a single QUIC/WebTransport connection with independent stream priorities, eliminating the overhead of N separate connections and reducing head-of-line blocking.

## Problem

Current architecture opens separate connections per protocol:
- WebRTC for video/audio (1 connection)
- WebSocket for input events (1 connection)
- WebSocket for clipboard (1 connection)
- HTTP for file transfer (N connections)
- WebSocket for signaling (1 connection)

**Total: 4+ connections per session** → 4x TLS handshake overhead, 4x memory, complex state management.

## Solution: Unified Session Multiplexer

```
┌─────────────── Session Multiplexer ───────────────┐
│                                                    │
│   ┌──────┐  ┌──────┐  ┌──────┐  ┌──────┐  ┌──────┐│
│   │Video │  │Audio │  │Input │  │Clip  │  │File  ││
│   │Stream│  │Stream│  │Stream│  │Stream│  │Stream││
│   └──┬───┘  └──┬───┘  └──┬───┘  └──┬───┘  └──┬───┘│
│      │         │         │         │         │     │
│   ┌──▼─────────▼─────────▼─────────▼─────────▼──┐  │
│   │           QUIC / WebTransport                │  │
│   └──────────────────────────────────────────────┘  │
│                                                    │
└────────────────────────────────────────────────────┘
```

## QUIC Stream Priorities

| Stream | Priority | Loss Tolerance | Latency Budget |
|--------|----------|---------------|----------------|
| Video frames | 3 (medium) | High | 50ms |
| Audio | 1 (highest) | Low | 10ms |
| Input events | 1 (highest) | Very Low | 5ms |
| Clipboard | 4 (low) | None | 500ms |
| File transfer | 5 (lowest) | None | 5000ms |

## Implementation

### Frontend Session Multiplexer

```typescript
class SessionMultiplexer {
  private transport: WebTransport | WebSocket;
  private streams: Map<StreamID, Stream>;

  async createStream(priority: number, type: StreamType): Promise<Stream> {
    if (this.transport instanceof WebTransport) {
      return this.transport.createUnidirectionalStream();
    }
    return this.createWSSimplexStream(priority, type);
  }

  async sendControl(msg: ControlMessage): Promise<void> {
    const stream = await this.createStream(0, 'control');
    await stream.write(msg.encode());
  }
}
```

### Stream Types

```typescript
enum StreamType {
  Control = 0x00,
  Video    = 0x01,
  Audio    = 0x02,
  Input    = 0x03,
  Clipboard = 0x04,
  FileMeta  = 0x05,
  FileData  = 0x06,
  SessionState = 0x07,
}
```

### Gateway Multiplexer

```go
type SessionMultiplexer struct {
    conn     quic.Connection
    streams  map[StreamID]*quic.Stream
    priority map[StreamID]int
    mu       sync.Mutex
}

func (m *SessionMultiplexer) AcceptStream() (*Stream, error) {
    qs, err := m.conn.AcceptStream(context.Background())
    if err != nil { return nil, err }
    // Read first byte for stream type
    var typeByte [1]byte
    qs.Read(typeByte[:])
    return &Stream{qs, StreamType(typeByte[0])}, nil
}
```

## WebSocket Fallback

When QUIC/WebTransport is unavailable, streams are coalesced into a single WebSocket with a lightweight frame header:

```
┌─────┬──────┬──────────┬──────────┐
│Type │SeqNo │ Payload  │ Checksum │
│1byte│2bytes│ Variable │ 1byte    │
└─────┴──────┴──────────┴──────────┘
```

## Performance Impact

| Metric | Before (N connections) | After (Fusion) | Improvement |
|--------|----------------------|----------------|-------------|
| Handshake time | 4 × 150ms = 600ms | 1 × 100ms | 6x |
| Memory per session | ~128KB (4 conns) | ~32KB (1 conn) | 4x |
| Head-of-line blocking | Per-connection | Per-stream | Eliminated |
| File transfer overhead | HTTP headers | 3-byte frame header | ~50x |
| NAT state entries | 4 | 1 | 4x |

## Implementation Phases

1. **Phase 1** — Single WebSocket with frame multiplexing (no QUIC)
2. **Phase 2** — WebTransport with priority streams
3. **Phase 3** — QUIC connection migration support
4. **Phase 4** — Dynamic priority adjustment based on network conditions
