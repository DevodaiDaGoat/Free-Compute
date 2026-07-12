# Implementation Plan: FreeCompute Fixes

## Overview

Four independent bug fixes. Each task maps 1-to-1 to a requirement and can be
implemented, reviewed, and merged independently. Tasks 1 and 2 are Go changes;
tasks 3 and 4 are TypeScript/React changes.

---

## Tasks

- [ ] 1. Fix AudioBuffer deadlock in `audio.go`
  - In `apps/gateway/internal/audio/audio.go`, add a private `availableLocked() int` method on `*AudioBuffer` that contains the current body of `Available()` (the raw `writePos`/`readPos` arithmetic) but does NOT acquire `b.mutex`.
  - Rewrite the public `Available() int` to acquire `b.mutex`, call `availableLocked()`, and return the result.
  - In `Read()`, replace the call to `b.Available()` with `b.availableLocked()` (the lock is already held at that point).
  - No other callers of `Available()` need changing — `GetStats` and any external callers already call the public method without holding the lock.
  - _Requirements: 1.1, 1.2, 1.3, 1.4_

  - [ ]* 1.1 Write property test for AudioBuffer round-trip correctness
    - Create `apps/gateway/internal/audio/audio_test.go`.
    - Use `testing/quick` to generate random `[]byte` values up to `maxBufferSize`.
    - **Property 1: AudioBuffer Read/Write round trip** — for any bytes written, reading them back returns identical bytes without error.
    - **Property 2: AudioBuffer Available is consistent with Read** — `Available()` before read equals bytes written; after read equals 0.
    - Add a concurrency smoke test using `t.Parallel()` goroutines writing and reading simultaneously to confirm no deadlock (requires test `-timeout 5s`).
    - `// Feature: freecompute-fixes, Property 1 & 2: AudioBuffer round trip`
    - _Requirements: 1.1, 1.2, 1.3, 1.4_

- [ ] 2. Fix UDP socket buffer double-set in `udp.go`
  - In `apps/gateway/internal/tunnel/udp.go`, extract a pure helper function:
    ```go
    func chooseUDPBufSize(cfgSize, fallback int) int {
        if cfgSize > 0 {
            return cfgSize
        }
        return fallback
    }
    ```
  - In `dialUDP`, replace the four existing `Set*Buffer` calls with:
    ```go
    bufSize := chooseUDPBufSize(s.cfg.UDPBufferSize, udpSocketBufferSize)
    _ = upstream.SetReadBuffer(bufSize)
    _ = upstream.SetWriteBuffer(bufSize)
    ```
  - _Requirements: 2.1, 2.2, 2.3_

  - [ ]* 2.1 Write property test for UDP buffer size selection
    - Add `TestUDPBufSizeSelection` in `apps/gateway/internal/tunnel/udp_test.go` (create file if it does not exist).
    - Use `testing/quick` to generate random positive `(cfgSize int, fallback int)` pairs.
    - **Property 3: UDP buffer size is set to expected value** — `chooseUDPBufSize(cfgSize, fallback) == cfgSize` when `cfgSize > 0`; `chooseUDPBufSize(0, fallback) == fallback`.
    - `// Feature: freecompute-fixes, Property 3: UDP buffer size selection`
    - _Requirements: 2.1, 2.2, 2.3_

- [ ] 3. Checkpoint — run `go test ./...` from `apps/gateway/`
  - All existing tests must pass. Ensure all tests pass; ask the user if questions arise.

