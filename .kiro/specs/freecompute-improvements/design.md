# Design Document — FreeCompute Improvements

## Overview

Eight changes across two layers: four Go concurrency/correctness fixes in `apps/gateway/`, two React/TypeScript UI additions in `apps/frontend/`, and two Markdown documentation updates. Each change is self-contained with no cross-feature dependencies.

---

## Architecture

The changes touch three distinct sub-systems:

1. **Go audio package** (`apps/gateway/internal/audio/audio.go`): Fixes a mutex re-entry bug and a divide-by-zero in two standalone functions. No new dependencies or interfaces.
2. **Go tunnel package** (`apps/gateway/internal/tunnel/`): Removes dead code in `udp.go` and hardens the signal store goroutine lifecycle in `server.go`/`signaling.go`. No new dependencies.
3. **Next.js 15 WebOS frontend** (`apps/frontend/app/webos/`): Adds two new React components (`TaskManager`, `DesktopIconGrid`) wired into the existing `Desktop.tsx`. Uses lucide-react icons already imported in the file.

All changes are additive or surgical replacements. No new packages are introduced. No API contracts change.

---

## Components and Interfaces

### AudioBuffer (modified — `apps/gateway/internal/audio/audio.go`)

```go
type AudioBuffer struct {
    data     []byte
    writePos int
    readPos  int
    size     int
    mutex    sync.Mutex
    ready    bool
}

// New unexported helper — call only while mutex is held
func (b *AudioBuffer) availableLocked() int

// Existing public method — now delegates to availableLocked()
func (b *AudioBuffer) Available() int

// Existing method — fixed to call availableLocked() instead of Available()
func (b *AudioBuffer) Read(size int) ([]byte, error)
```

### CalculateRMS (modified — `apps/gateway/internal/audio/audio.go`)

```go
// Guard added: returns 0.0 when len(samples) == 0
func CalculateRMS(samples []float32) float32
```

### signalStore.sweepLoop (modified signature — `apps/gateway/internal/tunnel/signaling.go`)

```go
// Context parameter added for clean shutdown
func (s *signalStore) sweepLoop(ctx context.Context)
```

### Server.dialUDP (modified — `apps/gateway/internal/tunnel/udp.go`)

```go
// First pair of SetReadBuffer/SetWriteBuffer calls removed
func (s *Server) dialUDP(route *Route) (*net.UDPConn, error)
```

### TaskManager (new — `apps/frontend/app/webos/apps/task-manager/TaskManager.tsx`)

```tsx
interface TaskManagerProps {
  windows: AppWindow[];
  onClose: (id: string) => void;
}
export default function TaskManager(props: TaskManagerProps): JSX.Element
```

### DesktopIconGrid (new — inline in `Desktop.tsx`)

```tsx
interface DesktopIconGridProps {
  onOpenApp: (id: string) => void;
}
function DesktopIconGrid(props: DesktopIconGridProps): JSX.Element
```

---

## Data Models

No new data models are introduced. The changes operate on existing types:

- `AudioBuffer` — existing struct, two methods modified, one helper added
- `signalStore` — existing struct, `sweepLoop` signature updated
- `AppWindow` — existing interface in `apps/frontend/app/webos/system/types.ts`, passed through unchanged to `TaskManager`

---

## Error Handling

| Change | Error scenario | Handling |
|--------|---------------|----------|
| R1 AudioBuffer | Concurrent read+write | Deadlock removed; `Read` returns `io.EOF` when buffer is empty |
| R3 dialUDP | `SetReadBuffer`/`SetWriteBuffer` returns error | Errors are already ignored with `_ =`; no change |
| R4 CalculateRMS | Empty input | Returns `0.0` early; no panic, no NaN |
| R5 TaskManager | `windows` is empty | Renders "No apps are currently open." placeholder |
| R2 sweepLoop | `ctx` cancelled | Goroutine exits via `select`; rooms are not swept on exit (acceptable) |

---

## Testing Strategy

- **Unit tests** (Go): Run `go test ./...` from `apps/gateway/` — currently 2 tests pass; no regressions expected.
- **Type checking** (TypeScript): Run `npm run typecheck` from the repo root after all frontend changes.
- **Property tests** (optional): See tasks 1.4, 3.2, 4.2, 6.3, 7.3 — these require a test framework to be configured.

---

## R1 — Fix AudioBuffer Deadlock

**File:** `apps/gateway/internal/audio/audio.go`

### Problem

