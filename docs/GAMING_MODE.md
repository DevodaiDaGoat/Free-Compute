# Gaming Mode Design

Treat gaming as a first-class session mode with low-latency streaming, hardware acceleration, controller support, and host-aware scheduling.

---

## 1. Goal & Scenarios

FreeCompute supports four primary session modes, each with distinct scheduling and streaming priorities:

| Mode | Primary use | Default session type | Scheduling priority |
| --- | --- | --- | --- |
| Desktop Mode | General cloud desktop access | Desktop Session | Stability, compatibility, fair cost |
| Development Mode | Browser IDEs, terminals, SSH, app preview ports | Desktop Session | CPU, RAM, storage, persistent networking |
| Gaming Mode | Fullscreen game streaming with controller input | Gaming Session | Lowest latency, GPU, encoder, network quality |
| Remote Support Mode | Temporary access to owned or approved systems | Remote Support Session | Approval, audit logging, timeout controls |

Every session declares:

- Mode: `desktop`, `development`, `gaming`, or `remote-support`.
- Type: `desktop`, `gaming`, `remote-support`, or `host`.
- Resource class: `basic`, `standard`, `gaming`, or `workstation`.
- Stream preset: `safe` or `fast`.
- Supported inputs: keyboard, mouse, touch, Xbox controllers, PlayStation controllers, and generic gamepads.

### Safe Mode vs Fast Mode

Safe Mode prioritizes compatibility:

- WebRTC with H.264 baseline/main or VP8 fallback.
- Conservative bitrate ramp-up.
- Adaptive resolution enabled early.
- Strong packet loss recovery and lower CPU pressure.
- Browser-first controls with keyboard, mouse, touch, clipboard, file transfer, and audio.

Fast Mode prioritizes latency:

- WebRTC UDP preferred, relay only when required.
- H.264 low-latency hardware encoding first, H.265 when available and licensed.
- Higher start bitrate and faster bitrate adaptation.
- Fullscreen capture, high refresh rate targets, and controller polling.
- Gamepad events sent over the lowest-latency control channel available.

---

## 2. Current State & Gaps

### What exists

**Gaming manager endpoints** (`apps/gateway/internal/gaming/gaming.go`, `apps/gateway/internal/tunnel/server.go:849-1001`):

| Endpoint | Method | Function |
| --- | --- | --- |
| `/gaming/{sessionID}` | POST | `CreateGamingSession` — creates a `GamingSession` with mode, target FPS, target latency |
| `/gaming/{sessionID}` | PUT | `UpdateGamingState` — dispatches `register-controller`, `update-controller`, `set-rumble`, `update-metrics`, `optimize-mode` |
| `/gaming/{sessionID}` | GET | `GetGamingSession` — returns current session state |

**Input events** (`apps/gateway/internal/input/input.go:43-96`):

`InputEvent` is a type-tagged envelope (`input.mouse.move`, `input.keyboard.press`, `input.gamepad`, etc.). `GamepadEvent` carries `GamepadID`, `Vendor` (`xbox`, `playstation`, `generic`), `Axes`, and `Buttons`. `InputDevice.Kind` already enumerates `xbox-controller`, `playstation-controller`, `generic-gamepad`.

**WebRTC codec support** (`apps/gateway/internal/webrtc/webrtc.go:45-55`, `encoder.go`, `adaptive_bitrate.go`):

`CodecSupport` toggles H264, H265, AV1, VP8, VP9, Opus, AAC, and `HardwareAccel`. `CreateSessionRequest` accepts `Preset`, `EncodingMode`, `VideoCodecs`, `AudioCodecs`, `Resolution`, `RequestedFPS`, and `LatencyTarget`. `selectVideoCodec` (`webrtc.go:793-855`) already applies different codec selection logic for `fast` vs `safe` presets. `AdaptiveBitrate` (`adaptive_bitrate.go:164-204`) adjusts resolution and jitter buffer from RTT/jitter/loss samples.

**Host capability detection** (`host-agent/cmd/host-agent/main.go:389-413`):

`detectCapabilities` populates `HostCapabilities` with `GPUModel`, `GPUVendor`, `VRAMGB`, `EncoderSupport` (H.264, H.265, AV1 on NVIDIA/AMD; H.264, H.263 otherwise), `CPUCores`, `RAMGB`, `DiskGB`, `NetworkMbps`, and `Region`. The host agent reports this to `/hosts/metrics` every 30 seconds.

### Gaps

