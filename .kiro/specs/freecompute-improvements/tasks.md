# Implementation Plan: FreeCompute Improvements

## Overview

Eight targeted changes: four Go bug fixes in the gateway, two React/TypeScript UI additions in the WebOS frontend, and two documentation updates. Tasks are ordered: backend fixes first (R1–R4), then frontend additions (R5–R6), then docs (R7–R8).

---

## Tasks

- [ ] 1. Fix AudioBuffer deadlock (`apps/gateway/internal/audio/audio.go`)
  - [x] 1.1 Add `availableLocked()` helper method to `AudioBuffer`
    - Add unexported method `func (b *AudioBuffer) availableLocked() int` that computes `b.writePos - b.readPos` (or wrap-around equivalent) without touching `b.mutex`
    - Place the method immediately above the existing `Available()` method
    - _Requirements: R1.1_

  - [ ] 1.2 Update `Read()` to call `availableLocked()` instead of `Available()`
    - Inside `Read()`, replace `b.Available()` with `b.availableLocked()` — the lock is already held at that point
    - Verify the surrounding lock/unlock structure is unchanged
    - _Requirements: R1.2_

  - [ ] 1.3 Update `Available()` to delegate to `availableLocked()`
    - Replace the inline arithmetic in `Available()` with a call to `b.availableLocked()` (still under `b.mutex.Lock()`)
    - _Requirements: R1.3_

  - [ ] 1.4 Write property test for concurrent AudioBuffer access
    - **Property 1: Available() is lock-safe**
    - Spin up two goroutines: one calling `Write` with random data, one calling `Read` — verify no deadlock and `Available()` always returns a value in `[0, size)`
    - _Requirements: R1.1, R1.2, R1.3_

- [ ] 2. Verify and harden signal store sweep (`apps/gateway/internal/tunnel/signaling.go`, `server.go`)
  - [ ] 2.1 Confirm `sweepLoop` is started and add lifecycle comment
    - Read `NewServer()` to confirm `go sigStore.sweepLoop()` is present (it is)
    - Add a short comment above the call explaining the goroutine's purpose and that it runs for the lifetime of the process
    - _Requirements: R2.1, R2.2_

  - [ ] 2.2 Add context parameter to `sweepLoop` for clean shutdown
    - Change `func (s *signalStore) sweepLoop()` to `func (s *signalStore) sweepLoop(ctx context.Context)`
    - Replace the `for range ticker.C` loop with a `select` on `ctx.Done()` and `ticker.C`
    - Update the call in `NewServer` to `go sigStore.sweepLoop(context.Background())` (or pass the server context if it is available at construction time)
    - _Requirements: R2.2, R2.3_

- [ ] 3. Remove redundant UDP socket buffer calls (`apps/gateway/internal/tunnel/udp.go`)
  - [ ] 3.1 Delete the first `SetReadBuffer`/`SetWriteBuffer` pair from `dialUDP`
    - In `dialUDP`, remove the two lines that call `upstream.SetReadBuffer(udpSocketBufferSize)` and `upstream.SetWriteBuffer(udpSocketBufferSize)`
    - Leave the second pair (`s.cfg.UDPBufferSize`) untouched
    - _Requirements: R3.1, R3.2, R3.3_

  - [ ] 3.2 Write unit test asserting buffer size is applied exactly once
    - **Property 3: dialUDP applies buffer size exactly once**
    - Use a mock `net.UDPConn` or inspect the call count — verify `SetReadBuffer` and `SetWriteBuffer` are each called exactly once with `s.cfg.UDPBufferSize`
    - _Requirements: R3.1, R3.2_

- [ ] 4. Guard `CalculateRMS` against empty input (`apps/gateway/internal/audio/audio.go`)
  - [ ] 4.1 Add zero-length guard to `CalculateRMS`
    - At the top of `CalculateRMS`, add `if len(samples) == 0 { return 0.0 }`
    - _Requirements: R4.1_

  - [ ] 4.2 Write property test for `CalculateRMS` totality
    - **Property 2: CalculateRMS is total**
    - Generate random float32 slices of length 0–1000; assert the result is always finite (not NaN, not Inf)
    - _Requirements: R4.1, R4.2_

- [ ] 5. Checkpoint — run Go tests
  - Run `go test ./...` from `apps/gateway/` and confirm the two existing tests still pass. Ask the user if any failures arise.

- [ ] 6. Implement TaskManager component (`apps/frontend/app/webos/apps/task-manager/TaskManager.tsx`)
  - [ ] 6.1 Create `TaskManager.tsx` with window list and close buttons
    - Create the file at `apps/frontend/app/webos/apps/task-manager/TaskManager.tsx`
    - Accept props `windows: AppWindow[]` and `onClose: (id: string) => void`
    - Render a row per window with the window title and a close `<button>` containing `<X size={14} />` from lucide-react
    - When `windows` is empty, render a "No apps are currently open." placeholder message
    - Add `aria-label={`Close ${win.title}`}` to each close button for accessibility
    - _Requirements: R5.1, R5.2, R5.3, R5.4_

  - [ ] 6.2 Wire `task-manager` into `appComponents` in `Desktop.tsx`
    - Import `TaskManager` from `../apps/task-manager/TaskManager`
    - Move the `appComponents` object definition inside the `Desktop` function body so that `windows` and `closeWindow` are in scope
    - Add `'task-manager': () => <TaskManager windows={windows} onClose={closeWindow} />` to `appComponents`
    - _Requirements: R5.5_

  - [ ] 6.3 Write property test for TaskManager rendering
    - **Property 4: TaskManager renders all open windows**
    - Generate an array of arbitrary `AppWindow` objects; mount `TaskManager` and assert one close button per window, each with the correct `aria-label`
    - _Requirements: R5.2, R5.3_

