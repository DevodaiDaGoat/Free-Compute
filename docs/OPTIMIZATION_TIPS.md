# Optimization Tips

Practical, high-leverage settings and configurations for deploying and operating FreeCompute. Not code changes — these are runtime, deployment, and infra-level optimizations.

---

## 1. Kernel & Network Tuning

### sysctl for WebRTC/QUIC workloads

```bash
# Increase buffer sizes for high-throughput UDP (WebRTC media)
net.core.rmem_max = 134217728
net.core.wmem_max = 134217728
net.ipv4.udp_rmem_min = 65536
net.ipv4.udp_wmem_min = 65536

# TCP backlog for HTTP proxy routes
net.core.somaxconn = 65535
net.ipv4.tcp_max_syn_backlog = 65535

# Fast TCP keepalive for detecting dead connections
net.ipv4.tcp_keepalive_time = 60
net.ipv4.tcp_keepalive_intvl = 10
net.ipv4.tcp_keepalive_probes = 6

# Enable BBR congestion control for WAN proxy routes
net.core.default_qdisc = fq
net.ipv4.tcp_congestion_control = bbr

# Faster TIME_WAIT recycling for high-throughput gateways
net.ipv4.tcp_fin_timeout = 15
net.ipv4.tcp_tw_reuse = 1
```

Apply with `sysctl -p` or drop into `/etc/sysctl.d/99-freecompute.conf`.

### NIC offloading

```bash
# Enable GRO/GSO for bulk throughput, disable TSO if CRC errors appear
ethtool -K eth0 gro on gso on tso on
# Tune ring buffer size
ethtool -G eth0 rx 4096 tx 4096
```

---

## 2. Go Runtime Tuning

### GOGC

Gateway handles many concurrent long-lived WebRTC connections. Default GOGC=100 causes frequent GC scans.

```bash
export GOGC=400    # Less frequent, larger GC runs
export GOMEMLIMIT=4GiB  # Hard memory cap (Go 1.19+)
```

Set `GOMEMLIMIT` to ~80% of available RAM to prevent OOM while allowing GOGC to stay high.

### GOMAXPROCS

Pin to available CPUs. Overprovisioning causes scheduler thrashing.

```bash
export GOMAXPROCS=$(nproc)   # or set via cpu quota in container
```

In Kubernetes, use `GOMAXPROCS` matching the container's CPU request (use `automaxprocs` library).

### Memory profile

For gateway:

```bash
export GORACE=                    # disable race detector in prod
export GODEBUG=madvdontneed=1     # return memory to OS faster
```

---

## 3. Docker / Container Tuning

### Gateway container

```yaml
# docker-compose.yml snippet
gateway:
  image: freecompute/gateway
  cap_add:
    - NET_ADMIN          # needed for TURN relay port allocation
    - SYS_PTRACE         # optional — pprof profiling
  sysctls:
    net.core.rmem_max: "134217728"
    net.core.wmem_max: "134217728"
  ulimits:
    nofile: 1048576      # high concurrent connections
    memlock: -1          # pin memory for TURN relay
  ports:
    - "8080:8080/tcp"
    - "50000-51000:50000-51000/udp"   # WebRTC ICE port range
  environment:
    GOMEMLIMIT: 4GiB
    GOGC: 400
```

### Host agent resource limits

```yaml
host-agent:
  image: freecompute/host-agent
  privileged: true       # QEMU/kvm access
  devices:
    - /dev/kvm
    - /dev/net/tun
  volumes:
    - /var/run/libvirt/libvirt-sock:/var/run/libvirt/libvirt-sock
```

---

## 4. WebRTC ICE & TURN Optimization

### Minimize ICE candidates

```env
FREECOMPUTE_ICE_TYPE=host     # host-only for LAN (zero additional latency)
FREECOMPUTE_ICE_TYPE=srflx    # add reflexive for NAT traversal
FREECOMPUTE_ICE_TYPE=relay    # add TURN as last resort only
```

In production, set `srflx` as default and `relay` only when STUN fails — TURN relay adds ~5-20ms latency.