`AudioBuffer.Read()` acquires `b.mutex.Lock()` at the top of the method, then calls `b.Available()`. `Available()` also acquires `b.mutex.Lock()`. This is a recursive lock on a `sync.Mutex` — Go mutexes are not reentrant — so the goroutine deadlocks immediately.

The same pattern appears in `GetStats`, but there `Available()` is called without the `AudioBuffer` mutex held, so it is safe.

### Solution

Introduce an unexported `availableLocked()` helper that performs the calculation without locking:

```go
// availableLocked computes available bytes. Must be called with b.mutex held.
func (b *AudioBuffer) availableLocked() int {
    if b.writePos >= b.readPos {
        return b.writePos - b.readPos
    }
    return b.size - b.readPos + b.writePos
}
```

Update `Read()` to call `availableLocked()`:

```go
func (b *AudioBuffer) Read(size int) ([]byte, error) {
    b.mutex.Lock()
    defer b.mutex.Unlock()

    if !b.ready {
        return nil, io.EOF
    }

    available := b.availableLocked()   // ← was b.Available()
    if available == 0 {
        return nil, io.EOF
    }
    // ... rest unchanged
}
```

Keep `Available()` as the public entry point, but have it call `availableLocked()` under the lock:

```go
func (b *AudioBuffer) Available() int {
    b.mutex.Lock()
    defer b.mutex.Unlock()
    return b.availableLocked()
}
```

`GetStats` in `AudioStreamer` already calls `session.Buffer.Available()` from outside any `AudioBuffer` lock, so it is unaffected.

---

## R2 — Verify Signal Store Sweep Is Running

**File:** `apps/gateway/internal/tunnel/server.go`

### Current State

Reading `NewServer()` confirms that `go sigStore.sweepLoop()` is already present:

```go
sigStore := &signalStore{rooms: map[string]*signalRoom{}}
go sigStore.sweepLoop()                    // ← already there
```

The `sweepLoop` method in `signaling.go` runs on a `time.Ticker` and never exits (no context cancellation). This is acceptable for a long-lived server process, but the goroutine leaks if the server restarts without a full process exit. This requirement is essentially **already satisfied** by the existing code.

### Verification Task

The implementation task for R2 is to confirm the existing code is correct, add a code comment explaining the lifecycle, and optionally pass a `context.Context` to `sweepLoop` so it can exit cleanly on server shutdown.

**Optional improvement** — context-aware sweep:

```go
// In signalStore:
func (s *signalStore) sweepLoop(ctx context.Context) {
    ticker := time.NewTicker(signalCleanupInterval)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            s.sweep()
        }
    }
}
```

Call site in `NewServer` or `Start`:

```go
go sigStore.sweepLoop(ctx)   // ctx is the server's lifecycle context
```

Since `Start()` receives a `context.Context`, the cleanest approach is to pass the context through and start the goroutine in `Start()` rather than `NewServer()`. However, given that this is a pre-alpha project, the simpler fix is to just add the context parameter and call it from `NewServer` with `context.Background()` — matching the existing behaviour while documenting the pattern.

---

## R3 — Remove Redundant UDP Socket Buffer Calls

**File:** `apps/gateway/internal/tunnel/udp.go`

### Problem

`dialUDP()` calls `SetReadBuffer` and `SetWriteBuffer` twice:

```go
_ = upstream.SetReadBuffer(udpSocketBufferSize)   // first call  ← remove
_ = upstream.SetWriteBuffer(udpSocketBufferSize)  // first call  ← remove
_ = upstream.SetReadBuffer(s.cfg.UDPBufferSize)   // second call ← keep
_ = upstream.SetWriteBuffer(s.cfg.UDPBufferSize)  // second call ← keep
```

The first pair uses the package-level `udpSocketBufferSize` default. The second pair immediately overwrites that with `s.cfg.UDPBufferSize`. The effective value is always `s.cfg.UDPBufferSize`, but the intent is unclear and the first pair of calls is dead code.

### Solution

Remove the first pair of calls:

```go
func (s *Server) dialUDP(route *Route) (*net.UDPConn, error) {
    targetAddr, err := net.ResolveUDPAddr("udp", route.Target)
    if err != nil {
        return nil, fmt.Errorf("resolve udp target route=%s target=%s: %w", route.ID, route.Target, err)
    }

    upstream, err := net.DialUDP("udp", nil, targetAddr)
    if err != nil {
        return nil, err
    }
    _ = upstream.SetReadBuffer(s.cfg.UDPBufferSize)
    _ = upstream.SetWriteBuffer(s.cfg.UDPBufferSize)

    return upstream, nil
}
```

