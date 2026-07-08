# Scheduler Design

Low-latency, GPU-aware host selection for desktop, gaming, development, and remote-support sessions.

---

## 1. Goal & Requirements

The scheduler picks the best host for a session request, balancing latency, GPU/encoder availability, network quality, host load, region affinity, and resource class. Gaming sessions have the strictest latency and encoder constraints.

---

## 2. Current State

### Scheduler service (`apps/scheduler`)

Not a stub. The service is fully implemented:

| Component | File | Behavior |
|-----------|------|----------|
| HTTP API | `apps/scheduler/cmd/scheduler/main.go` | Exposes `/api/queue`, `/api/allocations`, `/api/schedule`, `/api/hosts/register`, `/api/hosts/heartbeat`, `/api/hosts` |
| Queue & priority heap | `apps/scheduler/internal/scheduler/queue.go` | Priority-ordered `QueueItem` queue with enqueue/dequeue/peek |
| Scheduler loop | `apps/scheduler/internal/scheduler/scheduler.go` | Periodic `scheduleCycle` (default 5 s) that ranks available hosts and allocates |
| Ranking function | `apps/scheduler/internal/scheduler/ranker.go` | Scores hosts by CPU/RAM availability, latency, uptime, GPU VRAM, and current load |
| Resource allocator | `apps/scheduler/internal/scheduler/allocator.go` | Tracks CPU/RAM/GPUVRAM capacity and leases with TTL expiry |
| Host manager | `apps/scheduler/internal/host/manager.go` | In-memory host registry with register/heartbeat/allocate/release |
| Metrics collector | `apps/scheduler/internal/host/metrics.go` | Rolling `HostMetrics` history per host with `AverageLoad` window queries |
| Config | `apps/scheduler/internal/config/config.go` | Env-driven: `FREECOMPUTE_SCHEDULER_ADDR`, interval, max queue, TTL, auth token |

### Gateway-side scheduling (`apps/gateway/internal/session`)

| Component | File | Behavior |
|-----------|------|----------|
| `HostAllocator` | `apps/gateway/internal/session/allocator.go` | In-process host registry with region/class indexes, `AllocateHost` filter + `scoreHost` heuristic |
| `SessionScheduler` | `apps/gateway/internal/session/scheduler.go` | Priority heap of `QueuedSession`; gaming priority = 100, desktop = 50, support = 75, host = 25 |
| `SessionManager` | `apps/gateway/internal/session/session.go` | `CreateSession` calls `hostAllocator.AllocateHost` directly; does **not** yet call out to the scheduler service |

### Host capability reporting

`host-agent/cmd/host-agent/main.go:389-413` (`detectCapabilities`) populates `HostCapabilities` (GPU model/vendor/VRAM, encoder support, CPU, RAM, disk, network, region). `runStatusReporter` (`main.go:548-578`) POSTs `HostStatus` to `/hosts/metrics` every 30 s. The gateway's `handleHostMetrics` (`apps/gateway/internal/tunnel/server.go:1216-1228`) currently only logs the payload — it does **not** persist capabilities for scheduling.

---

## 3. Gaps

| Gap | Impact |
|-----|--------|
| `SessionManager.CreateSession` bypasses the scheduler service | Host selection is local to one gateway process; no cross-gateway visibility or queueing |
| Ranking function omits encoder availability, UDP/P2P reachability, and region affinity | Gaming sessions may land on hosts with software-only encoders or high-latency paths |
| No gaming-priority weight in scheduler service ranker | `SessionScheduler.calculatePriority` sets gaming=100, but the scheduler service `ranker.go` treats all sessions identically |
| Host capabilities are not persisted by the gateway | `handleHostMetrics` discards `HostCapabilities`; the gateway cannot advise session creation on encoder support |
| No failover path when a host goes offline mid-session | `HostAllocator` has no eviction or reallocation logic; allocations are lost if the host disappears |

---

## 4. Ranking Function

Extend `apps/scheduler/internal/scheduler/ranker.go:36-84` (`scoreHost`) with the following weighted factors:

| Factor | Weight | Source |
|--------|--------|--------|
| Latency (inverse, budget-aware) | 25 % | `Host.LatencyMs` vs `QueueItem.LatencyBudgetMs` |
| GPU/encoder availability | 20 % | `Host.EncoderSupport` + `Host.GPUVramGB`; penalize if session requires hardware encode and host lacks it |
| Network quality | 15 % | `Host.UplinkMbps`, `Host.SupportsUDP`, `Host.SupportsP2P`; boost for UDP/P2P-capable hosts |
| Host load (inverse) | 20 % | CPU/RAM/GPU/encoder utilization from `HostMetrics` history |
| Region match | 10 % | Same-region bonus; cross-region penalty proportional to RTT |
| Resource class match | 10 % | Exact `ResourceClass` match bonus; downgrade allowed only if no exact match exists |

