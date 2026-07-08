# Error Recovery & Fault Tolerance

## Overview

A layered error recovery system that gracefully handles connection drops, host failures, network degradation, and gateway crashes — maintaining session continuity with minimal user impact.

## Recovery Layers

```
Layer 0: Application retry (client-side)
Layer 1: Transport reconnection (WebSocket / WebRTC ICE restart)
Layer 2: Session recovery (checkpoint restore)
Layer 3: Host failover (migrate to different host-agent)
Layer 4: Gateway failover (mesh redirect to healthy gateway)
```

## Layer 0 — Application Retry

For HTTP API calls, the frontend uses exponential backoff with jitter:

```typescript
async function fetchWithRecovery<T>(
  url: string,
  opts: RequestInit,
  maxRetries = 3,
): Promise<T> {
  for (let i = 0; i <= maxRetries; i++) {
    try {
      const resp = await fetch(url, {
        ...opts,
        signal: AbortSignal.timeout(5000),
      });
      if (!resp.ok) throw new GatewayError(resp.status);
      return resp.json();
    } catch (err) {
      if (i === maxRetries) throw err;
      const delay = Math.min(1000 * Math.pow(2, i) + Math.random() * 200, 8000);
      await new Promise(r => setTimeout(r, delay));
    }
  }
  throw new Error('unreachable');
}
```

| Scenario | Retry Strategy |
|----------|---------------|
| 429 Too Many Requests | Retry after `Retry-After` header |
| 502/503/504 Gateway Error | Retry 3x with backoff |
| Network offline | Wait for `online` event |
| DNS failure | Retry with `window.setTimeout` exponential |

## Layer 1 — Transport Reconnection

### WebSocket Reconnect

```typescript
class ResilientWebSocket {
  private ws: WebSocket | null = null;
  private retries = 0;
  private maxRetries = 10;

  connect() {
    this.ws = new WebSocket(this.url);
    this.ws.onclose = (e) => {
      if (e.code !== 1000 && this.retries < this.maxRetries) {
        const delay = Math.min(1000 * Math.pow(1.5, this.retries), 10000);
        this.retries++;
        setTimeout(() => this.connect(), delay);
      }
    };
    this.ws.onopen = () => { this.retries = 0; };
  }
}
```

### WebRTC ICE Restart

When the WebRTC connection degrades:

```typescript
async function restartIce(pc: RTCPeerConnection): Promise<void> {
  const offer = await pc.createOffer({ iceRestart: true });
  await pc.setLocalDescription(offer);
  // Send offer to remote via signaling channel
  await signaling.send({ type: 'ice-restart', sdp: offer });
}
```

Triggers:
- `iceconnectionstatechange` → `disconnected` or `failed`
- No media frames received for 3 seconds
- `RTCPeerConnection.getStats()` shows 10%+ packet loss

### Connection Health Monitor

```typescript
class ConnectionHealthMonitor {
  private pings: number[] = [];
  private checkInterval = 2000;

  async check(): Promise<Health> {
    const start = performance.now();
    try {
      await this.conn.ping();
      const rtt = performance.now() - start;
      this.pings.push(rtt);
      if (this.pings.length > 20) this.pings.shift();
      return this.assess(rtt);
    } catch {
      return 'dead';
    }
  }

  private assess(rtt: number): Health {
    const avg = this.pings.reduce((a, b) => a + b, 0) / this.pings.length;
    if (avg > 500) return 'degraded';
    if (this.pings.filter(p => p > 1000).length > 3) return 'failing';
    return 'healthy';
  }
}
```

## Layer 2 — Session Recovery

When transport reconnection succeeds but the session state is stale:

1. Client sends `GET /sessions/{id}/state` with last-known sequence number
2. Gateway returns all state changes since that sequence (or full state if gap too large)
3. Client replays changes to restore: cursor position, open windows, clipboard content
4. Media pipeline is restarted with previous codec/resolution config

## Layer 3 — Host Failover

If the host-agent becomes unreachable:

```
Gateway detects agent disconnect
    │
    ├── Mark route unavailable
    ├── Queue pending requests
    │
    ├── Idle session?
    │   ├── Yes → Wait for agent re-registration (up to 30s)
    │   └── No  → Find replacement host-agent
    │       ├── Check resource pool for matching capacity
    │       ├── Assign new host
    │       ├── Restore latest checkpoint
    │       └── Notify client of migration
    │
    └── Send session-migration event to client
```

Session migration is transparent when using connection fusion — the single QUIC connection survives the host swap.

## Layer 4 — Gateway Failover

With mesh routing, a client can fail over to a different gateway:

```
Client (Tokyo) → Gateway A (latency spike)
    │
    ├── Client detects >500ms RTT for 5s
    ├── Client queries mesh DNS for alternative
    ├── Client receives Gateway B (Seoul)
    ├── QUIC connection migration to Gateway B
    └── Gateway B fetches session state from Gateway A
        via mesh Redis
```

## Testing Recovery

| Scenario | Test | Expected |
|----------|------|----------|
| WiFi drop | Kill WiFi for 10s | Auto-reconnect within 3s |
| Gateway crash | Stop gateway process | Client fails over in <5s |
| Agent death | Kill host-agent | Session migrates in <10s |
| Packet loss | 20% artificial loss | Codec downgrade, FEC enabled |
| Network swap | WiFi → cellular | QUIC migration, <100ms gap |

## Implementation Phases

1. **Phase 1** — Application retry with backoff (current frontend SSE)
2. **Phase 2** — WebSocket + WebRTC reconnection with health monitoring
3. **Phase 3** — Session checkpoint restore on reconnect
4. **Phase 4** — Host failover + gateway mesh failover