---

## R4 — Guard CalculateRMS Against Empty Input

**File:** `apps/gateway/internal/audio/audio.go`

### Problem

```go
func CalculateRMS(samples []float32) float32 {
    sum := float32(0)
    for _, sample := range samples {
        sum += sample * sample
    }
    return float32(math.Sqrt(float64(sum / float32(len(samples)))))
    //                                              ^^^^^^^^^^^^
    //                                              division by zero when len == 0
    //                                              produces NaN
}
```

When called with an empty slice, `float32(len(samples))` is `0.0`, producing `+Inf` or `NaN` from the divide, which then propagates as NaN through `math.Sqrt`.

### Solution

Add a zero-length guard:

```go
func CalculateRMS(samples []float32) float32 {
    if len(samples) == 0 {
        return 0.0
    }
    sum := float32(0)
    for _, sample := range samples {
        sum += sample * sample
    }
    return float32(math.Sqrt(float64(sum / float32(len(samples)))))
}
```

---

## R5 — Implement Missing TaskManager App

### Files

- **New:** `apps/frontend/app/webos/apps/task-manager/TaskManager.tsx`
- **Modify:** `apps/frontend/app/webos/desktop/Desktop.tsx`

### TaskManager Component

```tsx
'use client';

import { X } from 'lucide-react';
import type { AppWindow } from '../../system/types';

interface TaskManagerProps {
  windows: AppWindow[];
  onClose: (id: string) => void;
}

export default function TaskManager({ windows, onClose }: TaskManagerProps) {
  if (windows.length === 0) {
    return (
      <div style={{ padding: 24, color: '#888', textAlign: 'center' }}>
        No apps are currently open.
      </div>
    );
  }

  return (
    <div style={{ padding: 16 }}>
      <div style={{ fontSize: 12, color: '#888', marginBottom: 8 }}>
        {windows.length} open window{windows.length !== 1 ? 's' : ''}
      </div>
      {windows.map((win) => (
        <div
          key={win.id}
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '8px 12px',
            marginBottom: 4,
            background: 'rgba(255,255,255,0.04)',
            border: '1px solid rgba(255,255,255,0.08)',
            borderRadius: 6,
          }}
        >
          <span style={{ fontSize: 13, color: '#ccc' }}>{win.title}</span>
          <button
            onClick={() => onClose(win.id)}
            aria-label={`Close ${win.title}`}
            style={{
              background: 'transparent',
              border: 'none',
              cursor: 'pointer',
              color: '#888',
              display: 'flex',
              alignItems: 'center',
              padding: 4,
              borderRadius: 4,
            }}
          >
            <X size={14} />
          </button>
        </div>
      ))}
    </div>
  );
}
```

### Desktop.tsx Changes

1. Import `TaskManager`:

```tsx
import TaskManager from '../apps/task-manager/TaskManager';
```

2. Add entry to `appComponents` (replace the missing `task-manager` entry):

```tsx
'task-manager': () => <TaskManager windows={windows} onClose={closeWindow} />,
```

`windows` and `closeWindow` are already in scope inside the `Desktop` function body. Because `appComponents` is defined as a `const` outside the component, it needs to be moved inside `Desktop` (or converted to a factory function that receives these props). The cleanest approach is to convert `appComponents` to a function `getAppComponents(windows, closeWindow)` defined inside `Desktop`:

```tsx
const appComponents: Record<string, () => ReactNode> = {
  // ... existing entries
  'task-manager': () => <TaskManager windows={windows} onClose={closeWindow} />,
};
```

This object is recreated on each render, which is acceptable because the window manager already re-renders on state changes. If performance becomes a concern, `useMemo` can be applied.

---

## R6 — Add Desktop Icon Grid

**File:** `apps/frontend/app/webos/desktop/Desktop.tsx`

### DesktopIconGrid Component

Add a new sub-component inside `Desktop.tsx` (or as a separate file at `apps/frontend/app/webos/desktop/DesktopIconGrid.tsx`):

