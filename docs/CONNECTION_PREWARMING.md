# Connection Prewarming & Predictive Loading

## Overview

Connection prewarming anticipates user actions and establishes transport-layer connections *before* they are needed, eliminating DNS resolution, TCP/TLS handshake, and WebSocket upgrade latency from the critical path.

## Architecture

```
User intent (mouse hover, navigation) 
    │
    ▼
Predictor ──► Prewarm Manager ──► Transport Pool
    │                                  │
    │                           ┌──────┼──────┐
    │                           ▼      ▼      ▼
    │                       TCP     TLS    WebSocket
    │                       warm    warm   upgrade
    │
    └── Feedback loop (hit/miss → ML model)
```

## Prewarm Strategies

### 1. Hover-based Preconnect
- Detect `mouseenter` / `focusin` on navigation elements
- Initiate `preconnect` via `<link rel="preconnect">` or `fetch()` with `mode: 'navigate'`
- TTL: 10s after `mouseleave` / `blur`

### 2. Viewport Predictive Warmup
- IntersectionObserver on route cards / app icons
- When element is within 200px of viewport, warm connection to its target
- Uses `PerformanceResourceTiming` to learn viewport thresholds per user

### 3. Session Affinity Warmup
- After session creation, prewarm 2-3 adjacent routes (e.g., after `/sessions/` → warm `/input/`, `/audio/`, `/clipboard/`)
- Reduces perceived latency for multi-protocol sessions by ~300-800ms

### 4. Idle-time Prefetch
- During browser idle periods (`requestIdleCallback`), prefetch known-static resources
- API responses that change infrequently (capabilities, route list)

## Frontend Implementation

### Prewarm Hook (`usePrewarmConnection`)

```typescript
function usePrewarmConnection(url: string, options?: {
  type: 'preconnect' | 'prefetch' | 'prerender';
  ttl?: number;
}) {
  // Creates hidden <link> or sends early fetch
  // Manages TTL and cleanup
  // Returns { warm, elapsed }
}
```

### Connection Warmup Flow

```typescript
const WARMUP_PAYLOAD = new Uint8Array([0x01]); // lightweight keepalive

async function warmConnection(url: string): Promise<WebSocket> {
  const ws = new WebSocket(url);
  await new Promise((r) => (ws.onopen = r));
  ws.send(WARMUP_PAYLOAD); // establish full duplex
  return ws;
}
```

## Gateway-side Prewarm Endpoint

```
GET /prewarm
  → 200 OK (establishes TCP + TLS, upgrades to HTTP/2 if supported)
  → Connection held open for 15s with periodic PING frames
  → Client uses same socket for subsequent real request
```

## Metrics

| Metric | Target | How |
|--------|--------|-----|
| Prewarm hit rate | >70% | Predictor accuracy |
| Latency saved per hit | 150-800ms | Navigation timing API |
| Warm connection TTL | 15s | Gateway configurable |
| Memory per warm conn | ~4KB | Minimal socket state |

## Implementation Phases

1. **Phase 1 (Current)** — Static `<link rel="preconnect">` in layout
2. **Phase 2** — Hover-based preconnect for navigation elements
3. **Phase 3** — Viewport-based predictive warmup with IntersectionObserver
4. **Phase 4** — ML-based predictor using user navigation patterns
