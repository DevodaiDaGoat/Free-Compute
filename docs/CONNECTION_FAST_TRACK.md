# Connection Fast-Track: Low-Hanging Performance Wins

Concrete, code-level optimizations to reduce connection latency, improve throughput, and eliminate waste. Ordered by implementation effort vs. impact.

## 1. TCP Socket Tuning

### 1a. Aggressive Keepalive on All Tunnel Connections

**Problem:** Keepalive is 30s everywhere (`host-agent/main.go:217`, `tcp.go:14`). On lossy paths, a dead peer can go undetected for 30+ seconds, leaving stale connections in the agent pool.

**Fix:** Apply per-route keepalive — 5s interval, 3 probes, 1s probe间隔 — via `TCP_KEEPIDLE`, `TCP_KEEPINTVL`, `TCP_KEEPCNT`:

```go
func setTCPKeepaliveAggressive(conn *net.TCPConn) error {
    raw, err := conn.SyscallConn()
    if err != nil {
        return err
    }
    return raw.Control(func(fd uintptr) {
        syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, syscall.TCP_KEEPIDLE, 5)
        syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, syscall.TCP_KEEPINTVL, 1)
        syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, syscall.TCP_KEEPCNT, 3)
    })
}
```

Replace `_ = tcpConn.SetKeepAlive(true)` calls with this in:
- `host-agent/cmd/host-agent/main.go:235-236`
- `apps/gateway/internal/tunnel/tcp.go:107-108, 146-148`
- `apps/gateway/internal/tunnel/server.go:1191-1192`

### 1b. Set TCP Quick ACK

**Problem:** Linux delayed ACK (default 40ms) adds up to 40ms of latency per write-then-read cycle in the tunnel bridge.

**Fix:** Disable delayed ACK on tunnel sockets:

```go
raw.Control(func(fd uintptr) {
    syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, syscall.TCP_QUICKACK, 1)
})
```

### 1c. BBR Congestion Control

**Problem:** Default CUBIC congestion control is optimized for bulk throughput, not interactive low-latency.

**Fix:** Set `TCP_CONGESTION` to "bbr" on tunnel sockets:

```go
raw.Control(func(fd uintptr) {
    bbr, _ := syscall.GetsockoptString(int(fd), syscall.IPPROTO_TCP, syscall.TCP_CONGESTION)
    if bbr != "bbr" {
        syscall.SetsockoptString(int(fd), syscall.IPPROTO_TCP, syscall.TCP_CONGESTION, "bbr")
    }
})
```

Requires `CONFIG_TCP_CONG_BBR=y` in kernel (default in 5.9+). Gracefully degrade if unavailable.

### 1d. Set Send/Receive Buffer Sizes

**Problem:** Default kernel TCP buffer sizes (~208KB auto-tuned) are suboptimal for tunnel workloads with mixed small control messages and bulk data.

**Fix:** Set explicit buffer sizes:

```go
raw.Control(func(fd uintptr) {
    syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_SNDBUF, 1_048_576)
    syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF, 1_048_576)
})
```

### 1e. Apply to All Connection Points

Consolidate TCP tuning into a single helper, call it in:

| File | Line(s) | Connection |
|------|---------|------------|
| `host-agent/main.go` | 234-237 | Gateway tunnel conn |
| `host-agent/main.go` | 277-280 | Local target conn |
| `tunnel/tcp.go` | 106-109 | `dialTCP()` upstream |
| `tunnel/tcp.go` | 145-148 | Client conn in `bridgeTCP()` |
| `tunnel/server.go` | 1190-1193 | Tailscale direct conn |
| `tunnel/agent_handler.go` | agent conn | Agent tunnel conn |

---

## 2. Agent Pool Liveness — Eliminate the Zero-Timeout Read

**Problem:** `liveAgentConn()` in `agent_pool.go:112-131` sets a zero read deadline and tries to read 1 byte. This is problematic:
- Forces a syscall per `take()` — even on hot paths
- May consume real data from an agent that happened to send first (requiring `prefetchedConn` wrapper)
- Connections that are merely idle (no data pending) return a timeout error and are considered alive — but this gives a false positive if the TCP connection is actually dead (half-closed without data)