```tsx
const desktopIcons = [
  { id: 'browser',        label: 'Browser',        icon: <Monitor size={32} />,     color: '#1abc9c' },
  { id: 'terminal',       label: 'Terminal',        icon: <TerminalIcon size={32} />, color: '#3498db' },
  { id: 'files',          label: 'Files',           icon: <Folder size={32} />,      color: '#e67e22' },
  { id: 'settings',       label: 'Settings',        icon: <SettingsIcon size={32} />,color: '#95a5a6' },
  { id: 'remote-desktop', label: 'Remote Desktop',  icon: <Monitor size={32} />,     color: '#9b59b6' },
  { id: 'connection',     label: 'Connection',      icon: <Link2 size={32} />,       color: '#e74c3c' },
];

function DesktopIconGrid({ onOpenApp }: { onOpenApp: (id: string) => void }) {
  const [selected, setSelected] = React.useState<string | null>(null);

  return (
    <div
      style={{
        position: 'absolute',
        top: 16,
        left: 16,
        display: 'grid',
        gridTemplateColumns: 'repeat(2, 80px)',
        gap: 16,
        zIndex: 0,
      }}
    >
      {desktopIcons.map((app) => (
        <div
          key={app.id}
          onClick={() => setSelected(app.id)}
          onDoubleClick={() => { setSelected(null); onOpenApp(app.id); }}
          style={{
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            gap: 6,
            padding: 8,
            borderRadius: 8,
            cursor: 'pointer',
            background: selected === app.id
              ? 'rgba(255,255,255,0.12)'
              : 'transparent',
            border: selected === app.id
              ? '1px solid rgba(255,255,255,0.2)'
              : '1px solid transparent',
            userSelect: 'none',
          }}
        >
          <span style={{ color: app.color }}>{app.icon}</span>
          <span style={{ fontSize: 11, color: '#ccc', textAlign: 'center', lineHeight: 1.2 }}>
            {app.label}
          </span>
        </div>
      ))}
    </div>
  );
}
```

Add `<DesktopIconGrid onOpenApp={openApp} />` inside the `Desktop` return JSX, inside the `flex: 1` container (same level as `WindowManager`):

```tsx
<div style={{ flex: 1, position: 'relative' }}>
  <DesktopIconGrid onOpenApp={openApp} />
  <WindowManager ... />
</div>
```

The icon grid has `zIndex: 0`, and windows start at `zIndex: 10` (per `nextZRef.current = 10`), so windows always render above icons.

---

## R7 — Update PERFORMANCE_OPTIMIZATION.md

**File:** `docs/PERFORMANCE_OPTIMIZATION.md`

All twelve items listed in the document are implemented. Each section heading gets a ✅ prefix. The Priority Summary table gains a "Status" column:

| Priority | Optimization | Effort | Impact | Risk | Status |
|----------|-------------|--------|--------|------|--------|
| P0 | Fix dual HostAllocator bug | 1 day | Bug fix | Low | ✅ Done |
| P0 | UDP client map cleanup | 0.5 day | Memory leak | Low | ✅ Done |
| … | … | … | … | … | ✅ Done |

---

## R8 — Update ROADMAP.md Phase 3

**File:** `docs/ROADMAP.md`

Change the Phase 3 heading to `## Phase 3 — Performance Optimization ✅` and prefix each bullet with ✅.

---

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system — essentially, a formal statement about what the system should do.*

### Property 1: Available() is lock-safe

For any `AudioBuffer` with arbitrary `writePos`, `readPos`, and `size`, calling `Available()` from one goroutine while `Read()` or `Write()` is executing concurrently SHALL NOT deadlock and SHALL return a value in the range `[0, size)`.

**Validates: Requirements 1.1, 1.2, 1.3**

### Property 2: CalculateRMS is total

For any slice of float32 values (including the empty slice), `CalculateRMS` SHALL return a finite, non-NaN float32 value.

**Validates: Requirements 4.1, 4.2**

### Property 3: dialUDP applies buffer size exactly once

For any call to `dialUDP`, the effective read and write buffer sizes set on the returned `*net.UDPConn` SHALL equal `s.cfg.UDPBufferSize`, and each of `SetReadBuffer` and `SetWriteBuffer` SHALL be called exactly once.

**Validates: Requirements 3.1, 3.2, 3.3**

### Property 4: TaskManager renders all open windows

For any non-empty list of `AppWindow` values passed to `TaskManager`, the rendered output SHALL contain one close button per window, and each close button's `aria-label` SHALL reference the corresponding window's title.

**Validates: Requirements 5.1, 5.2, 5.3**

### Property 5: Desktop icon grid is always rendered

For any desktop state (zero or more open windows), the `DesktopIconGrid` SHALL always be present in the DOM with exactly six icon items.

**Validates: Requirements 6.1, 6.2**