Gaming sessions should apply a multiplier to the encoder and latency terms so that a host with a free hardware encoder and < 20 ms latency outranks a closer host with software encoding.

---

## 5. Integration with Session Creation

`SessionManager.CreateSession` (`apps/gateway/internal/session/session.go:265`) currently calls `hostAllocator.AllocateHost` synchronously. Replace this with:

1. If `req.HostID` is set, try that host directly (sticky sessions).
2. Otherwise, POST to the scheduler service `/api/queue` with the `CreateSessionRequest` fields.
3. Poll `/api/allocations` or receive a webhook/callback when allocation completes.
4. On success, create the `Session` with the allocated `HostID` and transition to `SessionStateProvisioning`.

This decouples session creation from a single gateway's host view and enables the scheduler service to balance load across gateways.

---

## 6. Gaming-Priority Scheduling

- `SessionScheduler.calculatePriority` (`apps/gateway/internal/session/scheduler.go:164-177`) already sets gaming=100. Mirror this in the scheduler service by adding a `SessionType`-aware weight in `ranker.go`.
- Pre-warm encoder sessions on the selected host before provisioning (reuse `GET /prewarm` in `apps/gateway/internal/tunnel/server.go:444`).
- If no host meets gaming latency + encoder constraints within `LatencyBudgetMs`, queue the session and re-evaluate every `ScheduleInterval` rather than failing immediately.

---

## 7. Host Metrics Ingestion

Extend `apps/gateway/internal/tunnel/server.go` `handleHostMetrics` to:

1. Parse `HostStatus.Capabilities` (GPU model, encoder support, VRAM).
2. Store in a `HostRegistry` keyed by host ID.
3. Update `HostAllocator` entries via `UpdateHostLoad` with the latest `HostLoad` snapshot.
4. Expose `GET /hosts` with capability filters so the scheduler service can pull host state via the gateway API.

---

## 8. Failover

- `apps/scheduler/internal/host/manager.go:153` (`MarkOffline`) already supports marking hosts offline.
- Add a watcher in `apps/scheduler/internal/scheduler/scheduler.go` that checks `LastHeartbeat` age (threshold: 2× reporting interval). If a host misses heartbeats, mark it offline and re-queue its active allocations.
- Re-queue by moving the session back to `SessionStateQueued` with priority bumped by 20 (so it jumps the queue faster than new requests).

---

## 9. Implementation Steps

| Step | File | Change |
|------|------|--------|
| 1 | `apps/scheduler/internal/scheduler/ranker.go` | Add encoder, UDP/P2P, region, and resource-class weights to `scoreHost` |
| 2 | `apps/scheduler/internal/scheduler/scheduler.go` | Add `SessionType` awareness; gaming multiplier on encoder/latency terms |
| 3 | `apps/gateway/internal/session/session.go` | `CreateSession` POSTs to scheduler service instead of calling `AllocateHost` directly |
| 4 | `apps/gateway/internal/tunnel/server.go` | `handleHostMetrics` persists `HostCapabilities` into `HostRegistry` |
| 5 | `apps/scheduler/internal/host/manager.go` | Add heartbeat age threshold check in `GetAvailable` |
| 6 | `apps/scheduler/internal/scheduler/scheduler.go` | Failover re-queue logic in `scheduleCycle` when host goes offline |
| 7 | `apps/scheduler/cmd/scheduler/main.go` | Add `/api/hosts/capabilities` endpoint for gateway pull |

---

## 10. Acceptance Criteria

- [ ] Gaming session (`SessionTypeGaming`, `ResourceClassGaming`) allocates to a host with hardware encoder and < 20 ms latency when available.
- [ ] If the closest host lacks H.264/H.265 hardware encode, the scheduler picks the next-best host with encoder support.
- [ ] `SessionManager.CreateSession` with no `HostID` delegates to the scheduler service; with a `HostID` uses sticky allocation.
- [ ] Host that misses 2 consecutive `/hosts/metrics` posts is marked offline; its active allocations are re-queued within one scheduler cycle.
- [ ] Scheduler service `/api/queue` rejects requests when queue exceeds `FREECOMPUTE_SCHEDULER_MAX_QUEUE`.
