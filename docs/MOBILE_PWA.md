# Mobile PWA Client

## Goal

Ship an installable, app-store-free **Progressive Web App** client for FreeCompute that runs on phones and tablets and reuses the existing universal proxy and WebOS console. The PWA must deliver remote desktop, SSH terminal, file access, and game streaming from the browser without any native code, native SDK, or platform-specific build. All transport goes through the same gateway endpoints the desktop WebOS shell already uses: WebRTC for media, `/ws` for tunneled control channels, and `/signal` for session negotiation. No new client-side protocol is introduced; the gaps are installability wiring, offline shell, and mobile-optimized input/layout.

## Current State

### What exists

- **Frontend framework.** `apps/frontend` is Next.js 15 + React 19 + TypeScript (`apps/frontend/package.json`). The WebOS console lives under `apps/frontend/app/webos/` as a window-manager desktop (boot → login → desktop) with Browser, Terminal, Files, Settings, Connection, Admin, and Calculator apps (`app/webos/page.tsx`, `app/webos/desktop/Desktop.tsx`).
- **Web App Manifest.** `apps/frontend/public/manifest.json` already exists and mirrors the spec in `docs/PWA_SUPPORT.md` (name, short_name, standalone display, theme/background colors, scope, icons). It additionally sets `orientation: "landscape"`, `categories: ["productivity","utilities","remote"]`, and `prefer_related_applications: false`.
- **Service Worker.** `apps/frontend/public/sw.js` already implements a cache-first app-shell strategy. It precaches `/` and `/webos`, maintains a `NEVER_CACHE` list covering `/api`, `/proxy`, `/ws`, `/webrtc`, `/signal`, `/connect`, `/agent`, and `/prewarm`, handles navigation fallback to `/`, and includes `sync` and `push`/`notificationclick` listeners (`apps/frontend/public/sw.js`).
- **SW registration helper.** `apps/frontend/app/sw/register.ts` exports `registerSW()` which registers `/sw.js`, handles `updatefound`, and subscribes to push via `PushManager`. This matches the design in `docs/PWA_SUPPORT.md` and reuses `NEXT_PUBLIC_VAPID_PUBLIC_KEY`.
- **Gateway capabilities advertise browser paths.** `handleCapabilities` in `apps/gateway/internal/tunnel/server.go:473` returns `clientPaths` for `browser`, `webos-app`, `native-client`, `host-agent`, and `edge-worker`. The `browser` block (`server.go:501`) exposes `proxyPath` (`/proxy/{routeID}/{path}`), `websocketTunnelPath` (`/ws/{routeID}`), and `signalingPath` (`/signal/{routeID}/rooms/{roomID}`). The `webos-app` and `native-client` blocks additionally carry `connectPath` (`/connect/{routeID}`) and `rawTcpListener`/`rawUdpListener` flags — **the browser block does not**, which is the correct constraint for a web client.
- **Touch and gamepad input already supported server-side.** `apps/gateway/internal/input/input.go` defines `InputEvent{Type, Timestamp, Data}` and dispatches `input.touch.start/move/end/cancel` → `TouchEvent{Touches[{id,x,y,pressure}]}`, `input.gamepad` → `GamepadEvent{gamepadId, vendor ("xbox"|"playstation"|"generic"), axes[], buttons[]}`, plus `input.keyboard.press/release`, `input.mouse.*`, and `input.scroll`. Device kinds include `touch`, `xbox-controller`, `playstation-controller`, and `generic-gamepad` (`input.go:26`, `:43`, `:73`, `:84`, `:207`, `:239`). The gateway `/input/` handler (`server.go:232`, `handleInputEvent` at `server.go:685`) accepts these events per session.
- **Gateway endpoints the PWA reuses.** Reverse proxy `/proxy/` (`server.go:224`), WebSocket tunnel `/ws/` (`server.go:227`, `handleWebSocketTunnel`), SSH-over-WebSocket `/ssh/` (`server.go:226`, `handleSSHTunnel` at `server.go:1173`), signaling `/signal/` (`server.go:229`), WebRTC session create `/webrtc/` (`server.go:230`, `server.go:554`), sessions `/sessions/` (`server.go:231`), input `/input/` (`server.go:232`), audio `/audio/` (`server.go:233`), transfer `/transfer/` (`server.go:234`), clipboard `/clipboard/` (`server.go:235`), and gaming `/gaming/` (`server.go:236`).
- **Reference docs.** `docs/PWA_SUPPORT.md` already specifies the manifest, service-worker caching strategy, registration, background sync, and push design. `docs/REMOTE_ACCESS_STREAMING.md` already establishes the universal-proxy model, the rule that browsers cannot open raw TCP/UDP and must use `/ws`, `/signal`, or WebRTC, and the touch/gamepad input requirements for mobile and gaming modes.

