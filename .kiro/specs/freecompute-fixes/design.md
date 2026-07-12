# Design Document: FreeCompute Fixes

## Overview

Four targeted bug fixes across the Go gateway and Next.js frontend:

1. **AudioBuffer deadlock** — eliminate re-entrant mutex acquisition in `audio.go`.
2. **UDP double buffer set** — apply socket buffer size exactly once with correct fallback in `udp.go`.
3. **Task Manager app** — implement a missing WebOS app component that polls the gateway.
4. **Hardcoded desktop widgets** — replace static strings with live `/healthz` and `/auth/profile` data.

Each fix is self-contained and can be implemented and reviewed independently.

---

## Architecture

No architectural changes. All fixes are local to existing files:

| Fix | File(s) |
|-----|---------|
| 1 | `apps/gateway/internal/audio/audio.go` |
| 2 | `apps/gateway/internal/tunnel/udp.go` |
| 3 | `apps/frontend/app/webos/apps/task-manager/TaskManager.tsx` (new) + `apps/frontend/app/webos/desktop/Desktop.tsx` |
| 4 | `apps/frontend/app/webos/desktop/Desktop.tsx` |

---

## Components and Interfaces

### Fix 1 — AudioBuffer (Go)

Current state: `AudioBuffer.Read()` acquires `b.mutex`, then calls `Available()` which also acquires `b.mutex` — deadlock.

`GetStats` in `AudioStreamer` also calls `session.Buffer.Available()` from outside a lock context, so `Available()` must remain public and safe to call externally.

Design:

```go
// availableLocked computes available bytes.
// MUST be called with b.mutex already held.
func (b *AudioBuffer) availableLocked() int {
    if b.writePos >= b.readPos {
        return b.writePos - b.readPos
    }
    return b.size - b.readPos + b.writePos
}

// Available is the public API for callers that do not hold b.mutex.
func (b *AudioBuffer) Available() int {
    b.mutex.Lock()
    defer b.mutex.Unlock()
    return b.availableLocked()
}

// Read holds b.mutex and calls availableLocked (not Available).
func (b *AudioBuffer) Read(size int) ([]byte, error) {
    b.mutex.Lock()
    defer b.mutex.Unlock()

    if !b.ready {
        return nil, io.EOF
    }
    available := b.availableLocked()   // <-- changed from b.Available()
    // ... rest unchanged
}
```

### Fix 2 — UDP dialUDP (Go)

Current state: buffers are set twice. When `s.cfg.UDPBufferSize` is 0 (default), the second call passes 0, which the OS silently accepts — potentially shrinking the buffer.

Design: single conditional block replaces the four `Set*Buffer` calls:

```go
func (s *Server) dialUDP(route *Route) (*net.UDPConn, error) {
    // ... resolve + dial unchanged ...

    bufSize := udpSocketBufferSize
    if s.cfg.UDPBufferSize > 0 {
        bufSize = s.cfg.UDPBufferSize
    }
    _ = upstream.SetReadBuffer(bufSize)
    _ = upstream.SetWriteBuffer(bufSize)

    return upstream, nil
}
```

### Fix 3 — TaskManager component (TypeScript/React)

New file: `apps/frontend/app/webos/apps/task-manager/TaskManager.tsx`

The component:
- Uses `getGatewayUrl()` from `BootSequence` to build fetch URLs (no auth needed for `/healthz`; `/capabilities` is also unauthenticated).
- Stores `healthStatus` (`'ok' | 'error' | 'loading'`), `protocols` (`string[]`), and `lastUpdated` (`Date | null`) in component state.
- Fetches on mount and on a 5-second interval via `useEffect` + `setInterval`.
- Cleans up the interval on unmount.
- Displays data in a simple styled table matching the existing WebOS dark theme (`rgba(17,17,40,0.85)` background, `#18e2ff` / `#888` text palette).

```tsx
// Minimal state shape
interface GatewayStats {
  status: 'loading' | 'ok' | 'error';
  protocols: string[];
  lastUpdated: Date | null;
  errorMessage: string | null;
}
```

Desktop wiring: add `'task-manager': () => <TaskManagerApp />` to `appComponents` in `Desktop.tsx`.

### Fix 4 — DesktopWidgets live data (TypeScript/React)

Modify `DesktopWidgets` in `Desktop.tsx`:

- Accept no new props; fetch internally.
- `/healthz` poll every 5 s, no auth token required. Derive `gatewayOnline: boolean` from `response.status === 'ok'`.
- `/auth/profile` fetched once on mount via `apiFetch` (which attaches the Bearer token). On success, read `storageUsed: number` (bytes) and `storageQuota: number` (bytes). Format as `"X.X / Y.Y GB used"` using a local `formatGB` helper. On failure (no token, 401, network error), display `"—"`.
- Cleanup: `clearInterval` on unmount return of `useEffect`.

```tsx
function formatGB(bytes: number): string {
  return (bytes / (1024 ** 3)).toFixed(1) + ' GB';
}
```