| Gap | Impact |
| --- | --- |
| Real controller HID passthrough — currently only in-memory state | Controllers work in browser but do not reach the guest OS |
| Adaptive bitrate loop — `AdaptiveBitrate` is standalone, not wired into session creation or `OptimizeForMode` | Preset changes do not change bitrate/render targets dynamically |
| Encoder selection by host — `CreateSession` does not consult host-reported `EncoderSupport` | Sessions may pick software encoders on GPU-capable hosts |
| Latency budget — no end-to-end latency target enforced across capture, encode, packetize, transit, decode, render | p95 latency drifts unbounded |
| Scheduler hook — `detectCapabilities` data is reported but not consumed by a scheduler for gaming ranking | Host selection is not latency- or GPU-aware |

---

## 3. Controller Support

### Current mapping

`InputEvent` (`input.go:43`) dispatches `input.gamepad` to `handleGamepad`, which unmarshals `GamepadEvent` and logs vendor/axes/buttons. `GamingManager.RegisterController` (`gaming.go:177`) stores a `ControllerState` with pre-allocated 20 buttons and 6 axes. `UpdateControllerState` (`gaming.go:222`) replaces the arrays on each poll.

### Xbox / PlayStation / generic gamepads

The data model already classifies vendors. The missing piece is translation from browser Gamepad API / native HID to the FreeCompute wire format and, on the host side, injection into the guest OS:

1. **Client-side (browser / WebOS / native client)**:
   - Use the Gamepad API (`navigator.GetGamepads()`) or platform HID libraries to poll at the native report rate (250–500 Hz for wired Xbox/PS controllers).
   - Map axis/button indices per vendor using standard layouts:
     - Xbox: triggers as axes 6/7 or buttons 6/7 depending on OS.
     - PlayStation: touchpad as buttons 13/14, gyro as motion data.
     - Generic: fallback to HID usage page 0x01 (gamepad) / 0x02 (joystick).
   - Pack into `GamepadEvent` with `Vendor` set, then POST to `/input/{sessionID}` (`tunnel/server.go:676`).

2. **Gateway-side**:
   - `InputManager.handleGamepad` (`input.go:309`) should validate vendor, deduplicate by `GamepadID`, and forward to the host agent via the existing tunnel or WebRTC data channel (`DCtrlGamepad` in `multichannel.go:13`).
   - Add a `GamepadRumbleEvent` type for `set-rumble` feedback (currently only exists in `GamingManager`).

3. **Host-side**:
   - Host agent receives gamepad frames and opens a virtual HID device (`/dev/uinput` on Linux, `ViGEmBus` on Windows).
   - `GamingManager.SetControllerRumble` (`gaming.go:245`) stores rumble intent; the host agent reads it and writes vibration effects to the virtual device.

### HID mapping approach

Use a vendor-neutral mapping layer in the host agent:

```go
// host-agent/cmd/host-agent/gamepad.go (proposed)
type HIDMapper struct {
    vendor string
    buttonMap map[int]int
    axisMap   map[int]int
}
```

- Xbox → XInput layout.
- PlayStation → SDL_GameControllerDB layout (evdev ABS_HAT0X, BTN_SOUTH, etc.).
- Generic → SDL fallback mapping with usage-page discovery.

---

## 4. Low-Latency Streaming

### Preset knobs

`handleCreateWebRTCSession` (`tunnel/server.go:561-602`) defaults to `safe`, 1920x1080@60 FPS, 50 ms latency target, and `[h264, vp8, vp9]`. `handleCreateGamingSession` (`tunnel/server.go:861-892`) defaults to standard mode, 60 FPS, 20 ms latency target.

Fast Mode should drive the following parameters at session creation and renegotiation:

| Knob | Safe default | Fast target | Controlled by |
| --- | --- | --- | --- |
| Video codec | H.264 baseline / VP8 | H.264 low-latency hardware or H.265 | `selectVideoCodec` (`webrtc.go:793`) |
| Audio codec | Opus / AAC | Opus 20 ms frame | `selectAudioCodec` (`webrtc.go:857`) |
| Start bitrate | 1–2 Mbps | 6–10 Mbps | `AdaptiveBitrate` (`adaptive_bitrate.go`) |
| Min bitrate | 500 Kbps | 2 Mbps | `BitrateConfig.MinBps` |
| Max bitrate | 8 Mbps | 20–50 Mbps | `BitrateConfig.MaxBps` |
| Resolution floor | 854x480 | 1280x720 | `OnQualitySample` resolution drops |
| Max FPS | 30–60 | 60–144 | `BitrateConfig.MaxFPS` |
| Jitter buffer | 20–40 ms | 10–20 ms | `BitrateConfig.JitterBuffer` |
| GOP size | 120 frames (2 s) | 60 frames (1 s) | `EncoderConfig.GopSize` (`encoder.go`) |
| B-frames | 0–2 | 0 | `EncoderConfig.BFrames` |
| Ref frames | 2–3 | 1–2 | `EncoderConfig.RefFrames` |
| Hardware accel | preferred if available | required | `EncoderConfig.HardwareAccel` |
| Latency target | 50 ms | 10–20 ms | `CreateSessionRequest.LatencyTarget` |
| Refresh rate | 60 Hz | 120–144 Hz | `Resolution.RefreshRate` |