### Gaps

1. **Service worker is never registered.** `registerSW()` is defined in `register.ts` but is never imported or called; `app/layout.tsx` declares the manifest and `appleWebApp` meta but does not invoke it. In the running app the SW never installs, so installability and offline behavior are inert.
2. **Missing PNG/maskable icons.** `manifest.json` and `sw.js` reference `/icons/icon-192.png` and `/icons/icon-512.png` (and a maskable entry), but `public/icons/` only contains `icon-192.svg` and `icon-512.svg`. Install prompts (especially on iOS and Android) require raster PNGs at 192 and 512, including a `purpose: "maskable"` entry.
3. **Landscape-only orientation.** `orientation: "landscape"` blocks natural portrait phone use; the PWA should permit `any` or `portrait-primary` and adapt layout to orientation.
4. **No offline shell fallback page.** The SW falls back failed navigations to `/` but there is no dedicated offline page; a cached lightweight shell (dashboard + "offline" indicator + last-known gateway status from IndexedDB) should be served when the network and `/capabilities` are unreachable. See `docs/PWA_SUPPORT.md` for the intended offline strategy.
5. **Desktop-oriented WebOS, no mobile layout.** `Desktop.tsx` uses absolute-positioned windows with fixed sizes (e.g. 800×500) and a desktop taskbar. There are no responsive breakpoints, no touch gesture handling for window drag/resize/minimize, and no phone/tablet navigation model.
6. **No on-screen keyboard.** Nothing in the frontend produces `input.keyboard.press/release` KeyboardEvents for a soft keyboard, which is required for SSH terminal and desktop text entry on touch devices.
7. **No virtual gamepad.** No UI emits `input.gamepad` GamepadEvents with Xbox/PlayStation button mapping for gaming mode.
8. **No touch-to-pointer mapping.** There is no layer translating touch/drag gestures into `input.mouse.move/down/up` and `input.touch.*` events for remote desktop control.
9. **No per-session `/ws` tunnel UI.** The WebOS apps do not yet open `websocketTunnelPath` (`/ws/{routeID}`) connections (e.g. for the SSH terminal or a desktop control channel); `app/websow/system/api/websocket.ts` currently only prewarms and creates WebRTC sessions.
10. **Push not end-to-end.** The SW handles `push`/`notificationclick`, but there is no gateway-to-push-server path that delivers session invites/auth requests, and no client Settings UI to manage the subscription (both described in `docs/PWA_SUPPORT.md`).

## PWA Shell

The shell strategy follows `docs/PWA_SUPPORT.md` and must not be re-implemented from scratch; complete the following concrete items:

- **Manifest** (`public/manifest.json`): keep the existing fields; change `orientation` to `any` (or `portrait-primary` with a landscape gaming override via media/display overrides), and add a `purpose: "maskable"` PNG icon. Keep `display: "standalone"`, `scope: "/"`, `start_url: "/"`.
- **Icons** (`public/icons/`): add raster `icon-192.png` and `icon-512.png` plus a maskable variant so install prompts succeed across iOS/Android. Leave the existing SVGs for favicon use.
- **Service Worker** (`public/sw.js`): the existing cache-first shell and `NEVER_CACHE` list are correct. Extend it with (a) an explicit offline document precached alongside `/` and `/webos`, served on navigation failures when `/capabilities` is unreachable, and (b) the `sync` queue already present should persist queued session-creation and input events per `docs/PWA_SUPPORT.md` background-sync design. Do **not** intercept `/ws`, `/signal`, `/webrtc`, `/proxy`, or `/connect` (already excluded).
- **Registration** (`app/sw/register.ts` + `app/layout.tsx`): invoke `registerSW()` from a client component mounted in the root layout (e.g. a `ServiceWorkerProvider` or a `useEffect` in a client boundary) so the SW actually installs in production. Reuse the existing `registerSW()` rather than writing a new one.
- **Push for session invites:** wire the existing `push`/`notificationclick` handlers to a gateway push path that notifies the PWA of session-ready / auth-request / idle-timeout events, and add a Settings toggle to manage the `PushManager` subscription (design in `docs/PWA_SUPPORT.md`). Notification `data.url` should deep-link into the relevant session.