### TURN port range

Allocate a dedicated port range for TURN relay UDP sockets. Avoid ephemeral port conflicts:

```env
FREECOMPUTE_TURN_PORT_MIN=50000
FREECOMPUTE_TURN_PORT_MAX=51000
```

Pre-open these in the firewall / security group.

### Trickle ICE

Ensure trickle ICE is enabled (it's on by default in Pion). Gives ~500ms faster connection establishment vs non-trickle.

---

## 5. Session & Scheduler Tuning

### Scoring weights

```env
FREECOMPUTE_SCHEDULER_LATENCY_WEIGHT=3.0
FREECOMPUTE_SCHEDULER_GPU_WEIGHT=2.0
FREECOMPUTE_SCHEDULER_LOAD_WEIGHT=1.0
FREECOMPUTE_SCHEDULER_ENCODER_WEIGHT=1.5
```

Tune weights to your workload. Gaming → higher GPU weight. Office desktop → higher latency weight.

### Session idle timeout

```env
FREECOMPUTE_SESSION_IDLE_TIMEOUT=15m   # shorter = better resource utilization
FREECOMPUTE_SESSION_MAX_DURATION=12h   # hard cap for fair scheduling
```

---

## 6. Frontend Optimization

### Lazy load WebRTC bundle

The `pion/webrtc` WASM or browser WebRTC adapter should load on-demand, not at app bootstrap:

```typescript
// Instead of: import * as webrtc from '@pion/webrtc'
const webrtc = await import('@pion/webrtc');
```

Saves ~200 KB from initial bundle.

### Video codec hint

```typescript
const pc = new RTCPeerConnection({
  sdpSemantics: 'unified-plan',
  iceServers: [...]
});
// Prioritize hardware encoder
const codec = RTCRtpSender.getCapabilities('video')?.codecs
  .filter(c => c.mimeType.includes('H264'))[0];
```

### Preconnect to gateway

```html
<link rel="preconnect" href="https://gateway.freecompute.io" />
<link rel="dns-prefetch" href="https://gateway.freecompute.io" />
```

---

## 7. Observability Without Overhead

### Sampling rate for telemetry

```env
FREECOMPUTE_TELEMETRY_SAMPLE_RATE=0.1   # 10% of sessions — enough for SLO tracking
```

Full 100% telemetry is expensive at scale. Sample aggressively; retain full detail for flagged sessions.

### Metrics cardinality

Prometheus metrics with high-cardinality labels (session ID, user ID) blow up TSDB. Use summaries/histograms instead:

```go
// Good
webrtcLatency := prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name: "webrtc_rtt_ms",
        Buckets: []float64{5, 10, 25, 50, 100, 250, 500},
    },
    []string{"region"},   // 10-20 values, not 10K+
)

// Bad
webrtcLatency := prometheus.NewGaugeVec(
    prometheus.GaugeOpts{Name: "webrtc_rtt_ms"},
    []string{"session_id"},  // unbounded cardinality — will OOM your prometheus
)
```

---

## 8. TLS Termination

Terminate TLS at the load balancer (LB → gateway on plain HTTP/2). Saves the gateway from per-connection TLS handshake overhead:

```
Client ──TLS──▶ LB (nginx / Caddy / Cloudflare) ──plain HTTP/2──▶ Gateway
```

If you must terminate on the gateway, use TLS session resumption (`SessionTicketKey` rotation) and OCSP stapling.

---

## 9. File Service Backend

For the file-service storage backend:

| Backend | Read Latency | Write Latency | Cost | Best For |
|---------|-------------|--------------|------|----------|
| Local SSD | ~0.1ms | ~0.1ms | $$$ | Low-latency sessions, single-node |
| S3 | ~5-20ms | ~10-30ms | $ | Multi-region, persistence |
| Ceph/MinIO | ~1-5ms | ~1-5ms | $$ | HA, multi-node on-prem |
| NFS | ~0.5-2ms | ~0.5-2ms | $$ | LAN cluster, shared home dirs |

For session home directories, prefer NFS or MinIO over S3 — S3's write latency is noticeable for desktop workloads (file saves, IDE autosave).

---

## 10. Deployment Patterns

| Pattern | Pros | Cons | When |
|---------|------|------|------|
| **All-in-one** | Simple, no network overhead | Single point of failure | Dev, small community hosts |
| **Gateway + Host co-located** | Low-latency tunnel (UNIX socket) | No redundancy | Single-tenant dedicated |
| **Gateway HA pair** | Failover, zero-downtime | Raft overhead, complexity | Production multi-tenant |
| **Multi-region gateway** | Global low-latency | Cross-region state sync | Enterprise |

Start with co-located, graduate to HA pair as traffic grows.

---

## 11. QEMU / Libvirt Tuning (Host Agent)

### CPU Pinning & hugepages

```xml
<!-- Pin VM vCPUs to physical cores to avoid NUMA migration -->
<numatune>
  <memory mode="strict" nodeset="0"/>
  <memnode cellid="0" mode="strict" nodeset="0"/>
</numatune>
<cputune>
  <vcpupin vcpu="0" cpuset="2"/>
  <vcpupin vcpu="1" cpuset="3"/>
  <emulatorpin cpuset="0-1"/>
</cputune>

<!-- 1 GB hugepages for VM RAM — reduces TLB misses -->
<memoryBacking>
  <hugepages>
    <page size="1048576" unit="KiB" nodeset="0"/>
  </hugepages>
  <nosharepages/>
  <locked/>
</memoryBacking>
```

Apply via `virsh edit` or embed in the domain XML template. Reduces VM exit overhead by ~20% for memory-heavy workloads.

### Disk cache mode

| Mode | Consistency | Performance | Risk |
|------|------------|-------------|------|
| `none` | Full | Lowest (guest flushes to host) | None — safest |
| `writethrough` | Full | Low (host cache, immediate write) | None |
| `writeback` | Guest if shutdown cleanly | High | Data loss on host crash |
| `unsafe` | None | Highest | Only for disposable VMs |

For gaming/desktop sessions, use `writeback` + periodic `virsh dompmsuspend` — 3-5x I/O improvement over `none` with acceptable risk for ephemeral sessions.

### virtio series

- Use `virtio` for everything: disk, NIC, balloon, RNG, input
- Ensure guest has `virtio-win` or `linux-virtio` drivers installed
- `virtio-net` with `mq=on` + `vectors=2N+2` for multi-queue — line-rate 10GbE even for small packets

### vhost-net acceleration

```bash
# Host agent should enable vhost-net for zero-copy virtio-net I/O
modprobe vhost_net
echo vhost_net >> /etc/modules

# Check it's active per queue
ls -l /sys/class/net/*/queues/rx-*/vhost*
```

---

## 12. WebRTC (Pion) Tuning

### ICE configuration

```go
// gateway/internal/webrtc/webrtc.go
settingEngine := webrtc.SettingEngine{}

// Reduce ICE candidate gathering timeout
settingEngine.SetICETimeouts(2*time.Second, 10*time.Second, 3*time.Second)
// Defaults are 5s/25s/5s — tightening means faster connect on good networks

// Bind to specific interface (avoid gathering on docker bridges)
settingEngine.SetICEUDPMux(udpMux) // reuse single port for all ICE
settingEngine.SetNetworkTypes([]webrtc.NetworkType{
    webrtc.NetworkTypeUDP4,
    webrtc.NetworkTypeUDP6,
})
// Skip TCP ICE candidates — TCP ICE adds 200ms+ and is rarely needed
// Keep it disabled unless you know users behind TCP-only firewalls
```

### Datagram size for data channels

```go
// Set larger SCTP receive buffer for high-throughput file transfers
settingEngine.SetSCTPMaxReceiveBufferSize(16 * 1024 * 1024)    // 16 MB

// Increase datagram MTU — default 1200 is conservative
settingEngine.SetReceiveMTU(1460)  // typical Ethernet jumbo frame-ish
```

### Simulcast / SVC tradeoffs

VP9 SVC encodes spatial/temporal layers in a single stream. Key tradeoff: ~15% bitrate overhead for the base layer vs sending a single stream. Enable only when clients span diverse network conditions.

```go
// Enable VP9 SVC if codec available
codecSelector := []string{"VP9", "H264"}
```

### Detach data channels

```go
// In ReadLoop, detach data channels for lower latency reads:
detached, err := dc.Detach()
// Avoids one goroutine per channel read; cuts data channel latency by ~30%
```

---

## 13. CI/CD Optimizations

### Turborepo caching

```bash
# In CI, restore turbo cache from remote
turbo run build --remote-only  # or:
turbo run build --team=freecompute --token=$TURBO_TOKEN

# Cache key includes: lockfile, tsconfig, package.json, .env.example
# Exclude from cache: dist/, .next/, node_modules/
```

`.turbo/config.json`:

```json
{
  "remoteCache": {
    "enabled": true,
    "teamId": "team_freecompute",
    "signature": true
  }
}
```

Remote cache cuts CI build times from 3-5 min to 10-20 seconds.

### Go module cache

```yaml
# GitHub Actions
- uses: actions/cache@v4
  with:
    path: |
      ~/go/pkg/mod
      ~/.cache/go-build
    key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
    restore-keys: |
      ${{ runner.os }}-go-
```

Go build cache + module cache reduces `go build` from ~2 min to ~15 seconds.

### Docker layer caching

```dockerfile
# Separate dependency layers from code layers
COPY go.mod go.sum ./
RUN go mod download          # cached unless go.sum changes

COPY . .
RUN go build -o /gateway ./cmd/gateway    # cached only when source changes
```

`DOCKER_BUILDKIT=1 docker build --cache-from=gateway:latest` in CI.

### Parallel Go test strategy

```bash
# Speed up tests across Go modules
go test ./apps/gateway/... ./host-agent/... -p=4 -count=1 -parallel=8
```

Use `-p` for package-level parallelism and `-parallel` for test-level. With 4+ CPU cores, test suite runs 3-4x faster.

---

## 14. Security Hardening

### Gateway API rate limiting

```bash
# env vars for token bucket rate limiter
FREECOMPUTE_RATE_LIMIT_REQUESTS=100       # requests per window
FREECOMPUTE_RATE_LIMIT_WINDOW=1s          # sliding window
FREECOMPUTE_RATE_LIMIT_BURST=50           # max burst

# Separate limits for session creation (expensive)
FREECOMPUTE_RATE_LIMIT_SESSION_CREATE=5   # per minute per user
FREECOMPUTE_RATE_LIMIT_SESSION_CREATE_WINDOW=60s
```

### Headers

```go
// middleware/security.go
func SecurityHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Security-Policy",
            "default-src 'self'; " +
            "connect-src 'self' wss: https:; " +
            "img-src 'self' data: blob:; " +
            "media-src 'self' blob:; " +
            "script-src 'self' 'wasm-unsafe-eval'; " +
            "frame-ancestors 'none';")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
        w.Header().Set("Permissions-Policy",
            "camera=(), microphone=(self), geolocation=()")
        next.ServeHTTP(w, r)
    })
}
```

### Token expiration

| Token | Lifetime | Storage | Rotation |
|-------|----------|---------|----------|
| Session token | 24h | Gateway memory | On reconnect |
| API key | 90d | Database | Manual |
| Share link | Configurable (1h-7d) | Database | HMAC-signed |
| Refresh token | 30d | Database | As needed |

### TLS for inter-service

Even inside the private network, use mTLS between gateway, file-service, and scheduler. A misconfigured firewall or container escape should not expose internal APIs:

```bash
FREECOMPUTE_INTERNAL_CA_CERT=/etc/freecompute/ca.pem
FREECOMPUTE_INTERNAL_CERT=/etc/freecompute/gateway.pem
FREECOMPUTE_INTERNAL_KEY=/etc/freecompute/gateway-key.pem
```

---

## 15. Graceful Shutdown

### Gateway

```go
// Main signal handler
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

<-sigCh
log.Println("Shutting down gracefully...")

// 1. Stop accepting new connections
server.Shutdown(context.WithTimeout(ctx, 30*time.Second))

// 2. Drain active sessions (allow up to 5s per session)
sessionManager.Drain(ctx, 5*time.Second)

// 3. Close agent pool (agents reconnect to another gateway)
agentPool.Close()

// 4. Stop metrics server
metricsServer.Shutdown(ctx)

// 5. Exit
log.Println("Goodbye.")
```

### Host agent

On `SIGTERM`, host agent should:
1. Stop accepting new VM launches
2. Send `draining` status to gateway (gateway stops scheduling to this host)
3. Allow running VMs to finish (up to `FREECOMPUTE_HOST_DRAIN_TIMEOUT=60s`)
4. Force-stop remaining VMs
5. Disconnect from gateway

### Kubernetes preStop hook

```yaml
lifecycle:
  preStop:
    exec:
      command: ["/bin/sh", "-c", "sleep 10 && kill -TERM 1"]
```

The sleep gives the service mesh / load balancer time to remove the pod from endpoints before the process shuts down.

---

## 16. Frontend Build Optimization

### Next.js config

```typescript
// next.config.ts
const config: NextConfig = {
  // Tree-shake moment.js locales
  webpack: (config) => {
    config.plugins.push(
      new webpack.IgnorePlugin({
        resourceRegExp: /^\.\/locale$/,
        contextRegExp: /moment$/,
      })
    );
    return config;
  },
  // aggressive code splitting
  modularizeImports: {
    'lodash': { transform: 'lodash/{{member}}' },
    '@mui/material': { transform: '@mui/material/{{member}}' },
  },
  // Compress with brotli
  compress: true,
  // Generate source maps only in dev
  productionBrowserSourceMaps: false,
  // Enable SWC minifier (faster than terser)
  swcMinify: true,
};
```

### Bundle analysis

```bash
# Run after every significant change
ANALYZE=true npm run build
# Opens bundle visualizer — identify large deps to code-split
```

### WebRTC adapter lazy loading

```typescript
// Lazy-load the heavy WebRTC package
const startSession = async () => {
  const { RTCPeerConnection } = await import('webrtc-adapter');
  // ... session setup
};
```

Reduces initial JS bundle by ~180 KB gzipped.

---

## 17. Logging Best Practices

### Structured JSON logging

```go
// Use zerolog or slog with structured output
log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))

// Include: request_id, session_id, duration_ms, error_kind
log.Info("session created",
    "session_id", session.ID,
    "user_id", session.UserID,
    "region", session.Region,
    "duration_ms", time.Since(start).Milliseconds(),
)
```

### Sampling strategy

| Log Level | Volume | Sampling | Retention |
|-----------|--------|----------|-----------|
| ERROR | Low (1/min) | Never sample | 30 days |
| WARN | Medium | Sample 50% | 7 days |
| INFO | High | Sample 10% | 24 hours |
| DEBUG | Very high | Off in prod | N/A |

### Log rotation

```yaml
# /etc/logrotate.d/freecompute
/var/log/freecompute/*.log {
    daily
    rotate 14
    compress
    delaycompress
    missingok
    notifempty
    postrotate
        kill -HUP $(pidof gateway)
    endscript
}
```

---

## 18. Database & Storage

### Connection pooling

```go
// Gateway talks to file-service (or a future database)
// Always pool connections
db.SetMaxOpenConns(50)           // max concurrent queries
db.SetMaxIdleConns(25)           // keep 25 connections warm
db.SetConnMaxLifetime(5 * time.Minute)  // recycle to avoid stale connections
db.SetConnMaxIdleTime(1 * time.Minute)  // release idle quickly
```

### S3 performance

```bash
# For file-service S3 backend:
# 1. Enable multipart upload threshold
AWS_S3_MULTIPART_THRESHOLD=50MB

# 2. Use S3 Transfer Acceleration for cross-region uploads
AWS_S3_ACCELERATE_ENDPOINT=true

# 3. In-minio deployment: set up erasure coding at "EC:4" (tolerate 4 disk failures)
# 4. Use NVMe SSDs for MinIO backing store, not spinning disks
```

### File service read-ahead

```go
// When a session reads a file sequentially, prefetch next 1 MB
// Reduces per-read latency from 10ms to 2ms for large sequential reads
type ReadAheadReader struct {
    reader    io.ReaderAt
    cache     []byte
    cacheOff  int64
    cacheSize int // 1 MB default
}
```

---

## 19. Host Agent VM Lifecycle

### QEMU startup flags

```bash
# Host agent should launch QEMU with these performance flags:
qemu-system-x86_64 \
  -accel kvm \
  -cpu host,migratable=off,+kvm_pv_unhalt,+kvm_pv_eoi \
  -smp "$VCPUS",cores="$VCPUS",threads=1,sockets=1 \
  -m "$RAM_MB" \
  -object memory-backend-file,id=mem,size="$RAM_MB"M,mem-path=/dev/hugepages,share=on \
  -numa node,memdev=mem \
  -netdev vhost-user,id=net0,chardev=char0,vhostforce=on \
  -device virtio-net-pci,netdev=net0,mq=on,vectors=6 \
  -drive file="$DISK_IMAGE",if=none,id=drive0,cache=writeback,discard=unmap \
  -device virtio-blk-pci,drive=drive0 \
  -vga none -nographic \
  -nodefaults -no-user-config
```

Key points:
- `-accel kvm` — mandatory for near-native performance
- `-cpu host` — expose host CPU features (AVX, AES-NI, etc.)
- `hugepages` — mandatory for memory-intensive workloads
- `vhost-user` for networking — bypasses QEMU's internal virtio-net; cuts p99 latency by 40%

### VM image pre-cache

```bash
# Pre-pull popular images so sessions start instantly
FREECOMPUTE_IMAGE_PRE_CACHE="ubuntu-24.04,ubuntu-22.04,debian-12,windows-11"

# On host agent startup or idle, background pull these images
# Gateway can query: GET /api/v1/hosts/{id}/cached-images
```

---

## 20. Monitoring Without Pain

### RED metrics (Rate, Errors, Duration)

Every component should track:

```go
// Rate — requests per second
httpRequestsTotal.WithLabelValues(method, path, status).Inc()

// Errors — failed requests
httpRequestsErrorsTotal.WithLabelValues(method, path, errorKind).Inc()

// Duration — latency distribution
httpRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
```

### RED targets for FreeCompute

| Component | Rate Target | Error Budget | Duration SLO |
|-----------|------------|--------------|-------------|
| Gateway API | < 10K req/s/gateway | < 0.1% 5xx | p99 < 100ms |
| WebRTC connect | < 100/s/gateway | < 1% failure | p99 < 3s |
| Session streaming | N/A | < 2% packet loss | p95 RTT < 50ms |
| File service | < 1K req/s | < 0.5% 5xx | p99 < 500ms |
| Host agent | N/A | < 0.5% VM crash | Session ready < 10s |

### Health check endpoints

```go
// /healthz should validate downstream dependencies
GET /healthz
{
  "status": "ok",
  "uptime_seconds": 123456,
  "dependencies": {
    "file_service": { "status": "ok", "latency_ms": 2 },
    "agent_pool":  { "status": "ok", "connected_agents": 42 },
    "scheduler":   { "status": "ok", "queue_depth": 3 }
  },
  "version": "0.1.0"
}
```

### Prometheus recording rules (cheap, pre-computed metrics)

```yaml
# rules.yml — run every 30s, cheap to query later
groups:
  - name: freecompute_aggregates
    rules:
      - record: gateway:session_count:avg_5m
        expr: avg(rate(gateway_sessions_total[5m]))
      - record: gateway:connection_rtt_p95:avg_5m
        expr: histogram_quantile(0.95, rate(webrtc_rtt_ms_bucket[5m]))
      - record: host:available:avg_5m
        expr: avg(host_agent_available) by (region)
```

Pre-computed rules are orders of magnitude cheaper than running raw PromQL queries in Grafana dashboards.