- [ ] 7. Add desktop icon grid to `Desktop.tsx`
  - [ ] 7.1 Add `DesktopIconGrid` sub-component inside `Desktop.tsx`
    - Define the `desktopIcons` array with entries for: Browser (`browser`), Terminal (`terminal`), Files (`files`), Settings (`settings`), Remote Desktop (`remote-desktop`), Connection (`connection`)
    - Implement the `DesktopIconGrid` function component with:
      - A `useState<string | null>(null)` for the selected icon ID
      - `onClick` handler that sets the selected state
      - `onDoubleClick` handler that clears selected state and calls `onOpenApp`
      - CSS grid layout with `gridTemplateColumns: 'repeat(2, 80px)'` positioned at `top: 16, left: 16`
      - Visual highlight (background + border) on the selected icon
      - Icon label text below each icon
    - _Requirements: R6.1, R6.2, R6.3, R6.4, R6.5_

  - [ ] 7.2 Mount `DesktopIconGrid` in the Desktop JSX
    - Inside the `flex: 1` div that wraps `WindowManager`, add `<DesktopIconGrid onOpenApp={openApp} />` as a sibling before `<WindowManager />`
    - The icon grid uses `zIndex: 0`; windows start at `zIndex: 10`, so icons always render beneath open windows
    - _Requirements: R6.1_

  - [ ] 7.3 Write property test for desktop icon grid completeness
    - **Property 5: Desktop icon grid is always rendered**
    - Mount `Desktop` (or the isolated `DesktopIconGrid`) with zero and several open windows; assert six icon items are present in the DOM in both cases
    - _Requirements: R6.1, R6.2_

- [ ] 8. Checkpoint — TypeScript type check
  - Run `npm run typecheck` from the repo root and resolve any type errors introduced by steps 6–7. Ask the user if errors cannot be resolved automatically.

- [ ] 9. Update `docs/PERFORMANCE_OPTIMIZATION.md`
  - [ ] 9.1 Add ✅ prefix to each completed optimization section heading
    - Prefix each of the 12 numbered section headings (e.g., `## 1. Buffer Pool Adoption` → `## ✅ 1. Buffer Pool Adoption`)
    - _Requirements: R7.1_

  - [ ] 9.2 Add "Status" column to the Priority Summary table
    - Add a `Status` column header to the table
    - Set all rows to `✅ Done`
    - _Requirements: R7.2_

- [ ] 10. Update `docs/ROADMAP.md` Phase 3
  - [ ] 10.1 Mark Phase 3 heading and all bullets as complete
    - Change `## Phase 3 — Performance Optimization` to `## Phase 3 — Performance Optimization ✅`
    - Prefix each bullet point in the Phase 3 section with `✅`
    - _Requirements: R8.1, R8.2_

- [ ] 11. Final checkpoint — Ensure all tests pass
  - Run `go test ./...` from `apps/gateway/` and `npm run typecheck` from the repo root. Confirm zero errors. Ask the user if questions arise.

---

## Task Dependency Graph

```json
{
  "waves": [
    {
      "wave": 1,
      "tasks": ["1.1", "2.1", "3.1", "4.1", "9.1", "10.1"],
      "description": "Leaf-level backend fixes and doc edits — all independent"
    },
    {
      "wave": 2,
      "tasks": ["1.2", "1.3", "2.2", "3.2", "4.2", "9.2"],
      "description": "Dependent sub-tasks and optional tests for wave 1 items"
    },
    {
      "wave": 3,
      "tasks": ["1.4", "5"],
      "description": "Optional AudioBuffer property test; Go checkpoint"
    },
    {
      "wave": 4,
      "tasks": ["6.1", "7.1"],
      "description": "New frontend components — independent of each other and backend"
    },
    {
      "wave": 5,
      "tasks": ["6.2", "7.2"],
      "description": "Wire components into Desktop.tsx"
    },
    {
      "wave": 6,
      "tasks": ["6.3", "7.3", "8"],
      "description": "Optional frontend property tests; TypeScript typecheck checkpoint"
    },
    {
      "wave": 7,
      "tasks": ["11"],
      "description": "Final checkpoint — all tests pass"
    }
  ]
}
```

---

## Notes

- Tasks marked with `*` are optional and can be skipped for a faster iteration.
- Backend tasks (1–5) have no frontend dependencies and can be executed in parallel with frontend tasks (6–8).
- Documentation tasks (9–10) are purely textual and carry no code risk.
- Property tests require a Go or JavaScript test framework to be configured; skip if no framework is set up.