## Mobile Input

All mobile input is expressed as the existing gateway `InputEvent` types from `input.go`; the PWA only needs UI layers that emit those JSON shapes to `POST /input/{sessionId}` (and over the `/ws` tunnel where the session uses WebSocket transport).

- **On-screen keyboard.** A soft keyboard component emits `input.keyboard.press` and `input.keyboard.release` KeyboardEvents (`input.go:63`), carrying `code`, `key`, and modifier flags (`ctrlKey/shiftKey/altKey/metaKey`). It must support the desktop Control/Alt/Super chords used by remote shells and window managers, plus a togglable function-row and a clipboard-paste passthrough via `/clipboard/`.
- **Virtual gamepad.** A touch gamepad (dual analog zones, d-pad, face/shoulder buttons) emits `input.gamepad` GamepadEvents (`input.go:84`) with `vendor` set to `xbox` or `playstation` and a stable `gamepadId`. Map on-screen buttons to the standard Xbox/PS button indices so that `gaming` mode controller mapping on the host side applies correctly; support haptic rumble feedback driven by `/gaming/` `set-rumble` state.
- **Touch-to-mouse.** Single-finger tap/drag translates to `input.mouse.down/move/up` (`input.go:56`, `:49`) with absolute `x/y` and deltas; two-finger gestures map to `input.scroll` (`input.go:98`) and right-click (two-finger tap → `button: "right"`). Multi-touch gestures specific to the remote app (pinch-zoom in a browser, draw strokes) emit `input.touch.start/move/end/cancel` TouchEvents (`input.go:73`) with `id`, `x`, `y`, `pressure` so the host receives raw touch rather than synthesized mouse.
- **Gesture mapping.** A mapping layer decides, per session mode, whether a gesture becomes mouse, scroll, or raw touch, so Desktop/Development sessions feel like a trackpad while Remote-Support and drawing apps receive native touch.

## Connection Choice on Mobile

Browsers cannot open raw TCP/UDP, so the PWA must use the paths advertised under `clientPaths.browser` in `handleCapabilities` (`server.go:501`), exactly as `docs/REMOTE_ACCESS_STREAMING.md` prescribes:

- **Control / tunneling:** use `websocketTunnelPath` `/ws/{routeID}` for SSH terminal and desktop control, and `/ssh/{routeID}` which upgrades to the WebSocket tunnel (`server.go:1173`). Do **not** use `connectPath` `/connect/{routeID}` — it is only present for `webos-app`/`native-client` (`server.go:507`, `:515`) and is not offered to browser clients.
- **Media:** negotiate sessions via `signalingPath` `/signal/{routeID}/rooms/{roomID}` (`server.go:229`) and stream audio/video over WebRTC (`/webrtc/`, `server.go:230`). The browser `clientPaths` intentionally excludes `rawTcpListener`/`rawUdpListener`, confirming WebRTC data channels are the only low-latency media/input path for the PWA.
- **App access:** browser-based hosted apps and previews use `proxyPath` `/proxy/{routeID}/{path}` (`server.go:224`).

Connection discovery should read `/capabilities` at boot (see `app/webos/system/api/websocket.ts:getCapabilities`) and select the `browser` client path set rather than hardcoding endpoints.

## Implementation Steps (files to create/modify)