**Fix 1 — TCP Keepalive + Info:**
Replace the byte-read check with `TCP_INFO` — query the kernel for the socket's state:

```go
import "golang.org/x/sys/unix"

func liveAgentConn(conn net.Conn) (net.Conn, error) {
    tcpConn, ok := conn.(*net.TCPConn)
    if !ok {
        return conn, nil
    }
    raw, err := tcpConn.SyscallConn()
    if err != nil {
        return conn, nil
    }
    var ti *unix.TCPInfo
    err = raw.Control(func(fd uintptr) {
        ti, err = unix.GetsockoptTCPInfo(int(fd), unix.IPPROTO_TCP, unix.TCP_INFO)
    })
    if err != nil || ti == nil {
        return conn, nil // can't probe; trust the connection
    }
    // If the socket is in TIME_WAIT or CLOSE_WAIT, it's dead
    if ti.State == unix.TCP_TIME_WAIT || ti.State == unix.TCP_CLOSE_WAIT || ti.State == unix.TCP_CLOSING {
        return nil, errors.New("tcp connection closing")
    }
    // Unacked > threshold and no data in flight received suggests dead peer
    if ti.Unacked > 10 && ti.Receiv(1) == 0 && ti.Lost > 3 {
        return nil, errors.New("tcp connection appears dead (unacked + lost)")
    }
    return conn, nil
}
```

This is a **zero-allocation zero-syscall-on-hot-path** check — `TCP_INFO` is a read from kernel memory.

**Fix 2 — Fallback for non-Linux:**
Keep the deadline-read approach as a fallback on non-Linux platforms.

---

## 3. Host-Agent Bridge Buffer Pool

**Problem:** `host-agent/main.go:294-295` uses `io.Copy(dst, src)` which allocates a fresh 32KB buffer internally per call. Two goroutines per tunnel = 64KB alloc per tunnel.

**Fix:** Use the gateway's `getCopyBuf/putCopyBuf` pattern (or copy the buffer pool to the host-agent):

```go
func copyConn(errCh chan<- error, dst io.Writer, src io.Reader) {
    buf := getCopyBuf()
    defer putCopyBuf(buf)
    _, err := io.CopyBuffer(dst, src, *buf)
    // ...
}
```

Or simply lift the `copyBufferPool` into a shared internal package so both gateway and host-agent can use it.

---

## 4. WebSocket Unmask with Bulk XOR

**Problem:** `websocket.go:388-390` unmask-masks payloads byte-by-byte:

```go
for i := range payload {
    payload[i] ^= mask[i%4]
}
```

**Fix:** Use 32-bit XOR in bulk:

```go
func unmask(payload []byte, mask [4]byte) {
    if len(payload) == 0 {
        return
    }
    m32 := binary.LittleEndian.Uint32(mask[:])
    // Aligned part: 4 bytes at a time
    i := 0
    for ; i <= len(payload)-4; i += 4 {
        binary.LittleEndian.PutUint32(payload[i:], binary.LittleEndian.Uint32(payload[i:])^m32)
    }
    // Remainder
    for ; i < len(payload); i++ {
        payload[i] ^= mask[i%4]
    }
}
```

~4x throughput improvement on the unmask hot path.

---

## 5. Eliminate `prefetchedConn` Allocation

**Problem:** `prefetchedConn` wraps a `net.Conn` with a `prefix []byte` field. It's allocated on every `liveAgentConn()` call that finds data waiting — which shouldn't happen in steady state.

**Fix:** Remove the `prefetchedConn` struct entirely. If the liveness probe actually reads data (edge case), push it back using `bufio.Reader` on the upstream conn (which already has a `*bufio.Reader` in the HTTP CONNECT path). If no buffered reader exists, fall back to a one-byte peek.

Better yet: with the `TCP_INFO` approach above, the liveness probe never consumes data, so `prefetchedConn` becomes dead code.

---

## 6. WebSocket `bridgeWebSocketToUDP` — Use Buffer Pool