### OptimizeForMode feedback loop

`OptimizeForMode` (`gaming.go:335-369`) currently only adjusts `TargetLatency` and `TargetFPS` on the `GamingSession`. It should also call back into the WebRTC session to update encoder and ABR settings:

- `GamingModeCompetitive` → target 10 ms, 144 FPS, GOP=30, zero B-frames, H.264 hardware with ultrafast preset.
- `GamingModeStandard` → target 20 ms, 60 FPS, GOP=60, B-frames=0.
- `GamingModeCasual` → target 50 ms, 30 FPS, GOP=120, B-frames=2.
- `GamingModeVR` → target 5 ms, 90 FPS, GOP=45, B-frames=0, ASW motion vectors.

`UpdatePerformanceMetrics` (`gaming.go:294-320`) is the feedback loop: it records FPS, frame time, bitrate, and latency, then computes a 0–100 performance score. When the score drops below a threshold for N samples, trigger `OptimizeForMode` with a lower-demand mode or signal the ABR controller to reduce bitrate/resolution.

---

## 5. Host Scheduling Hook

### Current host reporting

`runStatusReporter` (`host-agent/cmd/host-agent/main.go:548-578`) POSTs `HostStatus` to `/hosts/metrics` every 30 seconds. `HostCapabilities` includes GPU model, VRAM, encoder support, CPU, RAM, disk, network, and region.

### How to feed session creation / scheduler

1. **Gateway stores host capabilities**:
   `handleHostMetrics` (`tunnel/server.go:1216-1228`) currently only logs the payload. Extend it to persist `HostStatus` in a `HostRegistry` keyed by host ID.

2. **WebRTC session creation consults host capabilities**:
   `CreateSession` (`webrtc.go:258`) should accept an optional `HostID` or `Region`. If provided, look up the host’s `EncoderSupport` and `HardwareAccel` flag, then override `EncodingMode` preset and codec priority:
   - NVIDIA/AMD with H.265 → prefer H.265 hardware (`hevc_nvenc`).
   - Intel iGPU with VAAPI → prefer H.264 hardware.
   - No GPU → fall back to VP8/VP9 software, disable H.265/AV1.

3. **Scheduler ranking for gaming sessions** (`docs/REMOTE_ACCESS_STREAMING.md:85-91`):

   The scheduler ranks gaming sessions by:

   1. User-to-host latency and jitter.
   2. Available hardware encoder and GPU allocation.
   3. Host load and current stream pressure.
   4. Network quality and UDP/P2P reachability.
   5. Requested region and resource class.

   Implementation in `session/scheduler.go` (not yet present in this repo, referenced in `ROADMAP.md:21`):

   ```go
   type GamingHostScore struct {
       HostID         string
       LatencyMs      float64
       EncoderFree    bool
       GPUVendor      string
       GPUUtilization float64
       ActiveStreams  int
       MaxStreams     int
       PacketLoss     float64
       UDPAvailable   bool
       Region         string
       Score          float64
   }
   ```

   Scoring formula:

   ```
   score = (w1 * latencyNorm) +
           (w2 * encoderScore) +
           (w3 * (1 - utilization)) +
           (w4 * networkQuality) +
           (w5 * regionMatch)
   ```

   Where `encoderScore` = 1.0 if hardware encoder is free, 0.5 if software, 0.0 if maxed out.

4. **Pre-warm**:
   Use `GET /prewarm` (`tunnel/server.go:444`) to keep a warm ICE/DTLS state and encoder process ready for the selected host, reducing session startup latency by 200–500 ms.

---

## 6. Implementation Steps