1. **Wire the service worker.** Invoke `app/sw/register.ts::registerSW()` from a client component mounted in `app/layout.tsx` so the SW installs in production. (No new registration code; just call the existing export.)
2. **Fix install assets.** Add `public/icons/icon-192.png`, `public/icons/icon-512.png`, and a maskable PNG. Update `public/manifest.json` to reference the PNGs, set `orientation` to `any`, and keep the maskable entry.
3. **Offline shell.** Add a lightweight offline document and precache it in `public/sw.js` alongside `/` and `/webos`; serve it on navigation failure when `/capabilities` is unreachable, with a cached "offline" indicator and last-known gateway status from IndexedDB (per `docs/PWA_SUPPORT.md`).
4. **Mobile layout.** Create a responsive phone/tablet navigation model under `app/webos/` (e.g. a `MobileShell` that replaces the desktop `WindowManager`/`Taskbar` on small viewports) reusing existing apps (`Browser`, `Terminal`, `Files`, `Settings`, `Connection`). Add touch gesture handlers for window drag/resize/minimize and safe-area insets.
5. **On-screen keyboard.** Add a soft-keyboard component that emits `input.keyboard.press/release` KeyboardEvents to `/input/{sessionId}` (or the `/ws` tunnel), with modifier chords and clipboard paste.
6. **Virtual gamepad.** Add a touch gamepad component emitting `input.gamepad` GamepadEvents with `vendor` `xbox`/`playstation`, mapped to standard button indices, with rumble feedback from `/gaming/`.
7. **Touch-to-pointer mapping.** Add a gesture-mapping layer that emits `input.mouse.*`, `input.scroll`, and `input.touch.*` events according to the active session mode.
8. **Per-session `/ws` tunnel.** Extend `app/webos/system/api/websocket.ts` with helpers that open `websocketTunnelPath` `/ws/{routeID}` and `signalingPath` for terminal/desktop/gaming sessions, selecting the `browser` client path from `/capabilities`.
9. **Push invites.** Implement the gateway→push path for session-ready/auth-request/idle-timeout events and a Settings toggle for the `PushManager` subscription, reusing the existing `push`/`notificationclick` handlers in `public/sw.js`.

## Acceptance Criteria

- Installs to the home screen from Safari (iOS) and Chrome (Android) and launches in standalone mode.
- Launches and shows the dashboard/offline shell with no network (cached shell + last-known status).
- After online, connects to a remote session using the `browser` `websocketTunnelPath` (`/ws/{routeID}`) and/or WebRTC signaling (`/signal/`) — never `/connect`.
- Touch input (tap, drag, scroll, multi-touch) drives remote desktop via `input.mouse.*` / `input.touch.*`.
- On-screen keyboard produces `input.keyboard.*` events usable in the SSH terminal and desktop.
- Virtual gamepad produces `input.gamepad` events (Xbox/PlayStation mapping) playable in gaming mode.
- Receives a push notification for a session invite and deep-links into that session on tap.

## Risks

- **iOS PWA limitations.** iOS does not support `PushManager` for web apps (no web push until recent Safari versions and only with caveats), background sync is limited, and standalone mode still shows some browser chrome. The PWA must degrade gracefully: in-app polling of `/capabilities`/session state instead of relying on push, and an in-app "session invites" list as a fallback to notifications.
- **WebSocket over mobile networks.** Carrier NAT, aggressive proxy timeouts, and intermittent handoffs between Wi-Fi/cellular can silently drop `/ws` and WebRTC signaling. Mitigate with the existing `/prewarm` keepalive (`websocket.ts:preconnectGateway`), exponential-backoff reconnect, and TURN/relay fallback for WebRTC as described in `docs/REMOTE_ACCESS_STREAMING.md`.
- **Battery and thermal.** Continuous WebRTC media plus a virtual gamepad and touch sampling drains battery and can trigger thermal throttling on phones. Default to the `safe` stream preset on mobile, cap encoder bitrate/FPS, reduce control-channel polling frequency, and let the OS suspend the SW when backgrounded.
- **Input latency.** Touch-to-mouse and software-gamepad translation add round-trip latency versus native clients; prefer WebRTC data channels for input where the session supports them, and batch input events to reduce per-message overhead.
- **Install asset mismatch.** If PNG/maskable icons are missing or orientation is wrong, install prompts may be suppressed; keep `public/manifest.json` and `public/icons/` in sync with what `public/sw.js` and `app/layout.tsx` reference.
