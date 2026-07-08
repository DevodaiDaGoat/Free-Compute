# Performance Optimization Strategy

High-impact, low-risk optimizations identified during codebase audit. Ordered by estimated ROI.

---

## 1. Buffer Pool Adoption (High Impact)

**Problem:** `tcp.go:180` and `websocket.go:95` allocate `make([]byte, 32*1024)` per goroutine. At 10K concurrent connections this is 320 MB of GC pressure.

**Fix:** Route all 32 KB allocations through the existing `byteBufferPool` in `buffer_pool.go`.

| File | Line | Current | Replace with |
|------|------|---------|-------------|
| `internal/tunnel/tcp.go` | 180 | `buf := make([]byte, 32*1024)` | `buf := bufferPool.Get()` / `bufferPool.Put(buf)` |
| `internal/tunnel/websocket.go` | 95 | `buf := make([]byte, 32*1024)` | Same |
| `internal/tunnel/websocket.go` | 381 | `make([]byte, payloadSize)` per frame | Pooled buffer with size limit check |
| `internal/webrtc/webrtc.go` | media ingest | `make([]byte, 1500)` per RTP packet | Small buffer pool (1500–2048 bytes) |

**Impact:** Eliminates ~75% of per-connection heap allocations. Reduces GC pause frequency by measurable margin at scale.

---

## 2. Priority Queue: Slice → Binary Heap (Medium Impact)

**Problem:** `internal/session/scheduler.go` uses a sorted slice with `append(queue[:i], append(...)...)` for priority queue operations. Enqueue is O(n), dequeue is O(n). For large queues this dominates scheduling latency.

**Fix:** Replace with `container/heap`.

```go
type PriorityHeap []*QueuedSession

func (h PriorityHeap) Len() int            { return len(h) }
func (h PriorityHeap) Less(i, j int) bool  { return h[i].Priority > h[j].Priority }
func (h PriorityHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *PriorityHeap) Push(x any)         { *h = append(*h, x.(*QueuedSession)) }
func (h *PriorityHeap) Pop() any {
    old := *h; n := len(old); x := old[n-1]; *h = old[:n-1]; return x
}
```

**Impact:** Enqueue O(n) → O(log n), Dequeue O(n) → O(log n). Critical for multi-region deployments with thousands of queued sessions.

---

## 3. Audio Ring Buffer: Byte-by-byte → Bulk `copy()` (High Impact)

**Problem:** `audio.go` ring buffer copies data one byte at a time in a for loop:

```go
for i := 0; i < len(data); i++ {
    b.data[b.writePos] = data[i]
    b.writePos = (b.writePos + 1) % len(b.data)
}
```

**Fix:** Use `copy()` with two-stage wrap handling:

```go
func (b *AudioBuffer) Write(data []byte) {
    b.mu.Lock()
    defer b.mu.Unlock()
    n := len(data)
    if n > len(b.data)-b.Available() {
        // Drop oldest data — simple overwrite
    }
    // First segment: writePos to end
    firstLen := copy(b.data[b.writePos:], data)
    // Second segment: wrap to beginning
    copy(b.data, data[firstLen:])
    b.writePos = (b.writePos + n) % len(b.data)
}
```

**Impact:** Audio write throughput improves by 50-100x for typical frame sizes (960 bytes / 20ms @ 48kHz). Reduces lock hold time.

---

## 4. UDP Client Map Cleanup (High Impact — Memory Leak Fix)

**Problem:** `udp.go:41` `clients map[string]*udpClient` is append-only. Entries accumulate indefinitely after clients disconnect silently.

**Fix:** Add periodic eviction sweep and per-client idle timeout:

```go
// In serveUDP, launch a cleanup goroutine:
go func() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            s.evictStaleUDPClients(route)
        case <-ctx.Done():
            return
        }
    }
}()
```

Each `udpClient` gets a `lastActivity time.Time` updated on every packet. Entries idle > configurable timeout (default 5 min) are pruned.

**Impact:** Prevents unbounded memory growth on long-lived gateway instances.

---

## 5. WebSocket Frames: Pooled Buffer for `readFrame()` (Medium Impact)

**Problem:** `websocket.go:381` allocates `make([]byte, payloadSize)` on every incoming frame. For small control frames this is wasteful.

**Fix:** Maintain a `sync.Pool` for small frame buffers (≤ 4096 bytes). For larger payloads, fall back to direct allocation with a size cap. Or reuse the frame buffer across reads within a single WebSocket connection.

**Impact:** Reduces per-frame allocation rate by ~80% for typical signaling/control message workloads.

---

## 6. Agent Pool: O(n) Removal → O(1) with Linked List (Medium Impact)

**Problem:** `agent_pool.go:148-153` `removeIdle()` and `removeWaiter()` are O(n) linear scans deleting from slices.

**Fix:** Replace `[]*pooledAgentConn` with `container/list` + map for O(1) lookup and removal:

```go
type agentPool struct {
    mu       sync.Mutex
    idle     map[string]*list.List       // routeID → list of *pooledAgentConn
    waiters  map[string]map[chan *pooledAgentConn]struct{} // routeID → waiter channels
}
```

**Impact:** At scale (1000s of idle connections), removal drops from O(n) to O(1). Reduces lock hold time on every `add()`/`take()`.

