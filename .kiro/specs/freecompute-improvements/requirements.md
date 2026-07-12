# Requirements Document

## Introduction

This spec covers eight targeted bug fixes and feature additions to the FreeCompute gateway and WebOS frontend. The changes fall into three categories: Go concurrency/correctness bugs (R1–R4), TypeScript/React UI additions (R5–R6), and documentation updates (R7–R8). All items are independent and can be implemented and reviewed in any order, though the recommended order is backend fixes first (low risk), then frontend additions, then docs.

---

## Glossary

- **AudioBuffer**: The ring buffer type defined in `apps/gateway/internal/audio/audio.go` that provides `Write`, `Read`, and `Available` methods protected by a `sync.Mutex`.
- **availableLocked**: An unexported helper method on `AudioBuffer` that computes available bytes without acquiring the mutex — safe to call only while the caller already holds the lock.
- **signalStore**: The struct defined in `apps/gateway/internal/tunnel/signaling.go` that manages WebRTC signaling rooms. A `*signalStore` is a field on `Server`.
- **sweepLoop**: A goroutine method on `signalStore` that periodically removes expired rooms.
- **dialUDP**: The method on `Server` in `apps/gateway/internal/tunnel/udp.go` that creates an upstream UDP connection for a new client.
- **CalculateRMS**: A utility function in `apps/gateway/internal/audio/audio.go` that computes the root-mean-square of a slice of float32 audio samples.
- **Desktop**: The React component in `apps/frontend/app/webos/desktop/Desktop.tsx` that renders the WebOS desktop, window manager, and taskbar.
- **TaskManager**: A new React component to be created at `apps/frontend/app/webos/apps/task-manager/TaskManager.tsx` that lists open windows and allows closing them.
- **DesktopIconGrid**: A new React component to be added inside `Desktop.tsx` that renders a grid of double-clickable app icons on the wallpaper.
- **appComponents**: The `Record<string, () => ReactNode>` map in `Desktop.tsx` that maps app IDs to their rendered content.

---

## Requirements

### R1 — Fix AudioBuffer Deadlock

**User Story:** As a gateway operator, I want audio sessions to remain functional under concurrent read and write operations, so that users do not experience audio freezes or gateway deadlocks.

#### Acceptance Criteria

1. THE `AudioBuffer` SHALL expose an unexported method `availableLocked() int` that computes available bytes using `b.writePos`, `b.readPos`, and `b.size` without acquiring `b.mutex`.
2. WHEN `Read()` is called on an `AudioBuffer`, THE `AudioBuffer.Read` method SHALL call `b.availableLocked()` instead of `b.Available()` to determine available bytes, while `b.mutex` is already held.
3. THE exported `Available()` method SHALL acquire `b.mutex`, call `b.availableLocked()` under that lock, and return the result — maintaining its existing public contract.
4. THE `GetStats` helper in `AudioStreamer` that calls `session.Buffer.Available()` from outside any `AudioBuffer` lock SHALL continue to call the public `Available()` method unchanged.

---

### R2 — Verify Signal Store Sweep Is Running

**User Story:** As a gateway operator, I want signaling rooms to be garbage-collected after they expire, so that the gateway does not accumulate unbounded in-memory state for abandoned WebRTC sessions.

#### Acceptance Criteria

1. WHEN `NewServer` constructs a `*signalStore`, THE `NewServer` function SHALL start `go sigStore.sweepLoop()` before assigning `sigStore` to `server.signalStore`.
2. THE `sweepLoop` method SHALL run until the process exits, periodically calling `sweep()` at the `signalCleanupInterval` (1 minute).
3. THE `sweep` method SHALL delete any room whose `updatedAt` is older than `signalRoomTTL` (10 minutes) and SHALL close the room's `notify` channel before deletion to unblock any waiting long-poll goroutines.

---

### R3 — Remove Redundant UDP Socket Buffer Calls

**User Story:** As a gateway developer, I want `dialUDP` to apply socket buffer sizes exactly once per connection, so that the effective buffer size is deterministic and the code does not silently override the configured value with a stale default.

#### Acceptance Criteria

