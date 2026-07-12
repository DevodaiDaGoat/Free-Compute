# Requirements Document

## Introduction

Five concrete bugs found in the FreeCompute codebase: a deadlock in the audio buffer,
a UDP socket-buffer configuration being silently overwritten, a Task Manager app
registered in the WebOS desktop but never implemented, and desktop status widgets
that show hardcoded data instead of live gateway and user-profile values.

## Glossary

- **AudioBuffer**: Ring-buffer type in `apps/gateway/internal/audio/audio.go` used by `AudioSession` to hold in-flight PCM data.
- **Gateway**: The Go HTTP server in `apps/gateway/` that proxies tunnel traffic and exposes REST endpoints.
- **TaskManager**: A WebOS desktop app (app ID `task-manager`) that shows live gateway statistics.
- **DesktopWidgets**: The React component rendered on the WebOS desktop background showing system status.
- **apiFetch**: Helper in `apps/frontend/app/webos/boot/BootSequence.tsx` that calls the Gateway with an auth token.
- **getGatewayUrl**: Helper that returns `NEXT_PUBLIC_GATEWAY_URL` or `http://localhost:8080`.

---

## Requirements

### Requirement 1: Fix AudioBuffer Deadlock

**User Story:** As a developer running the gateway, I want audio streaming to work without deadlocking, so that audio sessions do not hang indefinitely.

#### Acceptance Criteria

1. WHEN `AudioBuffer.Read()` is called, THE `AudioBuffer` SHALL compute available bytes without attempting to acquire `b.mutex` a second time.
2. THE `AudioBuffer` SHALL expose an internal `availableLocked()` method that computes available bytes assuming the caller already holds `b.mutex`.
3. WHEN `AudioBuffer.Available()` is called from outside the buffer, THE `AudioBuffer` SHALL acquire `b.mutex` once and delegate to `availableLocked()`.
4. WHEN `AudioBuffer.Read()` calls the available-bytes check internally, THE `AudioBuffer` SHALL call `availableLocked()` and not `Available()`.

---

### Requirement 2: Fix UDP Socket Buffer Double-Set

**User Story:** As an operator configuring the gateway, I want UDP socket buffer sizes to be applied exactly once using the correct configured value, so that network tuning takes effect as intended.

#### Acceptance Criteria

1. WHEN `dialUDP` creates an upstream UDP connection and `s.cfg.UDPBufferSize` is greater than zero, THE `Server` SHALL set `ReadBuffer` and `WriteBuffer` to `s.cfg.UDPBufferSize` only.
2. WHEN `dialUDP` creates an upstream UDP connection and `s.cfg.UDPBufferSize` is zero, THE `Server` SHALL set `ReadBuffer` and `WriteBuffer` to the package-level `udpSocketBufferSize` fallback value.
3. THE `Server` SHALL NOT call `SetReadBuffer` or `SetWriteBuffer` more than once per socket in `dialUDP`.

---

### Requirement 3: Implement Task Manager App

**User Story:** As a WebOS user, I want a Task Manager app that shows live gateway statistics, so that I can monitor the gateway without leaving the desktop.

#### Acceptance Criteria

1. WHEN a user opens the `task-manager` app, THE `Desktop` SHALL render a `TaskManager` component instead of a blank placeholder.
2. WHEN the `TaskManager` component mounts, THE `TaskManager` SHALL fetch `/healthz` from the Gateway and display the gateway status.
3. WHEN the `TaskManager` component mounts, THE `TaskManager` SHALL fetch `/capabilities` from the Gateway and display the list of supported protocols.
4. WHILE the `TaskManager` is open, THE `TaskManager` SHALL re-fetch `/healthz` every 5 seconds and update the displayed status.
5. WHEN a Gateway fetch fails, THE `TaskManager` SHALL display an error indicator and continue polling.
6. THE `TaskManager` SHALL display the fetched data in a readable table or list layout consistent with the existing WebOS app style.

---

### Requirement 4: Replace Hardcoded Desktop Widgets with Live Data

**User Story:** As a WebOS user, I want the desktop status widget to show real gateway health and my actual storage usage, so that I can see the true system state at a glance.

#### Acceptance Criteria

1. WHEN the `DesktopWidgets` component mounts, THE `DesktopWidgets` SHALL fetch `/healthz` from the Gateway and display the real gateway status.
2. WHILE `DesktopWidgets` is rendered, THE `DesktopWidgets` SHALL re-fetch `/healthz` every 5 seconds and update the gateway status indicator.
3. WHEN the gateway responds with `{"status":"ok"}`, THE `DesktopWidgets` SHALL display the gateway status as online.
4. IF the `/healthz` fetch fails or returns a non-ok status, THEN THE `DesktopWidgets` SHALL display the gateway status as offline.
5. WHEN `DesktopWidgets` mounts and a valid auth token is present, THE `DesktopWidgets` SHALL fetch `/auth/profile` via `apiFetch` and display `storageUsed` and `storageQuota` from the response.
6. IF no auth token is present or the `/auth/profile` fetch fails, THEN THE `DesktopWidgets` SHALL display storage as unavailable rather than showing hardcoded values.
7. THE `DesktopWidgets` SHALL cancel all polling intervals when the component unmounts.