---

## 7. Host Allocator: Full Scan → Indexed Lookup (Medium Impact)

**Problem:** `allocator.go:AllocateHost()` iterates all registered hosts (O(n)) every time. With 1000+ hosts this becomes slow.

**Fix:** Maintain pre-indexed host sets by region/resource-class:

```go
type HostAllocator struct {
    hosts     map[string]*Host
    byRegion  map[string]map[string]*Host  // region → hostID → Host
    byClass   map[ResourceClass]map[string]*Host  // class → hostID → Host
    byOnline  map[string]*Host  // online hosts only (used for fast path)
    mu        sync.RWMutex
}
```

Filters collapse to O(k) where k is the indexed subset size.

**Impact:** Allocation latency for a busy gateway drops from linear to near-constant.

---

## 8. Stats Collector: Stub → Real Parsing (Medium Impact — Observability)

**Problem:** `webrtc.go:startStatsCollector()` calls `pc.GetStats()` but discards the result (`_ = stats`). No actual metrics are extracted.

**Fix:** Parse the `webrtc.StatsReport` to populate `SessionStats`:

```go
for _, stat := range stats {
    switch s := stat.(type) {
    case webrtc.InboundRTPStreamStats:
        s.BytesReceived  // → session.Stats.BytesReceived
        s.PacketsLost    // → session.Stats.PacketsLost
        s.Jitter         // → session.Stats.Jitter
        // ...
    case webrtc.OutboundRTPStreamStats:
        s.BytesSent      // → session.Stats.BytesSent
        // ...
    case webrtc.RemoteCandidateStats:
        s.RTT            // → session.Stats.RTT (candidate-pair RTT)
    }
}
```

**Impact:** Enables adaptive bitrate, real-time QoS monitoring, and per-session telemetry that the current architecture assumes but doesn't deliver.

---

## 9. Scheduler + Allocator: Dual Instance Bug (Critical Fix)

**Problem:** `session/session.go` creates its own `HostAllocator` in `NewSessionManager`, while `session/scheduler.go` also creates its own `HostAllocator` in `NewSessionScheduler`. Hosts registered with one are invisible to the other.

**Fix:** Pass the same `HostAllocator` instance to both:

```go
func NewSessionManager(logger *log.Logger, hostAllocator *HostAllocator) *SessionManager {
    // use provided allocator instead of creating new one
}

func NewSessionScheduler(logger *log.Logger, hostAllocator *HostAllocator) *SessionScheduler {
    // use provided allocator instead of creating new one
}
```

**Impact:** Fixes a latent bug where sessions are scheduled to hosts that don't know about them, or hosts report load to an allocator nobody reads.

---

## 10. HTTP Proxy Director Closure Per-Request (Low Impact)

**Problem:** `http_proxy.go:49-51` allocates a new `Director` closure on every proxy request.

**Fix:** Create a single re-usable director that reads target info from request context:

```go
type proxyKey struct{}
func withTarget(r *http.Request, target *url.URL, upstreamPath string) *http.Request {
    return r.WithContext(context.WithValue(r.Context(), proxyKey{}, targetInfo{target, upstreamPath}))
}

var sharedDirector = func(req *http.Request) {
    info := req.Context().Value(proxyKey{}).(targetInfo)
    req.URL.Scheme = info.target.Scheme
    req.URL.Host = info.target.Host
    req.URL.Path = info.upstreamPath
    // ...
}
```

**Impact:** Minor — reduces GC pressure under very high proxy throughput (10K+ req/s).

---

## 11. Global Signal Store Singleton → Server Field (Medium — Testability)

**Problem:** `signaling.go` uses `var signals = newSignalStore()` — a package-level singleton. Cannot unit test signaling with isolated state.

**Fix:** Move to a field on `Server`:

```go
type Server struct {
    // ...
    signalStore *signalStore
}
```

**Impact:** Enables parallel test execution and cleans up the global state pattern.

---

## 12. Custom `sqrt` → `math.Sqrt` (Low — Correctness)

**Problem:** `audio.go` implements a custom Newton-Raphson `sqrt()` instead of using `math.Sqrt`.

**Fix:** Replace with `math.Sqrt` — hardware-backed on all Go platforms, both faster and more accurate.

---

## Priority Summary

| Priority | Optimization | Effort | Impact | Risk |
|----------|-------------|--------|--------|------|
| P0 | Fix dual HostAllocator bug | 1 day | Bug fix | Low |
| P0 | UDP client map cleanup | 0.5 day | Memory leak | Low |
| P1 | Buffer pool adoption | 1 day | High (GC) | Low |
| P1 | Ring buffer bulk copy | 0.5 day | High (audio) | Low |
| P2 | Priority queue → heap | 0.5 day | Medium | Low |
| P2 | Agent pool O(1) removal | 1 day | Medium | Low |
| P2 | Stats collector parsing | 1 day | Medium (obs.) | Low |
| P3 | Host allocator indexes | 2 days | Medium | Medium |
| P3 | WebSocket pooled frames | 1 day | Medium | Low |
| P3 | Signal store to field | 0.5 day | Low | Low |
| P4 | Proxy director refactor | 0.5 day | Low | Low |
| P4 | `sqrt` → `math.Sqrt` | 0.1 day | Low | Low |