| Step | File | Function | Change |
| --- | --- | --- | --- |
| 1 | `apps/gateway/internal/tunnel/server.go` | `handleCreateWebRTCSession` | Accept `HostID` / `Region`; look up host encoder support; pass hardware accel flag into `webrtcServer.CreateSession` |
| 2 | `apps/gateway/internal/tunnel/server.go` | `handleCreateGamingSession` | Wire `GamingConfig` preset into WebRTC `EncodingMode` and `EncoderConfig`; call `webrtcServer.CreateSession` with fast-safe knobs |
| 3 | `apps/gateway/internal/gaming/gaming.go` | `OptimizeForMode` | After updating `TargetFPS`/`TargetLatency`, call `webrtcServer.UpdateSessionEncoding(sessionID, ...)` to change GOP, B-frames, preset |
| 4 | `apps/gateway/internal/gaming/gaming.go` | `UpdatePerformanceMetrics` | Add adaptive callback: if score < 40 for 3 consecutive samples, downgrade mode or signal ABR |
| 5 | `apps/gateway/internal/webrtc/webrtc.go` | `CreateSession` | Add `HostID` param; query host capabilities; auto-select codec priority and hardware encoder |
| 6 | `apps/gateway/internal/webrtc/adaptive_bitrate.go` | `OnQualitySample` | Add fast-mode branch: higher start bitrate, slower resolution floor, smaller jitter buffer |
| 7 | `apps/gateway/internal/webrtc/encoder.go` | `DefaultEncoderConfig` | Add `GamingPreset` variant with GOP=30, B-frames=0, `EncodingModeSpeed` |
| 8 | `apps/gateway/internal/tunnel/server.go` | `handleHostMetrics` | Persist `HostStatus` into a `HostRegistry`; expose `GET /hosts` with capability filters |
| 9 | `apps/gateway/internal/input/input.go` | `handleGamepad` | Validate vendor, deduplicate by `GamepadID`, forward via `DCtrlGamepad` data channel |
| 10 | `host-agent/cmd/host-agent/main.go` | `runTunnelOnce` / new file `gamepad.go` | Open virtual HID device; map Xbox/PS/generic layouts; inject gamepad frames and rumble effects |
| 11 | `packages/api-types/src/websocket.ts` | `GamepadEvent` | Add `motionData` field for accelerometer/gyroscope |
| 12 | `packages/api-types/src/host.ts` | `HostCapabilities` | Add `ActiveEncoderSessions`, `EncoderUtilization`, `GPUTemperature`, `GPUClockSpeed` |

---

## 7. Acceptance Criteria

- [ ] **Create gaming session**: `POST /gaming/{sessionID}` with `GamingConfig{Mode: "competitive", TargetFPS: 144, TargetLatency: 10}` returns 201 and a session with matching fields.
- [ ] **Register controller**: `PUT /gaming/{sessionID}` with action `register-controller` and vendor `xbox` stores a connected `ControllerState` with 20 buttons and 6 axes.
- [ ] **Send input**: `POST /input/{sessionID}` with `type: "input.gamepad"` and `GamepadEvent{Vendor: "playstation"}` is processed without error and updates `LastActive` on the device.
- [ ] **Request keyframe**: `POST /keyframe/{sessionID}` returns 200 and the WebRTC peer connection receives a `PictureLossIndication` RTCP packet.
- [ ] **Measure p95 latency**: With Fast Mode preset, p95 end-to-end input-to-display latency is under 20 ms on a wired connection to a local host; Safe Mode p95 is under 50 ms.
- [ ] **Fast preset lowers latency vs safe**: `OptimizeForMode("competitive")` reduces target latency from 20 ms to 10 ms and updates GOP/B-frame/ref-frame parameters on the active encoder.
- [ ] **Host scheduling**: A gaming session allocates to a host with free hardware encoder and GPU utilization < 50 % when available, even if a closer host is software-only.

---

## 8. Risks

### H.265 / AV1 licensing

H.265 (HEVC) is covered by patents held by multiple parties (MPEG LA, HEVC Advance, Velos Media). Most browser and OS deployments accept the licensing burden, but software distribution in some jurisdictions requires royalty accounting. AV1 is royalty-free but encoder availability is still maturing. Recommendation: gate H.265/AV1 behind explicit host capability flags (`EncoderSupport`) and legal review per deployment region.

### Encoder availability

Not all hosts have NVIDIA NVENC / AMD VCE / Intel QSV. Software encoding (libx264, libx265) at 1080p/60 FPS can saturate a CPU core and increase latency. The system must degrade gracefully:

1. Detect encoder support at `detectCapabilities` (`host-agent/cmd/host-agent/main.go:405`).
2. If hardware encoder is absent, force `safe` preset, H.264 baseline, 30 FPS, and lower bitrate ceilings.
3. Surface `EncoderSupport` in `HostCapabilities` so the scheduler never assigns a competitive gaming session to a software-only host.

### HID passthrough complexity

Linux `uinput` requires root or `uinput` group membership. Windows `ViGEmBus` is a third-party driver. macOS `IOHIDDevice` requires entitlements. The host agent must handle permission failures gracefully and fall back to software-only controller handling (e.g., map gamepad to keyboard/mouse events via `evdev` or `SendInput`).
