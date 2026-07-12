# Requirements Document

## Introduction

This spec covers five targeted fixes and improvements to the FreeCompute codebase identified during a codebase audit. Three are Go backend fixes in `apps/gateway/`, one is a missing TypeScript/React component in `apps/frontend/`, and one is a documentation update. Each item is a self-contained, low-risk change with clear acceptance criteria.

## Glossary

- **AudioBuffer**: The ring-buffer struct in `apps/gateway/internal/audio/audio.go` used to queue raw audio data between writer and reader goroutines.
- **AudioBuffer.Read**: The `Read(size int)` method on `AudioBuffer` that acquires `b.mutex` and reads from the ring buffer.
- **AudioBuffer.Available**: The `Available() int` method on `AudioBuffer` that also acquires `b.mutex` to compute the number of readable bytes.
- **availableLocked**: A new unexported helper method on `AudioBuffer` that computes readable bytes without acquiring the mutex; intended for use inside methods that already hold `b.mutex`.
- **dialUDP**: The `dialUDP(route *Route)` method in `apps/gateway/internal/tunnel/udp.go` that dials an upstream UDP connection.
- **udpSocketBufferSize**: A package-level fallback constant in `udp.go` for the UDP socket read/write buffer size (32 MB).
- **UDPBufferSize**: The `s.cfg.UDPBufferSize` field from the server config, intended to be the authoritative socket buffer size when set.
- **TaskManager**: A new WebOS React component to be created at `apps/frontend/app/webos/apps/task-manager/TaskManager.tsx`.
- **Desktop.tsx**: The WebOS desktop shell at `apps/frontend/app/webos/desktop/Desktop.tsx` that manages open windows and registers app components.
- **AppWindow**: The `AppWindow` interface defined in `apps/frontend/app/webos/system/types.ts` representing an open window on the WebOS desktop.
- **PERFORMANCE_OPTIMIZATION.md**: The audit document at `docs/PERFORMANCE_OPTIMIZATION.md` tracking the status of all identified optimizations.

---

## Requirements

### Requirement 1: AudioBuffer Deadlock Fix

**User Story:** As a gateway operator, I want the audio streaming subsystem to be free of deadlocks, so that audio sessions do not hang indefinitely under load.

#### Acceptance Criteria

1. WHEN `AudioBuffer.Read` is executing and holds `b.mutex`, THE `AudioBuffer` SHALL compute available bytes using `availableLocked()` without attempting to re-acquire `b.mutex`.
2. THE `AudioBuffer` SHALL expose an unexported `availableLocked()` method that computes `writePos - readPos` (or the wrapped equivalent) without locking.
3. THE `AudioBuffer.Available()` public method SHALL continue to acquire `b.mutex` and delegate to `availableLocked()` for its computation.
4. WHEN `AudioBuffer.Read` is called concurrently with `AudioBuffer.Write`, THE `AudioBuffer` SHALL complete both operations without deadlocking.

---

### Requirement 2: UDP Double SetBuffer Fix

**User Story:** As a gateway operator, I want UDP upstream connections to use the configured buffer size without redundant system calls, so that the intended buffer size is applied correctly and no unnecessary syscalls are made.

#### Acceptance Criteria

1. WHEN `dialUDP` dials an upstream UDP connection, THE `dialUDP` function SHALL call `SetReadBuffer` exactly once on the upstream connection.
2. WHEN `dialUDP` dials an upstream UDP connection, THE `dialUDP` function SHALL call `SetWriteBuffer` exactly once on the upstream connection.
3. THE single `SetReadBuffer` call SHALL use `s.cfg.UDPBufferSize` as the buffer size argument.
4. THE single `SetWriteBuffer` call SHALL use `s.cfg.UDPBufferSize` as the buffer size argument.
5. THE `dialUDP` function SHALL NOT call `SetReadBuffer` or `SetWriteBuffer` with `udpSocketBufferSize` as the argument.

---

### Requirement 3: TaskManager WebOS Application

**User Story:** As a WebOS desktop user, I want a Task Manager application, so that I can see which windows are currently open and monitor simulated system resource usage.

#### Acceptance Criteria

1. THE `Desktop.tsx` SHALL register a `task-manager` entry in the `appComponents` map that renders the `TaskManager` component.
2. WHEN the Task Manager window is open, THE `TaskManager` component SHALL display a list of all currently open `AppWindow` instances, showing at minimum each window's `title` and `app` identifier.
3. WHEN no windows are open, THE `TaskManager` component SHALL display an empty-state message indicating no running applications.
4. WHEN the Task Manager window is open, THE `TaskManager` component SHALL display a mock system resource panel showing a simulated CPU percentage and a simulated RAM percentage.
5. THE simulated CPU and RAM values SHALL update on a regular interval (at minimum every 2 seconds) to give the appearance of live monitoring.
6. THE `TaskManager` component SHALL accept the current `AppWindow[]` list as a prop passed down from `Desktop.tsx`.
7. WHEN `npm run typecheck` is executed from the repository root, THE TypeScript compiler SHALL report zero type errors related to the `TaskManager` component or its integration in `Desktop.tsx`.

---

### Requirement 4: PERFORMANCE_OPTIMIZATION.md Update

**User Story:** As a developer reading the performance audit document, I want the Priority Summary table to accurately reflect which optimizations are complete, so that I can quickly identify remaining work.

#### Acceptance Criteria

1. THE `PERFORMANCE_OPTIMIZATION.md` Priority Summary table SHALL mark all previously listed optimizations (buffer pools, heap queue, agent pool O(1), UDP sweep, WebSocket frame pooling, dual allocator fix, audio bulk copy, WebRTC stats parsing, signal store as server field, shared proxy director) as DONE.
2. THE `PERFORMANCE_OPTIMIZATION.md` Priority Summary table SHALL include a new row for the AudioBuffer deadlock fix (Requirement 1) marked as DONE.
3. THE `PERFORMANCE_OPTIMIZATION.md` Priority Summary table SHALL include a new row for the UDP double SetBuffer fix (Requirement 2) marked as DONE.
4. THE `PERFORMANCE_OPTIMIZATION.md` document SHALL note in the body text of each relevant existing section that the optimization described has been implemented.