1. WHEN `dialUDP` is called, THE `Server.dialUDP` method SHALL call `upstream.SetReadBuffer` exactly once, using `s.cfg.UDPBufferSize` as the argument.
2. WHEN `dialUDP` is called, THE `Server.dialUDP` method SHALL call `upstream.SetWriteBuffer` exactly once, using `s.cfg.UDPBufferSize` as the argument.
3. THE first pair of `SetReadBuffer`/`SetWriteBuffer` calls that use the package-level `udpSocketBufferSize` variable SHALL be removed from `dialUDP`.

---

### R4 — Guard CalculateRMS Against Empty Input

**User Story:** As a gateway developer, I want `CalculateRMS` to handle an empty sample slice safely, so that callers do not receive a NaN or panic from a divide-by-zero.

#### Acceptance Criteria

1. WHEN `CalculateRMS` is called with an empty slice (`len(samples) == 0`), THE `CalculateRMS` function SHALL return `0.0` immediately without performing any arithmetic.
2. WHEN `CalculateRMS` is called with a non-empty slice, THE `CalculateRMS` function SHALL compute and return the root-mean-square value as before.

---

### R5 — Implement Missing TaskManager App

**User Story:** As a WebOS user, I want the Task Manager app to show me a list of all open windows so that I can quickly identify and close apps I no longer need.

#### Acceptance Criteria

1. THE system SHALL include a `TaskManager` React component at `apps/frontend/app/webos/apps/task-manager/TaskManager.tsx` that accepts `windows: AppWindow[]` and `onClose: (id: string) => void` as props.
2. WHEN the `TaskManager` component renders, THE component SHALL display each open window as a row containing the window's title and a close button.
3. WHEN the user clicks a close button in the TaskManager, THE `TaskManager` component SHALL call `onClose` with the corresponding window's `id`.
4. WHEN there are no open windows, THE `TaskManager` component SHALL display a message indicating no apps are currently open.
5. THE `Desktop` component SHALL register `task-manager` in `appComponents` by passing the current `windows` state and the `closeWindow` callback into `TaskManager`, so that opening the app from the Start menu renders the component instead of a blank window.

---

### R6 — Add Desktop Icon Grid

**User Story:** As a WebOS user, I want to see app icons on the desktop wallpaper so that I can launch apps by double-clicking them without opening the Start menu.

#### Acceptance Criteria

1. THE `Desktop` component SHALL render a `DesktopIconGrid` sub-component on the wallpaper area, positioned below the top-right system widgets and above the taskbar.
2. THE `DesktopIconGrid` SHALL display icons for the following six apps: Browser, Terminal, Files, Settings, Remote Desktop, and Connection.
3. WHEN a user double-clicks a desktop icon, THE `DesktopIconGrid` SHALL call `onOpenApp` with the corresponding app ID to open the app window.
4. WHEN a user single-clicks a desktop icon, THE `DesktopIconGrid` SHALL apply a selected/highlighted visual state to that icon without opening the app.
5. THE desktop icon grid SHALL use the same icon components already imported in `Desktop.tsx` (`Monitor`, `TerminalIcon`, `Folder`, `SettingsIcon`, `Link2`, `Monitor`) from `lucide-react`, with icon labels beneath each icon.

---

### R7 — Update PERFORMANCE_OPTIMIZATION.md Status

**User Story:** As a developer onboarding to FreeCompute, I want the performance optimization doc to reflect what has already been implemented, so that I do not duplicate work that is already done.

#### Acceptance Criteria

1. WHEN a developer reads `docs/PERFORMANCE_OPTIMIZATION.md`, THE document SHALL mark each completed optimization item with a ✅ prefix in its heading.
2. THE Priority Summary table in `docs/PERFORMANCE_OPTIMIZATION.md` SHALL include a "Status" column indicating whether each item is "✅ Done" or "⬜ Pending".

---

### R8 — Update ROADMAP.md Phase 3 Status

**User Story:** As a developer or contributor, I want the roadmap to reflect the current implementation state, so that I can quickly understand which Phase 3 items are complete.

#### Acceptance Criteria

1. WHEN a developer reads `docs/ROADMAP.md`, THE Phase 3 section SHALL mark every implemented item with a ✅ bullet prefix.
2. THE Phase 3 section heading in `docs/ROADMAP.md` SHALL include a status indicator reflecting that Phase 3 is complete (e.g., `## Phase 3 — Performance Optimization ✅`).