---

## Data Models

No new data models. The fixes use existing gateway response shapes:

- `GET /healthz` → `{ "status": "ok" }` (no auth required)
- `GET /capabilities` → `{ "protocols": string[], ... }` (no auth required)
- `GET /auth/profile` → `User` object with `storageUsed: number`, `storageQuota: number` (Bearer token required)

---

## Correctness Properties

These fixes are targeted bug corrections. Two of the four (fixes 1 and 2) involve pure
synchronization and configuration logic that is amenable to property-based testing.
Fixes 3 and 4 involve UI rendering and side-effect-driven data fetching, which are not
appropriate for PBT (behavior doesn't vary meaningfully with arbitrary input, and
rendering correctness is best captured by example tests).

*A property is a characteristic or behavior that should hold true across all valid
executions of a system — essentially, a formal statement about what the system should do.*

### Property 1: AudioBuffer Read/Write round trip

*For any* sequence of bytes written to an `AudioBuffer` of sufficient capacity, reading
back the same number of bytes SHALL return the originally written bytes without deadlocking
or returning an error.

**Validates: Requirements 1.1, 1.2, 1.3, 1.4**

### Property 2: AudioBuffer Available is consistent with Read

*For any* `AudioBuffer`, the value returned by `Available()` SHALL equal the number of
bytes that a subsequent `Read(Available())` call would successfully return without error.

**Validates: Requirements 1.1, 1.2, 1.3, 1.4**

### Property 3: UDP buffer size is set to expected value

*For any* `Server` configuration, the `bufSize` chosen in `dialUDP` SHALL be
`s.cfg.UDPBufferSize` when it is positive, and `udpSocketBufferSize` otherwise.
The chosen value SHALL be applied exactly once to both `ReadBuffer` and `WriteBuffer`.

**Validates: Requirements 2.1, 2.2, 2.3**

---

## Error Handling

| Fix | Error condition | Handling |
|-----|----------------|----------|
| 1 | `Read` on empty buffer | Returns `io.EOF` — unchanged |
| 2 | `SetReadBuffer`/`SetWriteBuffer` fails | Result ignored with `_` — unchanged |
| 3 | `/healthz` or `/capabilities` fetch fails | Show error state; keep polling |
| 4 | `/healthz` fetch fails | Show "● Offline" indicator |
| 4 | `/auth/profile` fetch fails or unauthenticated | Show "—" for storage |

---

## Testing Strategy

### Unit tests (Go)

**Fix 1** — Add tests in a new `audio_test.go` (or extend if one exists):
- `TestAudioBufferReadWrite`: write N bytes, read N bytes, assert equality.
- `TestAudioBufferAvailableConsistency`: write, check `Available()`, read, check `Available()` is 0.
- `TestAudioBufferNoDeadlock`: run `Write` and `Read` concurrently in goroutines; must not hang (use `testing.T` timeout via `-timeout` flag).

**Fix 2** — Logic is pure enough to unit test directly:
- `TestDialUDPBufferSizeDefault`: when `cfg.UDPBufferSize == 0`, verify the fallback value is used.
- `TestDialUDPBufferSizeConfigured`: when `cfg.UDPBufferSize > 0`, verify configured value is used.
  (These can be implemented by extracting the selection logic into a pure helper `chooseUDPBufSize(cfg, fallback int) int` and testing that.)

### Property-based tests (Go)

Use the standard `testing/quick` package (already available in Go's stdlib; no new dependency needed).

**Property 1 & 2** — `TestAudioBufferRoundTrip` using `quick.Check`:
- Generator produces random byte slices up to `maxBufferSize`.
- Property: `write(data); read(len(data))` returns identical bytes without error.
- Property: `Available()` before read equals `len(data)`; `Available()` after read equals 0.
- Run with at least 100 iterations (the default for `quick.Check` is 100).
- Tag: `// Feature: freecompute-fixes, Property 1: AudioBuffer round trip`

**Property 3** — `TestUDPBufSizeSelection` using `quick.Check`:
- Generator produces random `(cfgSize int, fallback int)` pairs where both are positive.
- Property: `chooseUDPBufSize(cfgSize, fallback) == cfgSize` when `cfgSize > 0`.
- Property: `chooseUDPBufSize(0, fallback) == fallback`.
- Tag: `// Feature: freecompute-fixes, Property 3: UDP buffer size selection`

### Frontend tests

Fixes 3 and 4 are React components with async data fetching. No JS test framework is
configured in this project. The recommended approach is manual verification plus
example-based tests once a framework (e.g., Vitest + React Testing Library) is set up:

- Render `<TaskManager />` with a mocked `getGatewayUrl`; assert status text updates.
- Render `<DesktopWidgets />` with mocked `apiFetch`; assert storage string updates.
- Assert intervals are cleared on unmount (no memory leaks).

Because no test framework exists today, these are listed as optional sub-tasks in the
task list with a note to add them when Vitest is configured.