- [ ] 4. Implement Task Manager app component
  - Create directory `apps/frontend/app/webos/apps/task-manager/`.
  - Create `apps/frontend/app/webos/apps/task-manager/TaskManager.tsx`.
  - Import `getGatewayUrl` from `../../boot/BootSequence`.
  - Define state shape:
    ```ts
    interface GatewayStats {
      status: 'loading' | 'ok' | 'error';
      protocols: string[];
      lastUpdated: Date | null;
      errorMessage: string | null;
    }
    ```
  - On mount, fetch `GET /healthz` (no auth) and `GET /capabilities` (no auth). Update state with results.
  - Set a `setInterval` for 5 000 ms that re-fetches `/healthz` and updates `status` and `lastUpdated`.
  - Return `clearInterval` from the `useEffect` cleanup.
  - Render: a dark-themed panel (`rgba(17,17,40,0.85)` background, `1px solid #2a2a4a` border, `borderRadius: 8`) with:
    - "Gateway Status" heading.
    - Status row: label + colored dot (green = ok, red = error, grey = loading).
    - "Protocols" section listing `stats.protocols` as a comma-separated string.
    - "Last updated" timestamp using `toLocaleTimeString()`.
    - If `status === 'error'`, show `errorMessage` in a muted red.
  - In `apps/frontend/app/webos/desktop/Desktop.tsx`:
    - Add `import TaskManagerApp from '../apps/task-manager/TaskManager';` at the top.
    - Add `'task-manager': () => <TaskManagerApp />` to the `appComponents` record.
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6_

  - [ ]* 4.1 Write example tests for TaskManager (requires Vitest to be configured first)
    - Render `<TaskManager />` with `getGatewayUrl` mocked to return a test URL.
    - Mock `fetch` to return `{ status: 'ok' }` for `/healthz` and `{ protocols: ['http', 'websocket'] }` for `/capabilities`.
    - Assert "ok" status indicator is rendered.
    - Mock `fetch` to reject; assert error state is displayed.
    - Assert `clearInterval` is called on unmount.
    - _Requirements: 3.2, 3.3, 3.4, 3.5_

- [ ] 5. Replace hardcoded Desktop widget data with live fetches
  - In `apps/frontend/app/webos/desktop/Desktop.tsx`, convert `DesktopWidgets` from a pure render function to a React component using `useState` and `useEffect`.
  - Add state:
    ```ts
    const [gatewayOnline, setGatewayOnline] = useState<boolean | null>(null);
    const [storage, setStorage] = useState<{ used: number; quota: number } | null>(null);
    ```
  - In a `useEffect` with an empty dependency array:
    - Define `pollHealth` as an async function that calls `fetch(\`${getGatewayUrl()}/healthz\`)`, parses the JSON, and sets `gatewayOnline` to `data.status === 'ok'`; on catch, sets `gatewayOnline` to `false`.
    - Call `pollHealth()` immediately, then schedule it with `setInterval(pollHealth, 5000)`.
    - Fetch the user profile once: `apiFetch('/auth/profile').then(p => setStorage({ used: p.storageUsed, quota: p.storageQuota })).catch(() => {})`.
    - Return a cleanup that calls `clearInterval`.
  - Add a local helper `formatGB(bytes: number): string` that returns `(bytes / 1073741824).toFixed(1) + ' GB'`.
  - Update the render:
    - Gateway row: `gatewayOnline === null` → "..." ; `true` → green "● Online" ; `false` → red "● Offline".
    - Storage row: `storage` is null → "—" ; otherwise `${formatGB(storage.used)} / ${formatGB(storage.quota)} used`.
  - Add the required imports: `useState`, `useEffect` from `'react'`; `apiFetch`, `getGatewayUrl` from `'../boot/BootSequence'`.
  - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7_

  - [ ]* 5.1 Write example tests for DesktopWidgets (requires Vitest to be configured first)
    - Mock `fetch` and `apiFetch`; render `<DesktopWidgets onOpenApp={() => {}} />`.
    - Assert gateway status shows online when `/healthz` returns `{ status: 'ok' }`.
    - Assert gateway status shows offline when fetch throws.
    - Assert storage string matches `formatGB` output when profile is returned.
    - Assert "—" is shown when `apiFetch` rejects.
    - Assert `clearInterval` is called on unmount.
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7_

- [ ] 6. Final checkpoint — verify TypeScript compiles cleanly
  - Run `npm run typecheck` from `apps/frontend/`.
  - Ensure no type errors in `Desktop.tsx` or `TaskManager.tsx`.
  - Ensure all tests pass; ask the user if questions arise.

---

## Notes

- Tasks marked with `*` are optional and can be skipped for a faster fix cycle.
- Property tests (tasks 1.1 and 2.1) use Go's stdlib `testing/quick` — no new dependency.
- Frontend example tests (tasks 4.1 and 5.1) require Vitest + React Testing Library to be set up first; they are marked optional because no JS test framework exists in the project today.
- Each Go fix can be verified immediately with `go test ./apps/gateway/...` from the repo root.
- The `sweepLoop` for `signalStore` is already started in `server.go` (`go sigStore.sweepLoop()`) and requires no fix.