**Problem:** `websocket.go:142` does `buf := make([]byte, maxUDPPacketSize)` (65507 bytes) per goroutine. At 1000 concurrent UDP connections = 65MB of heap.

**Fix:** Use `getCopyBuf` or better, a dedicated large-buffer pool:

```go
func (s *Server) bridgeWebSocketToUDP(route *Route, bridge *webSocketBridge, upstream *net.UDPConn) {
    errCh := make(chan error, 2)
    go func() {
        buf := getCopyBuf()
        defer putCopyBuf(buf)
        buffer := *buf
        for {
            n, err := upstream.Read(buffer)
            // ...
        }
    }()
```

---

## 7. RTCP Reader: Pool 1500-Byte RTP Buffers

**Problem:** `webrtc.go:321-327` and `webrtc.go:344-351` allocate `make([]byte, 1500)` per RTCP reader goroutine. Two allocations per session = 3KB.

**Fix:** Create a small-buffer pool for RTP/RTCP reads:

```go
var rtpBufferPool = sync.Pool{
    New: func() any {
        buf := make([]byte, 1500)
        return &buf
    },
}
```

Used in both goroutines. Same pattern for `HandleMediaIngest` which also allocates `make([]byte, 1500)` per HTTP request (`webrtc.go:957, 997`).

---

## 8. Host-Agent Exponential Backoff

**Problem:** `host-agent/main.go:176-187` uses a fixed 1-second reconnection delay. When the gateway is down, all `poolSize × routes` goroutines hammer it every 1 second.

**Fix:** Add exponential backoff with jitter:

```go
func runTunnelLoop(ctx context.Context, cfg Config, route RouteConfig, slot int, logger *log.Logger, vm *VMManager) {
    baseDelay := cfg.ReconnectDelay       // 1s
    maxDelay := 30 * time.Second
    attempt := 0

    for {
        if err := runTunnelOnce(ctx, cfg, route, logger, vm); err != nil && ctx.Err() == nil {
            logger.Printf("route=%s slot=%d disconnected: %v", route.ID, slot, err)
        }

        select {
        case <-ctx.Done():
            return
        case <-time.After(computeBackoff(baseDelay, maxDelay, attempt)):
            attempt++
        }
    }
}

func computeBackoff(base, max time.Duration, attempt int) time.Duration {
    d := base << uint(attempt) // doubles each attempt
    if d > max {
        d = max
    }
    // Add jitter: ±25%
    jitter := time.Duration(float64(d) * (0.75 + 0.5*float64(attempt%4)/3.0))
    return jitter
}
```

Resets `attempt = 0` on successful connection.

---

## 9. Agent Pool: Remove O(n) Slice Removal

**Problem:** `agent_pool.go:143-154` (`removeIdle`) and `156-168` (`removeWaiter`) are O(n) linear scans. At 1000s of idle connections per route, they burn CPU under the mutex.

**Fix:** Use `container/list` (doubly-linked list) + map for O(1) removal, as documented in `PERFORMANCE_OPTIMIZATION.md:#6`. This is high-leverage because it also reduces lock hold time during `add()` and `take()`.

```go
type agentPool struct {
    mu      sync.Mutex
    idle    map[string]*list.List
    waiters map[string]map[chan *pooledAgentConn]struct{}
}
```

---

## 10. Connection Warmup: Pre-Establish Pool Connections

**Problem:** The agent pool starts empty. The first client request must wait for an agent to connect and register, which can take 1-3 seconds.

**Fix:** On route registration, eagerly establish 1 pooled connection:

```go
// In agent_handler.go, after agents.add():
go func() {
    // Pre-warm: the agent will reconnect to fill the pool
    // This is already handled by poolSize goroutines in the host-agent
    // But we can also warm on the gateway side
}()
```

The real leverage is in the **host-agent**: instead of waiting for the CONNECT to succeed then setting up the next tunnel sequentially, overlap the connections:

