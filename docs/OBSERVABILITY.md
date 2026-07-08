# Observability

Metrics, tracing, logging, dashboards, and alerting for the gateway.

---

## 1. What exists today

### Metrics (`apps/gateway/internal/monitoring/metrics.go`)
`Metrics` is a process-wide singleton (`GetMetrics()`). Counters are `atomic.Int64`:

| Group | Fields |
|-------|--------|
| HTTP | `RequestsTotal/Active/2xx/4xx/5xx` |
| Sessions | `SessionsTotal/Active/Desktop/Gaming/Remote` |
| Transports | `WebRTC/WebSocket/TCP/UDP/Connections`, `AgentConnections/AgentPoolSize` |
| Traffic | `BytesUploaded/Downloaded`, `TransferOperations`, `ProxyRequests/ProxyBytesProxied` |

`Snapshot()` (metrics.go:75) returns all fields; `PrometheusText()` (metrics.go:108) is a **partial** export — only `uptime_seconds`, `requests_*`, `sessions_active`, `webrtc_connections`, `agent_connections` are emitted. Bytes, proxy, and per-transport breakdowns are NOT exposed.

### Health (`apps/gateway/internal/monitoring/health.go`)
`HealthChecker` aggregates `ComponentHealth` (status `ok|degraded|down`, `LatencyMs`, `LastCheck`). `HandleHealthDetailed` (health.go:104) serves `/health/detail` with JSON `HealthReport`. Startup registers `gateway`, `http`, `tunnel` (server.go:332-334); `/healthz` is the basic liveness path.

### Collector (`apps/gateway/internal/monitoring/collector.go`)
`Collector.Start` ticks every `interval` (default 10s) and calls `collect()`, which reads `runtime.MemStats` and reports `system` (goroutines/alloc MB) and `runtime` (Go version/GC) components. `CollectSystemStats()` (collector.go:84) returns `SystemStats` but `CPUSeconds` is never populated.

### Connection quality (`apps/gateway/internal/tunnel/quality.go`)
`QualityTracker` (quality.go:212) holds per-route `ConnQuality` with EWMA `SmoothedRTTMs`, `SmoothedLossRatio`, `SmoothedJitterMs`, `BandwidthBps`, and a QoE `Score` (0-100). Thresholds in `DefaultQualityThresholds()` (Good RTT 30ms, Fair 100ms; Good loss 1%, Fair 5%; Good jitter 15ms, Fair 40ms). `All()` (quality.go:249) returns snapshots for export.

---

## 2. Critical gaps

- **Dead counters.** `RecordRequest`/`CompleteRequest` (metrics.go:57-73) and every `*Connections`/`Bytes*`/`Proxy*` counter are declared but **never called** anywhere in `tunnel/`. Request and transport metrics are permanently 0.
- **Quality tracker is never fed.** `qualityTracker` (server.go:93) and `QualityTracker.GetOrCreate`/`Update` exist, but no code calls `Update` — no RTT/loss/jitter ever flows in. `All()` returns empty.
- **No tracing.** No OpenTelemetry / trace IDs / span propagation anywhere.
- **No per-user or per-session metrics.** All counters are global aggregates.
- **No structured logging.** Uses `log.Default()` printf; no levels, JSON, or request IDs.
- **UDP/WebSocket sweeps unmeasured.** `udpClientMap.sweepLoop` (udp.go:118) and `signalStore.sweepLoop` (server.go:161) run but emit no metrics.
- **Manual Prometheus text.** `PrometheusText()` is hand-rolled; new counters must be added in two places (struct + function).

---

## 3. Proposed dashboard (Grafana + Prometheus)

Scrape `GET /metrics` (served by `metricsHandler`, server.go:438) every 15s.

**Dashboards:**
- **Traffic:** `rate(freecompute_requests_total[1m])`, `freecompute_requests_active`, 2xx/4xx/5xx ratios.
- **Sessions:** `freecompute_sessions_active`, desktop/gaming/remote split.
- **Transports:** `freecompute_webrtc_connections`, `freecompute_agent_connections`, agent pool size, bytes up/down.
- **Health:** status panel from `/health/detail`; goroutines + alloc MB from `system`/`runtime` components.
- **Quality (when wired):** per-route RTT, jitter, loss, QoE score heatmap; count of `poor` routes.

**Alert thresholds:**
- **Reconnect storm:** rate of new agent/WebRTC connections > N/s for 2m (once counters are live). No current signal.
- **Error rate SLO:** `rate(requests_5xx[5m]) / rate(requests_total[5m]) > 0.02` → page.
- **Latency SLO:** route `SmoothedRTTMs > 100ms` (Fair) or QoE `Score < 40` for 5m → warn.
- **Health:** `/health/detail` status `down` or `degraded` → page.

---

## 4. Implementation steps

1. **Wire counters.** In `tunnel/server.go` and the proxy/transport handlers, call `Metrics` on request start/end and on connection open/close (`WebRTCConnections`, `AgentConnections`, `Sessions*`, `Bytes*`). Add a middleware in `server.go` calling `RecordRequest`/`CompleteRequest`.
2. **Feed quality tracker.** In WebRTC stats path, call `qualityTracker.GetOrCreate(routeID).Update(rtt, lossRatio, bw)` (e.g. from `webrtc.go` stats collector). Expose `All()` via a new `/quality` JSON endpoint and add to `PrometheusText()`.
3. **Complete Prometheus export.** Emit every `Metrics` field in `PrometheusText()`; consider `github.com/prometheus/client_golang` instead of hand-rolled text.
4. **Structured logging.** Replace `log.Default()` with `log/slog` JSON; add request ID middleware; thread into collector/health/security.
5. **Distributed tracing.** Add OpenTelemetry SDK; start a span per request and per tunnel session; propagate via HTTP headers. Expose `/metrics` for OTLP or ship to a collector.
6. **Per-user metrics.** Tag counters with `userID` (labels) in `usage.Tracker` and session handlers.
7. **Sweep metrics.** Report `udpClientMap` and `signalStore` sizes as gauges.

---

## 5. Acceptance criteria

- `GET /metrics` reflects non-zero request/connection/byte counters under load.
- `/quality` returns live per-route RTT/jitter/loss/score; Prometheus text includes them.
- Grafana dashboard renders all five panels; alert rules evaluate without errors.
- All gateway logs are structured JSON with request IDs; one trace ID spans request→tunnel→agent.
- Per-user metric labels present and queryable.
- `go test ./internal/monitoring/...` passes; no new global printf logging added.