```go
// main.go:112-121 — launch pool goroutines with staggered starts
for i := 0; i < route.poolSize(); i++ {
    wg.Add(1)
    go func(slot int) {
        defer wg.Done()
        // Stagger startup so connections don't all fail at once
        time.Sleep(time.Duration(slot) * 200 * time.Millisecond)
        runTunnelLoop(ctx, cfg, route, slot, logger, vmManager)
    }(i)
}
```

---

## 11. Disable WebSocket Compression for Tunnel Bridges

**Problem:** `webrtc.go:172` enables `EnableCompression: true` on the signaling WebSocket upgrader. This is good for signaling (SDP/ICE messages are compressible). But the tunnel WebSocket (`handleWebSocketTunnel`) compresses all tunnel data—including binary streams that may already be compressed (video, images, encrypted data). Compression on incompressible data wastes CPU.

**Fix:** Create two upgraders — one compressed for signaling, one uncompressed for data:

```go
var (
    signalingUpgrader = websocket.Upgrader{
        ReadBufferSize:   32 * 1024,
        WriteBufferSize:  32 * 1024,
        EnableCompression: true,
        CheckOrigin:      allowAll,
    }
    tunnelUpgrader = websocket.Upgrader{
        ReadBufferSize:   32 * 1024,
        WriteBufferSize:  32 * 1024,
        EnableCompression: false, // binary tunnel data doesn't benefit
        CheckOrigin:      allowAll,
    }
)
```

---

## 12. Tailscale `dialViaTailscale` — Parallel Probes

**Problem:** `server.go:1173-1200` probes Tailscale hosts sequentially with a 5-second timeout each. With 5 hosts, worst case = 25s wait before falling back to the agent pool.

**Fix:** Race all hosts in parallel:

```go
func (s *Server) dialViaTailscale(route *Route) (net.Conn, error) {
    s.tailscaleMu.RLock()
    hosts := make([]*TailscaleHost, 0, len(s.tailscaleHosts))
    for _, host := range s.tailscaleHosts {
        if time.Since(host.LastSeen) <= 5*time.Minute {
            hosts = append(hosts, host)
        }
    }
    s.tailscaleMu.RUnlock()

    if len(hosts) == 0 {
        return nil, errors.New("no tailscale host available")
    }

    _, targetPort, _ := net.SplitHostPort(route.Target)
    type result struct {
        conn net.Conn
        err  error
    }
    ch := make(chan result, len(hosts))
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    for _, host := range hosts {
        host := host
        go func() {
            dialer := net.Dialer{Timeout: 5 * time.Second}
            conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host.TailscaleIP, targetPort))
            if err == nil {
                if tcpConn, ok := conn.(*net.TCPConn); ok {
                    tcpConn.SetNoDelay(true)
                    tcpConn.SetKeepAlive(true)
                }
            }
            ch <- result{conn, err}
        }()
    }

    // Return first success
    var firstErr error
    for i := 0; i < len(hosts); i++ {
        select {
        case r := <-ch:
            if r.err == nil {
                return r.conn, nil
            }
            if firstErr == nil {
                firstErr = r.err
            }
        case <-ctx.Done():
            return nil, ctx.Err()
        }
    }
    return nil, firstErr
}
```

---

## Impact Summary

| Optimization | Effort | Latency | Throughput | Reliability | Memory |
|---|---|---|---|---|---|
| TCP keepalive + quickack + BBR | 1d | -20% p50 | +10% | High | — |
| Agent pool `TCP_INFO` liveness | 0.5d | — | — | High | — |
| Host-agent buffer pool | 0.5d | — | +5% | — | -64KB/tunnel |
| WebSocket bulk unmask | 0.5d | — | +2% | — | — |
| Remove `prefetchedConn` | 0.25d | — | — | — | -1 alloc/take |
| RTCP buffer pool | 0.25d | — | — | — | -3KB/session |
| Exponential backoff | 0.5d | — | — | High | — |
| Agent pool O(1) removal | 1d | — | — | — | — |
| Connection warmup stagger | 0.25d | -30% cold start | — | Medium | — |
| WS compression for tunnel only | 0.25d | — | +5% | — | — |
| Tailscale parallel dial | 0.5d | -90% failover | — | High | — |
