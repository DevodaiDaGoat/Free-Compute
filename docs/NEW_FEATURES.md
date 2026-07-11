# New Feature Proposals

Product features extending FreeCompute beyond the current roadmap. Ordered by user impact.

---

## 1. Collaborative Desktop Sessions

**Problem:** Only one user can control a session at a time. No built-in collaboration.

**Feature:** Multi-pointer, multi-cursor remote collaboration with role-based access.

### Design

```
Session Owner (Alice)
    │
    ├── Full control: keyboard, mouse, clipboard, file transfer
    ├── Can invite collaborators via temporary link
    └── Sees all collaborator cursors (color-coded)
            │
            ▼
Collaborators (Bob, Charlie)
    ├── Role: "viewer" | "editor" | "co-owner"
    ├── Viewer: see-only, no input
    ├── Editor: cursor + keyboard, no destructive actions
    ├── Co-owner: full control (except removing owner)
    └── Each has independent input stream multiplexed on same session
```

### Technical Changes

- **Multi-stream input:** `InputManager` accepts multiple `SessionInput` maps (one per peer).
- **Cursor overlay:** WebRTC data channel delivers cursor positions for overlay rendering.
- **Permissions:** `SessionPermissions` extended with `inviteUsers`, `maxCollaborators`, `roleAssignment`.
- **Audio mixing:** Gateway mixes audio from host for all participants; individual audio channels are forwarded independently.
- **Handoff:** Owner can transfer ownership; new owner gets full control.

### Implementation Phases

| Phase | Scope |
|-------|-------|
| 1 | Viewer-only: audio/video sharing, no input (screen-share equivalent) |
| 2 | Editor role: cursor + keyboard for one collaborator at a time |
| 3 | Multi-cursor: independent inputs, all cursors visible |
| 4 | Co-owner: full role system with transfer |

### Use Cases

- Pair programming on cloud desktop
- Collaborative 3D modeling / design review
- Remote education: instructor can take over student session
- Customer support: agent views customer desktop

---

## 2. Cloud Save & Session Persistence

**Problem:** VM state is ephemeral. Rebooting loses all unsaved work. No persistent home directories across sessions.

**Feature:** Persistent user profile tied to the account, mounted into every session.

### Architecture

```
User Profile Volume (S3 / EBS / NFS)
    │
    ├── /home/user/           → persistent home directory
    ├── /opt/app-data/        → application configs, browser profiles
    ├── /var/lib/games/       → game save data, cloud saves
    └── /var/lib/ssh-keys/    → persisted SSH host keys (no fingerprint warnings)
            │
            ▼
Mounted into every VM/container at session start via:
    ├── NFS export (low-latency LAN between gateway and host)
    ├── S3FS/CephFS FUSE mount (internet-accessible)
    └── Rsync-on-start (for hosts without shared storage)
```

### Sync Strategy

| Mode | Latency | Consistency | Best for |
|------|---------|-------------|----------|
| **NFS** | ~1ms | Close-to-open | Production deployments with shared storage |
| **S3FS** | ~10ms | Eventual | Community hosts without shared FS |
| **Rsync** | Depends | Snapshot | Lightweight, fire-and-forget |
| **Live-grpc** | ~5ms | Strong | Future: live sync via gateway proxy |

### User-facing Features

- **Cloud Save:** Game progress persists across sessions (like Steam Cloud).
- **Home Directory:** ~20 GB persistent storage per user (configurable).
- **App Settings:** Browser bookmarks, IDE extensions, terminal history survive reboots.
- **Quick Resume:** Session state snapshot on disconnect; resume within 30 seconds.

---

## 3. AI-Assisted Session Management

**Feature:** LLM integration for session diagnostics, quality optimization, and resource allocation.

### Use Cases

#### 3a. Quality Troubleshooter

User reports "stream feels laggy" → AI analyzes last 60 seconds of telemetry:

```
Telemetry ingested:
  ├── RTT: 12ms → 180ms (spike at T+30s)
  ├── Packet loss: 0.1% → 4.2% (spike at T+30s)
  ├── Bitrate: 25 Mbps → 3 Mbps (throttled at T+31s)
  └── Resolution: 1440p → 720p (downscale at T+33s)
        │
        ▼
AI diagnosis:
  "Network degradation detected at 14:32:15.
   Possible causes:
   1. Local WiFi congestion (2.4 GHz interference)
   2. ISP throttling
   Recommended action: Switch to Fast Mode with H.264 and 720p@60."
```

#### 3b. Adaptive Scheduler Tuning

AI observes scheduling patterns and adjusts weights:

- If GPU-heavy sessions consistently wait longer, increase GPU weight in scoring.
- If a region has consistent 30ms p95 latency, reduce its scheduling priority.
- If user always selects "Gaming, Fast, H.265," pre-allocate encoder slots.

#### 3c. Anomaly Detection

AI detects crypto mining, abnormal bandwidth usage, or policy violations from session metrics.

---

## 4. WebOS App Ecosystem & Store

**Feature:** An app store for the WebOS desktop environment, with installable cloud-native apps.

### Store Architecture

```
WebOS Store (apps.freecompute.io)
    │
    ├── App manifest (JSON):
    │   ├── id, name, version, icon, description
    │   ├── permissions (clipboard, filesystem, network, audio)
    │   └── entrypoint (URL or WebOS package)
    │
    ├── Categories: Development, Creativity, Games, Utilities, Social
    └── One-click install → pinned to WebOS taskbar
```

### Built-in WebOS Apps (to build)

| App | Purpose | Priority |
|-----|---------|----------|
| **Code Editor** | Monaco/VSCode-based in-browser IDE | High |
| **File Manager** | Dual-pane, S3/cloud-storage aware | High |
| **Terminal** | SSH + local shell in browser (exists, extend) | Medium |
| **Media Player** | Video/audio player with codec switching | Medium |
| **Clipboard Manager** | History, sync across sessions | Medium |
| **Network Monitor** | Real-time bandwidth, latency, packet loss | Low |
| **Resource Monitor** | CPU/RAM/GPU/disk usage in session | Low |

---

## 5. Real-Time Collaboration API

**Feature:** Expose the tunnel infrastructure as a public API for embedding remote desktops in third-party apps.

### API Surface

```
POST /api/v1/sessions             → Create session (returns embed token)
GET  /api/v1/sessions/{id}/embed  → Returns <iframe> snippet or JS SDK URL
POST /api/v1/sessions/{id}/input  → Inject keyboard/mouse events (bot API)
WebSocket /api/v1/sessions/{id}/events → Real-time session event stream
```

### Use Cases

- Embed a cloud desktop in a learning management system (LMS).
- Add remote desktop to a customer support ticketing system.
- Build a CI/CD pipeline that spawns a VM for manual testing.

---

## 6. Connection Quality Dashboard

**Feature:** Real-time and historical connection quality visualization for users and admins.

### User Dashboard

```
Session Health (live):
  ┌────────────────────────────────────────────┐
  │  Latency:  12ms ┤█▌                        │
  │  Jitter:    2ms ┤█                          │
  │  Loss:     0.1% ┤                           │
  │  Bitrate:  28M ┤███████████▌               │
  │  Codec:  H.264 High 4:2:0 @ 1440p@60       │
  │  Connection: WebRTC (UDP, direct P2P)      │
  └────────────────────────────────────────────┘
  30-min history: [latency chart] [bitrate chart] [events]
```

### Admin Dashboard

- Global heatmap: average connection quality by region.
- Host ranking: top/bottom hosts by connection score.
- Session list: filter by quality score < 50.
- Trend analysis: codec adoption, average bitrate over time.

---

## 7. Audio Enhancements

**Current state:** `audio.go` has stub AEC/NS/AGC and custom `sqrt`.

### Feature: Real-Time Audio Pipeline

| Stage | Library | Purpose |
|-------|---------|---------|
| AEC | `speexdsp` or `webrtc-apm` | Echo cancellation for 2-way audio |
| NS | `rnnoise` (RNN-based) | Neural noise suppression |
| AGC | Manual gain control | Volume normalization |
| Mixer | Gateway-side mixer | Multi-user collaboration audio mix |
| Opus | `github.com/gammazero/opus` | Full Opus encode/decode |

### Audio Forwarding Modes

| Mode | Latency | Quality | Use case |
|------|---------|---------|----------|
| Voice chat | ~40ms | 16kHz mono, Opus 24kbps | Collaboration, support |
| Music | ~100ms | 48kHz stereo, Opus 128kbps | Media playback |
| Passthrough | ~20ms | Raw PCM | Low-latency recording |

---

## 8. Headless Mode & API-Only Deployment

**Feature:** Run FreeCompute without the WebOS frontend — pure API gateway for programmatic access.

### Config toggle

```
FREECOMPUTE_HEADLESS=true
FREECOMPUTE_API_ONLY=true
```

Disables:
- WebOS BootSequence → redirect to console
- WebOS desktop UI → 404
- Streaming dashboard → 404

Enables:
- Full REST + WebSocket API on `:8080`
- Session CRUD, input injection, file transfer via API
- Webhook notifications on session events (start, end, quality degredation)

### Use Cases

- CI/CD integration: "Create VM → run tests → destroy VM" via curl.
- Game server hosting: API to start/stop game servers with player matchmaking.
- Enterprise: custom frontend built on FreeCompute API.

---

## Priority Matrix

| Feature | User Impact | Dev Effort | Risk | Do First? |
|---------|-------------|------------|------|-----------|
| Cloud Save & Persistence | High | Medium | Low | ✅ Yes |
| Audio Pipeline (Opus, AEC) | High | Medium | Low | ✅ Yes |
| Quality Dashboard | Medium | Low | Low | ✅ Yes |
| Collaborative Sessions | High | High | Medium | Phase 2 |
| App Ecosystem | Medium | High | Low | Phase 2 |
| AI Session Management | Medium | High | Medium | Phase 3 |
| Public API / Embed | High | Medium | Medium | Phase 3 |
| Headless Mode | Low | Low | Low | Phase 3 |

---

## 9. Native Desktop App (Electron/Tauri)

**Feature:** A native desktop wrapper for FreeCompute with system-level integration beyond what a browser tab can offer.

### Benefits Over Browser

| Capability | Browser | Desktop App |
|-----------|---------|-------------|
| Fullscreen | ✅ (F11) | ✅ (dedicated window, no chrome) |
| System tray | ❌ | ✅ — start/stop sessions from tray icon |
| Global hotkeys | ❌ | ✅ — Ctrl+Alt+F to toggle fullscreen from any app |
| Auto-start on login | ❌ | ✅ — session resume on boot |
| Native notifications | ✅ (limited) | ✅ — OS-native toast notifications |
| File drag-and-drop | ⚠️ (browser APIs) | ✅ — native OS file picker + drag |
| Clipboard sync | ❌ | ✅ — bidirectional clipboard between local and remote |
| Multi-monitor | ⚠️ (fullscreen only) | ✅ — span across monitors |
| Gamepad API | ✅ | ✅ (raw HID access for exotic controllers) |
| Offline session list | ❌ | ✅ — browse recent sessions without internet |
| Bandwidth monitor | ❌ (in-tab only) | ✅ — taskbar/menubar bandwidth widget |

### Technology Choices

| Option | Pros | Cons |
|--------|------|------|
| **Electron** | Mature, large ecosystem, Chromium-based | Heavy (~150MB binary), higher memory |
| **Tauri** | Lightweight (~5MB binary), lower memory, Rust backend | Younger ecosystem, fewer plugins |
| **Wails** | Go frontend, native widgets | Smallest ecosystem |

### Recommended Path

Start with **Tauri**. The gateway is already in Go, and Tauri's Rust backend can embed the Go WebRTC WASM client natively. On macOS/Windows, Tauri gives a native-feeling window without Chromium overhead.

```
desktop/
  src-tauri/    # Rust backend — system tray, hotkeys, native APIs
  src/          # Svelte/React frontend — same as web frontend
  Cargo.toml
```

### Feature: Session Quick Resume

Desktop app persists the last session state locally. On launch:

1. Check if user was in an active session
2. Show "Resume Last Session" button on splash screen
3. One-click reconnects with same settings, same host

---

## 10. Mobile App (iOS / Android)

**Feature:** Native mobile client for on-the-go access to FreeCompute sessions. View and control your desktop from a phone or tablet.

### Key Design Decisions

- **iOS:** Safari WebRTC + PWA wrapper initially; native SwiftUI for phase 2
- **Android:** Native Kotlin with WebRTC SDK; Chrome Custom Tab for WebOS
- **Control mode:** Touch-to-trackpad conversion with gestures (two-finger scroll, pinch zoom, long-press right-click)
- **Viewing mode:** Read-only stream with pinch-to-zoom, taps don't send input

### Touch Input Mapping

| Gesture | Action |
|---------|--------|
| Single finger drag | Mouse move |
| Single finger tap | Left click |
| Two finger tap | Right click |
| Two finger scroll | Scroll wheel |
| Pinch | Zoom (viewport) |
| Three finger swipe | Switch between open apps |
| Long press + drag | Window drag |

### Mobile-Specific Features

- **Push notifications** when session is about to timeout / resource low
- **Quick connect** from lock screen widget (Android) / control center (iOS)
- **Bandwidth saver** — cap resolution to 720p on cellular, 1080p on WiFi
- **Orientation lock** — force landscape for desktop sessions
- **External keyboard/mouse** — support for iPad Magic Keyboard, Bluetooth peripherals

### Platform Deployment

| Store | Phase | Notes |
|-------|-------|-------|
| PWA (both) | 1 | Installable, 80% features, no app store review |
| TestFlight (iOS) | 2 | Beta distribution |
| Google Play (Android) | 2 | Open beta |
| App Store (iOS) | 3 | Full native release |

---

## 11. Team Workspaces

**Feature:** Organizations can create workspaces with shared resources, pooled billing, and role-based access control.

### Workspace Model

```
Workspace "Acme Corp"
    |
    |-- Members: alice@, bob@, charlie@ (invite-only)
    |-- Roles: Owner, Admin, Member, Viewer
    |-- Resource Pool: 5 GPU hosts, 1 TB storage
    |
    |-- Teams (optional grouping):
    |   |-- "Design" -> alice, bob
    |   |-- "Dev" -> charlie
    |
    |-- Shared VM Images:
    |   |-- Ubuntu 24.04 with CUDA pre-installed
    |   |-- Windows 11 with Blender
    |   |-- Custom: Acme-Dev-Environment-v3
    |
    |-- Billing:
        |-- Pooled: workspace pays, no individual billing
        |-- Chargeback: per-member usage tracked for internal billing
```

### API Considerations

```json
POST /api/v1/workspaces
{
  "name": "Acme Corp",
  "plan": "team",
  "max_members": 25,
  "storage_gb": 1000,
  "allowed_regions": ["us-east", "eu-west"]
}
```

### Use Cases

- **Dev teams:** Shared dev environments with consistent tooling
- **Design studios:** GPU workstations shared across designers
- **Education:** Teacher provisions VMs for entire class at once
- **MSPs:** Managed service providers resell compute to end clients

---

## 12. Session Recording & Replay

**Feature:** Record desktop sessions (video + input events) for later playback, audit, or training.

### Architecture

```
Session Stream
    |
    |-- Video: Gateway encodes session video to H.264 at configurable FPS
    |           (5-30 FPS for archiving, lower than real-time stream)
    |
    |-- Audio: Opus audio recorded alongside video
    |
    |-- Input Events: Mouse clicks, keyboard, file transfers logged with timestamps
            |
            v
        Storage (S3 / local)
            |
            v
        Playback API:
            |-- GET /api/v1/sessions/{id}/recording -> MP4 with metadata
            |-- GET /api/v1/sessions/{id}/events -> input event log
            |-- Replay player in WebOS -> scrub through recording
```

### Storage Costs

| Quality | Resolution | FPS | Bitrate | 1 hour size |
|---------|-----------|-----|---------|-------------|
| Low (audit) | 720p | 5 | 200 Kbps | ~90 MB |
| Medium | 1080p | 15 | 1 Mbps | ~450 MB |
| High | 1440p | 30 | 4 Mbps | ~1.8 GB |

### Use Cases

- **Compliance:** Audit what users did in a session (PCI-DSS, HIPAA)
- **Bug reports:** User records session -> shares link -> dev replays to see the bug
- **Education:** Teacher records a lesson; students replay later
- **Support:** Customer support replays session to diagnose issue

### Privacy Considerations

- Users must opt-in to recording (per-session toggle)
- Visual indicator in WebOS when recording is active (red dot)
- Input events are sanitized — no passwords or credit card numbers captured
- Recordings auto-delete after retention period (configurable: 7–365 days)

---

## 13. Session Sharing (One-Click Guest Access)

**Feature:** Share a live session with anyone via link. No account required on the guest side.

### Flow

```
Alice is in a session -> clicks "Share"
    |
    |-- Generates link: https://freecompute.io/s/abc123
    |-- She sets permissions: "View only" | "Let them control"
    |-- Optional: PIN code, expiry (1 hour, 24 hours, never), max viewers
    |
    v
Bob clicks the link -> browser opens WebRTC stream
    |-- No login required
    |-- Sees Alice's desktop in real-time
    |-- If "Let them control" -> Bob's mouse/keyboard forwarded
    |-- WebRTC direct or relay via TURN (Bob's NAT may differ from Alice's)
```

### Technical Requirements

- **Session token** embedded in share URL (HMAC-signed, time-limited)
- **Multi-stream WebRTC** — each viewer gets their own receive-only stream
- **Gateway fan-out** — gateway rebroadcasts the session video to N viewers
- **Input serialization** — if multiple controllers, last-write-wins (or cursor per user)

### Use Cases

- **Live demo:** "Here's what I'm building — watch me code it live"
- **Customer support:** "Let me show you how to fix that setting"
- **Watch parties:** "Check out this game I'm playing"
- **Remote help:** "Can you take over my mouse and fix this?"

---

## 14. User-Defined VM Images & ISO Upload

**Feature:** Users upload their own ISOs or container images to create custom environments, not just the pre-seeded catalog.

### Image Sources

| Source | Latency | Storage | Best For |
|--------|---------|---------|----------|
| **Pre-seeded catalog** | Instant | Gateway cache | Ubuntu, Debian, Windows, common dev envs |
| **ISO upload** | Slow (upload time) | User bucket | Custom OS, niche distros, legacy systems |
| **Dockerfile import** | Medium (build time) | Registry | Reproducible dev environments |
| **Snapshot from session** | Fast | VM storage | "Save my current state as a new image" |
| **Public registry** | Fast (download) | Cache | Community-shared images |

### Image Pipeline

```
User uploads 10GB.iso
    |
    v
File service stores to user's S3 bucket
    |
    v
Host agent downloads ISO to local cache (10GbE)
    |
    v
QEMU boots from ISO (as if inserting a physical disc)
    |
    v
User installs OS in the session (one-time setup)
    |
    v
User clicks "Create Image from Current State"
    |
    v
VM snapshot saved as reusable image in user's library
```

### Limitations

- Upload speed depends on user's internet (a 10 GB ISO takes ~15 min on 100 Mbps)
- ISO uploads are charged to user's storage quota
- Images are private by default; can be shared to workspace or made public

---

## 15. Multi-Region & Edge Deployments

**Feature:** Gateway and hosts deployed across multiple geographic regions. Users automatically routed to the nearest healthy region.

### Architecture

```
User in Tokyo
    |
    |-- DNS resolves to nearest gateway (via latency-based routing)
    |   |-- us-east.gateway.freecompute.io (300ms — no)
    |   |-- apne1.gateway.freecompute.io (2ms — yes)
    |
    v
    apne1 Gateway accepts session
    |
    |-- Selects host in same region (apne1, latency < 1ms)
    |-- If no hosts available -> spill over to adjacent region (ap-southeast-1)
    |-- If no hosts anywhere -> queue with estimated wait time
```

### Routing Strategies

| Strategy | Latency | Complexity | Best For |
|----------|---------|-----------|----------|
| **GeoDNS** (Route53) | ~50ms resolution | Low | Simple multi-region |
| **Anycast IP** | ~1ms resolution | High (BGP) | Global edge |
| **Gateway redirect** | Session-creation time | Medium | On-prem gateways |
| **Client-side latency probe** | Pre-connection | Medium | Mobile users roaming |

### Region Health

Each gateway reports to a central discovery service:

```json
GET /api/v1/regions
{
  "regions": [
    {
      "id": "us-east-1",
      "healthy": true,
      "p95_latency_ms": 12,
      "available_hosts": 42,
      "queue_depth": 3,
      "avg_queue_time_s": 15
    },
    {
      "id": "eu-west-1",
      "healthy": true,
      "p95_latency_ms": 8,
      "available_hosts": 28,
      "queue_depth": 0,
      "avg_queue_time_s": 0
    }
  ]
}
```

Clients fetch this on login and display expected wait times per region.

---

## 16. Bandwidth Saver & Quality Tiers

**Feature:** Users choose a connection quality profile that matches their network conditions and usage.

### Tiers

| Tier | Max Resolution | Max FPS | Bitrate Cap | Bandwidth/Hour | Best For |
|------|---------------|---------|-------------|----------------|----------|
| **Data Saver** | 720p | 30 | 2 Mbps | ~900 MB | Cellular, metered connections |
| **Balanced** | 1080p | 60 | 8 Mbps | ~3.6 GB | Default home WiFi |
| **High Quality** | 1440p | 60 | 25 Mbps | ~11 GB | Gaming, design, LAN |
| **Ultra** | 4K | 120 | 50 Mbps | ~22 GB | Low-latency LAN, high-end gaming |

### Auto Switch

```typescript
const connectionProfile = {
  wifi: 'High Quality',
  cellular: 'Data Saver',
  ethernet: 'Ultra',
  hotspot: 'Data Saver',
};
```

Automatically switch tiers based on active network interface. User gets a notification toast when switching.

### Data Caps

Track bandwidth usage per session. If user is on a metered plan, show a live bandwidth counter and warn at 75%, 90%, 100% of cap.

---

## 17. Game Controller Remapping & Profiles

**Feature:** Remap controller buttons on-the-fly and save profiles per game.

### UI

```
Controller: Xbox Wireless (connected)
    |
    |-- Profile: "Default"
    |-- Profile: "FPS"  (<- active)
    |   |-- A -> Jump
    |   |-- B -> Crouch
    |   |-- Right Stick -> Look (sensitivity: 2.5)
    |   |-- LT -> ADS
    |
    |-- Profile: "Racing"
    |   |-- A -> Accelerate
    |   |-- B -> Brake
    |   |-- Left Stick -> Steer (linearity: 1.2)
    |
    |-- Profile Manager: Import/Export JSON
```

### Implementation

- Controller state captured via Gamepad API in the browser
- Remapping done client-side before sending over WebRTC data channel
- Profiles synced to user's cloud profile (survives across sessions)
- Controller deadzone, sensitivity, and vibration intensity sliders

### Use Cases

- **Cloud gaming:** WASD -> left stick, mouse -> right stick
- **Accessibility:** Remap for mobility-impaired users (one-button actions)
- **Flight sim:** Map button combos to keystrokes

---

## 18. SSO / OIDC / Enterprise Auth

**Feature:** Integrate with identity providers so enterprises can use their existing auth system.

### Supported Providers

| Provider | Protocol | Phase |
|----------|----------|-------|
| Google Workspace | OIDC | 1 |
| Microsoft Entra ID | OIDC + SAML | 1 |
| GitHub / GitLab | OAuth 2.0 | 1 |
| Okta | OIDC + SAML | 2 |
| OneLogin | SAML | 2 |
| LDAP / Active Directory | LDAP bind | 2 |
| Any OIDC provider | OIDC Discovery URL | 1 |

### Config

```
FREECOMPUTE_OIDC_ISSUER=https://accounts.google.com
FREECOMPUTE_OIDC_CLIENT_ID=xxx.apps.googleusercontent.com
FREECOMPUTE_OIDC_CLIENT_SECRET=xxx
FREECOMPUTE_OIDC_SCOPES=openid,profile,email
```

### Benefits

- No password to manage — auth delegated to IdP
- SCIM provisioning for automatic user onboarding/offboarding
- Enterprise SSO is a hard requirement for most B2B deals

---

## 19. Clipboard Sync Across Devices

**Feature:** Seamless clipboard sharing between local machine and remote session, and across user's sessions.

### Modes

| Mode | Direction | Latency | Security |
|------|-----------|---------|----------|
| **Local -> Remote** | Copy on local, paste in session | ~100ms | Text only, max 1 MB |
| **Remote -> Local** | Copy in session, paste on local | ~100ms | Text only, max 1 MB |
| **Bidirectional** | Both directions | ~200ms | Full sync |
| **Clipboard History** | Last 50 clipboard items | Instant | Local storage only |

### Security

- Only plain text in phase 1 (no images/files — security risk)
- User must explicitly enable clipboard sync (off by default)
- Auto-clear clipboard from gateway memory after 30 seconds
- Enterprise: clipboard sync can be disabled entirely via policy

---

## Priority Matrix

| Feature | User Impact | Dev Effort | Risk | Do First? |
|---------|-------------|------------|------|-----------|
| Cloud Save & Persistence | High | Medium | Low | ✅ Yes |
| Audio Pipeline (Opus, AEC) | High | Medium | Low | ✅ Yes |
| Quality Dashboard | Medium | Low | Low | ✅ Yes |
| Session Sharing | High | Low | Low | ✅ Yes |
| Bandwidth Saver Tiers | High (mobile) | Low | Low | Phase 2 |
| Collaborative Sessions | High | High | Medium | Phase 2 |
| App Ecosystem | Medium | High | Low | Phase 2 |
| Custom VM Images / ISO | High (power users) | Medium | Medium | Phase 2 |
| Game Controller Remapping | Medium (gamers) | Low | Low | Phase 2 |
| Clipboard Sync | Medium | Low | Low | Phase 2 |
| Session Recording | High | Medium | Medium | Phase 2 |
| AI Session Management | Medium | High | Medium | Phase 3 |
| Public API / Embed | High | Medium | Medium | Phase 3 |
| Headless Mode | Low | Low | Low | Phase 3 |
| Desktop App (Tauri) | High | High | Medium | Phase 3 |
| Mobile App | High | High | High | Phase 3 |
| Team Workspaces | High (enterprise) | High | Medium | Phase 3 |
| SSO / OIDC | High (enterprise) | Medium | Low | Phase 3 |
| Multi-Region Deployments | High | High | High | Phase 4 |

---

## 20. Billing & Credit System

**Feature:** Usage-based billing with a free tier, prepaid credits, and subscription plans.

### Pricing Model

| Tier | Monthly Price | Compute Hours | Storage | GPU Access | Max Sessions |
|------|--------------|---------------|---------|------------|-------------|
| **Free** | $0 | 10 hours/mo | 5 GB | CPU only | 1 |
| **Starter** | $9.99 | 50 hours/mo | 50 GB | Shared GPU (2h limit) | 2 |
| **Pro** | $29.99 | 200 hours/mo | 200 GB | Dedicated GPU | 4 |
| **Team** | $99.99 | 1000 hours/mo (shared) | 1 TB | Dedicated GPU | 10 |
| **Enterprise** | Custom | Unlimited | Custom | Dedicated GPU + pinning | Unlimited |

### Credit System

```
User buys $50 in credits
    |
    v
Credits applied to account (1 credit = 1 compute minute)
    |
    v
Per-session billing:
    Session starts -> credits held (estimated duration)
    Session ends -> actual credits deducted
    Remaining credits returned
    |
    v
If credits exhausted -> session gets 5 min grace period then force-stop
```

### Free Tier Constraints

- Session max 1 hour (60 min hard cap)
- Queue priority: Free < Starter < Pro < Enterprise
- No persistent storage (ephemeral only)
- Watermark overlay on stream: "FreeCompute Free Tier" (subtle, corner)
- No custom VM images — pre-seeded catalog only

### Subscription Management

```json
POST /api/v1/billing/subscribe
{
  "plan_id": "pro",
  "payment_method_id": "pm_xxx",
  "auto_renew": true
}

GET /api/v1/billing/usage?period=current
{
  "compute_minutes_used": 4820,
  "storage_gb_used": 42,
  "plan_max": 12000,
  "percent_used": 40.2,
  "estimated_bill": 29.99
}
```

### Invoicing

- Auto-generated invoice on subscription renewals
- Manual top-ups generate receipt immediately
- Downloadable PDF invoices with company details
- VAT/GST handling per region (Stripe Tax or equivalent)

---

## 21. Community Host Program

**Feature:** Users donate their spare compute capacity to the network and earn credits or revenue.

### How It Works

```
Home User (Bob)
    |
    |-- Installs host-agent on his gaming PC (linux/windows)
    |   |-- Downloads > double-click > registers with his account
    |
    |-- Sets resource limits:
    |   |-- Max 4 vCPUs (leaves 4 for his own use)
    |   |-- Max 16 GB RAM (leaves 16 GB for his own use)
    |   |-- Only when idle (detects keyboard/mouse activity)
    |   |-- Schedule: 2 AM - 8 AM (while he sleeps)
    |
    |-- Gateway detects host, adds to pool
    |-- Sessions scheduled on Bob's machine
    |-- Bob earns credits per compute-hour
    |
    v
Bob redeems credits for:
    - Premium plan upgrade
    - More storage
    - Cash out (PayPal / crypto, threshold $50)
```

### Host Requirements

| Requirement | Minimum | Recommended |
|------------|---------|-------------|
| CPU | 4 cores | 8+ cores |
| RAM | 16 GB | 32+ GB |
| Storage | 10 GB free | 500 GB+ SSD |
| GPU | None (optional) | NVIDIA GTX 1060+ / AMD equivalent |
| Network | 50 Mbps | 500 Mbps+ |
| Uptime | 50%+ | 90%+ |
| OS | Linux | Ubuntu 24.04 / Windows 11 |

### Isolation & Security

- VMs run with `-accel kvm` but NO PCI passthrough — host GPU not exposed to guest
- Each VM is firewalled to the gateway only (no direct internet, no LAN access)
- Guest traffic is tunneled through gateway (host cannot see decrypted content)
- "Kill switch" — host owner can instantly terminate all VMs from their dashboard
- Monthly host certification (automated benchmark to verify performance claims)

### Rating System

```
Host "Bob's Gaming PC"
    |
    |-- Performance score: 87/100
    |   |-- CPU: AMD Ryzen 5900X (single-core: 92, multi-core: 88)
    |   |-- RAM: 32 GB DDR4-3600 (score: 95)
    |   |-- Storage: 1 TB NVMe (score: 90)
    |   |-- Network: 800 Mbps (score: 85)
    |
    |-- Reliability: 99.2% uptime over 30 days
    |-- User rating: 4.5 stars (12 sessions)
    |-- Compensation: 1.2 credits per compute-hour (adjusted for score)
```

### Use Cases

- **Crypto miners** switch to useful compute during bear markets
- **Gamers** earn while they sleep (idle GPU pays for their own subscription)
- **Cloud bursting** — companies spin up community hosts for batch jobs
- **Edge nodes** — deploy host-agent on Raspberry Pi clusters for ultra-low-latency

---

## 22. Passwordless Auth & WebAuthn / Passkeys

**Feature:** Log in without a password — magic links, passkeys, or hardware security keys.

### Authentication Methods

| Method | Security | UX Friction | Users |
|--------|----------|-------------|-------|
| **Magic link (email)** | Medium (email access == login) | Low — click link | General users |
| **Passkey (biometric)** | High (device-bound keypair) | Low — face/touch | Mobile users |
| **WebAuthn (security key)** | Very high (FIDO2 hardware) | Medium — insert key | Enterprise |
| **TOTP (authenticator app)** | High | Medium — type code | Power users |
| **SMS code** | Low (SIM swap risk) | Low — receive text | Legacy / convenience |
| **Google / GitHub OAuth** | Medium (delegated) | Low — click once | General users |

### Auth Flow (Passkey)

```
User clicks "Sign in with Passkey"
    |
    v
Browser shows OS passkey dialog (Face ID / Touch ID / Windows Hello)
    |
    v
User authenticates (biometric or PIN)
    |
    v
Browser creates assertion using private key (never leaves device)
    |
    v
Gateway verifies assertion against stored public key
    |
    v
Session token issued — user is logged in
```

### Implementation

```go
// Use github.com/go-webauthn/webauthn for WebAuthn RP
webAuthn, _ := webauthn.New(&webauthn.Config{
    RPDisplayName: "FreeCompute",
    RPID:          "freecompute.io",
    RPOrigins:     []string{"https://freecompute.io"},
})

// Registration
options, _ := webAuthn.BeginRegistration(user)
// Return options to browser -> browser calls navigator.credentials.create()

// Login
assertion, _ := webAuthn.BeginLogin(user)
// Return assertion to browser -> browser calls navigator.credentials.get()
```

### Passwordless Migration Strategy

1. Phase 1: Add OAuth (Google, GitHub) alongside passwords
2. Phase 2: Add magic link (email-based)
3. Phase 3: Add passkey support (registration + login)
4. Phase 4: Deprecate passwords — users with passwords can migrate
5. Phase 5: Force passkey for all new accounts

---

## 23. Two-Factor Authentication (2FA)

**Feature:** Optional second factor for account security.

### Supported 2FA Methods

| Method | Phase | Notes |
|--------|-------|-------|
| TOTP (Authenticator App) | 1 | Google Authenticator, Authy, 1Password |
| SMS backup codes | 1 | For phone-based recovery |
| Recovery codes | 1 | 10 one-use codes, print/save |
| WebAuthn (hardware key) | 2 | FIDO2/U2F as second factor |
| Push notification | 3 | "Approve login?" on phone |

### User Flow

```
User enables 2FA in settings
    |
    v
1. Scan QR code in authenticator app
2. Enter 6-digit code to verify setup
3. Save 10 recovery codes (required step)
4. Done — 2FA enabled
    |
    v
On next login:
    after password/OAuth -> prompt for 6-digit TOTP code
    or click "Use recovery code"
```

### Trusted Devices

- Option to "Trust this device for 30 days" (cookie-based)
- Trusted devices skip 2FA prompt
- Revoke all trusted devices from account settings

---

## 24. API Key Management

**Feature:** Users generate personal API tokens for automated access.

### Key Types

| Key Type | Permissions | Expiry | Example Use |
|----------|------------|--------|-------------|
| **Full access** | All APIs | Custom | Personal automation |
| **Session only** | Create/read sessions | Custom | CI/CD pipelines |
| **Read only** | View sessions/stats | Max 1 year | Monitoring dashboards |
| **Webhook** | Can create webhooks only | Custom | External integrations |

### Key Creation Flow

```
User goes to Settings > API Keys > "Create Key"
    |
    v
Name: "CI/CD Pipeline"
Permissions: [Session: create, read]  [Webhooks: create]
Expires: 90 days
    |
    v
Key generated: fc_api_v1_abc123def456... (shown once, never stored in plaintext)
    |
    v
Usage:
    curl -H "Authorization: Bearer fc_api_v1_abc123def456..." \
         https://gateway.freecompute.io/api/v1/sessions
```

### Security

- Keys are stored as bcrypt hash in database (plaintext never persisted)
- One-way hashed key prefix (`fc_api_v1_`) for key lookup (prefix + bcrypt)
- Rate limiting per key (1000 req/min)
- Auto-disable keys idle > 90 days
- Audit log of all API key activity

---

## 25. Scheduled Sessions

**Feature:** Pre-schedule a session to start and stop at defined times.

### Use Cases

- **CI build worker:** Spawn a VM at 9 AM every weekday, run tests, destroy at 5 PM
- **Remote desktop:** "Start my work desktop at 8:30 AM every morning"
- **Scheduled gaming:** "Launch my gaming VM every Friday at 7 PM"
- **Batch processing:** "Process these renders overnight, shut down at 6 AM"

### API

```json
POST /api/v1/schedules
{
  "name": "Daily Work Desktop",
  "image": "ubuntu-24.04-dev",
  "region": "us-east",
  "schedule": {
    "timezone": "America/New_York",
    "start_cron": "30 8 * * 1-5",  // 8:30 AM weekdays
    "stop_cron": "0 17 * * 1-5",   // 5:00 PM weekdays
    "max_duration_minutes": 480     // safety cap: 8 hours
  },
  "resources": {
    "vcpus": 4,
    "ram_gb": 16,
    "gpu": "t4"
  },
  "persistence": {
    "home_dir": true,
    "snapshot_on_stop": true
  }
}
```

### Schedule Types

| Type | Description | Reset |
|------|-------------|-------|
| **Daily** | Same time every day | State resets (or snapshot) |
| **Weekly** | Specific days + times | State resets |
| **Cron** | Full cron expression | Custom |
| **One-shot** | Run at datetime + duration | N/A |

### Recurring Session Management

- Dashboard shows upcoming scheduled sessions
- Skip next occurrence button
- Pause schedule (temporarily disable)
- Email notification 5 min before session starts

---

## 26. Session Templates & Presets

**Feature:** Save session configurations as reusable templates.

### Template Fields

```json
{
  "id": "tpl_dev_ubuntu",
  "name": "Ubuntu Dev Environment",
  "description": "Ubuntu 24.04 with VS Code, Node.js, Docker, Python",
  "image": "ubuntu-24.04-dev",
  "resources": {
    "vcpus": 4,
    "ram_gb": 16,
    "gpu": "none",
    "disk_gb": 100
  },
  "region": "auto",
  "codec": "h264",
  "quality": "balanced",
  "persistence": {
    "enabled": true,
    "home_size_gb": 20
  },
  "tags": ["development", "ubuntu", "web"],
  "is_public": false,
  "created_by": "user_abc",
  "used_count": 42
}
```

### Template Categories

| Category | Example Templates |
|----------|------------------|
| **Development** | VS Code + Node, JetBrains + Java, PyCharm + ML |
| **Gaming** | Windows 11 Gaming, Retro Gaming (Batocera) |
| **Design** | Blender + Photoshop, Figma + Illustrator |
| **Education** | CS Student, Data Science, Linux Admin Lab |
| **Empty** | Blank Ubuntu, Blank Windows (minimal) |

### Sharing

- Private templates (only you)
- Workspace templates (shared in team)
- Community templates (published to gallery)
- One-click "Launch from Template" button

---

## 27. Container-Native Sessions (Docker/Podman)

**Feature:** Run containerized sessions for lightweight, fast-starting dev environments (alternative to full VMs).

### Comparison: VM vs Container

| Aspect | VM (QEMU) | Container (Docker) |
|--------|-----------|-------------------|
| Start time | 15-45 seconds | < 2 seconds |
| Image size | 2-10 GB | 100 MB - 2 GB |
| Isolation | Strong (full kernel) | Medium (kernel shared) |
| GPU access | Passthrough (complex) | `--gpus all` (easy) |
| Kernel | Guest kernel | Host kernel |
| Use case | Gaming, Windows, full desktop | Dev, CLI, CI/CD, web apps |

### Architecture

```
Container Session
    |
    |-- Docker container running on host agent
    |   |-- Runs: SSH server + web terminal + optional VS Code Server
    |   |-- Mounted: user's persistent home volume
    |   |-- Ports: container ports mapped to gateway tunnel
    |
    |-- Gateway exposes:
        |-- SSH: container-session-id.gateway.freecompute.io:22
        |-- HTTP: container-session-id.gateway.freecompute.io (apps)
        |-- WebTerminal: browser-based terminal via WebSocket
```

### Supported Images

```json
[
  {"name": "Node.js 22", "image": "node:22", "ports": [3000]},
  {"name": "Python 3.12 + ML", "image": "nvidia/cuda:12.2-runtime", "ports": [8888]},
  {"name": "Go Dev", "image": "golang:1.22", "ports": [8080]},
  {"name": "Rust Dev", "image": "rust:1.77", "ports": [8080]},
  {"name": "PostgreSQL", "image": "postgres:16", "ports": [5432]},
  {"name": "Custom", "image": "user/image:tag", "ports": [80, 443]}
]
```

### When to Use

| Use Case | VM | Container |
|----------|----|-----------|
| Gaming / Windows | ✅ | ❌ |
| Full desktop experience | ✅ | ❌ |
| Quick dev environment | ❌ | ✅ |
| CI/CD build worker | ❌ | ✅ |
| Testing multiple kernels | ✅ | ❌ |
| Database / service hosting | ❌ | ✅ |

---

## 28. Anonymous / Guest Sessions

**Feature:** Try FreeCompute without creating an account. Users get a limited session immediately.

### Flow

```
User clicks "Try Now" on landing page
    |
    v
Browser requests anonymous session token
    |
    v
Gateway issues temp token (no account, no email):
    - Expires in 15 minutes
    - Rate-limited: 3 anonymous sessions per IP per day
    - Max session duration: 10 minutes
    - Resolution capped at 720p
    - CPU-only (no GPU)
    - Watermark: "Anonymous User"
    |
    v
Session launches immediately
    |
    v
At 8-minute mark: banner shows "Sign up to continue your session"
At 10-minute mark: session ends
User can sign up and resume (if signed up within 5 min)
```

### Conversion Funnel

```
Landing page
    |-- 100% visitors see "Try Now" CTA
    |
    v
Anonymous session started
    |-- ~5% of visitors click Try Now
    |
    v
Session in progress
    |-- User sees value
    |
    v
Sign-up prompt (at 80% of session time)
    |-- ~20% of anonymous users sign up
    |
    v
Full account created
```

### Security

- Anonymous sessions run in a separate, isolated VM pool
- No network access to internal services
- Temp token cannot invite others, create API keys, or access billing
- Guest analytics: page view count, interaction time, browser — no PII

---

## 29. Integrations & Ecosystem

**Feature:** First-party and third-party integrations to embed FreeCompute in existing workflows.

### First-Party Integrations

| Integration | What it does | Effort |
|------------|-------------|--------|
| **VS Code Extension** | "Open in FreeCompute" — right-click a repo, launch a dev VM | 2 weeks |
| **JetBrains Plugin** | Same for IntelliJ, WebStorm, PyCharm | 2 weeks |
| **GitHub Actions** | `uses: freecompute/setup-session@v1` — spawn VM in CI | 1 week |
| **GitLab CI** | `image: freecompute/runner` — CI job runs in FreeCompute VM | 1 week |
| **Slack Bot** | `/freecompute start session` — manage sessions from Slack | 1 week |
| **Discord Bot** | Same for Discord — especially for gaming communities | 1 week |
| **CLI Tool** | `fc start`, `fc stop`, `fc list`, `fc ssh` — Go CLI binary | 2 weeks |
| **curl/bash API** | Shell script wrapper for session management | 0.5 week |

### Third-Party Integrations (API)

```typescript
// Example: Embed a FreeCompute VM in an LMS
// Teacher creates assignments that launch in a pre-configured VM

const session = await freecompute.sessions.create({
  image: 'cs101-lab-3',
  student_id: 'user_xyz',
  duration_minutes: 60,
  webhook_url: 'https://lms.example.edu/gradebook/callback',
  on_complete: 'grade_by_script', // run a grading script inside VM
});
```

### CI/CD Integration Deep Dive

```yaml
# .github/workflows/test.yml
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: freecompute/setup-session@v1
        with:
          image: 'test-runner-v2'
          duration: 30m
          api-token: ${{ secrets.FREECOMPUTE_TOKEN }}

      - name: Run tests inside VM
        run: |
          fc scp ./tests :/home/user/tests    # upload test files
          fc exec "cd /home/user/tests && pytest --junitxml=results.xml"
          fc scp :/home/user/tests/results.xml ./results.xml

      - name: Publish results
        uses: actions/upload-artifact@v4
        with:
          name: test-results
          path: ./results.xml
```

---

## 30. Speed Test & Connection Checker

**Feature:** A diagnostic tool that tests the user's connection to FreeCompute gateways before starting a session.

### What it tests

```json
GET /api/v1/speedtest
{
  "gateway": "us-east-1",
  "results": {
    "download_mbps": 285.3,
    "upload_mbps": 42.1,
    "latency_ms": 14,
    "jitter_ms": 2.3,
    "packet_loss_pct": 0.02,
    "nat_type": "Cone NAT",
    "ipv6_supported": true,
    "webrtc_supported": true,
    "meets_minimum": true
  },
  "recommended_action": null,
  "estimated_quality": "high"
}
```

### How it works

```
1. Browser fetches a 5 MB blob from gateway (download speed)
2. Browser uploads a 1 MB blob to gateway (upload speed)
3. Gateway sends a series of UDP pings (latency, jitter, loss)
4. Browser performs STUN binding to detect NAT type
5. Browser checks: WebRTC support, WebCodecs, WebAssembly, WebTransport
6. Gateway returns: estimated quality level + recommended settings
```

### UI

```
┌─────────────────────────────────────────────────┐
│  Connection Check                               │
│                                                 │
│  ┌─────────────────────────────────────────┐    │
│  │  Download   ───████████████████████░ 285│    │
│  │  Upload     ───██████████░░░░░░░░░  42 │    │
│  │  Latency    ───███░░░░░░░░░░░░░░░░  14 │    │
│  │  Jitter     ───░░░░░░░░░░░░░░░░░░░ 2.3│    │
│  │  Loss       ───░░░░░░░░░░░░░░░░░░░0.02│    │
│  └─────────────────────────────────────────┘    │
│                                                 │
│  ✅ Your connection is good enough for 1440p@60 │
│  🡒 Start Session                               │
└─────────────────────────────────────────────────┘
```

### When to show

- On first visit (auto-run, or "Check My Connection" button)
- Before starting a high-end gaming / 4K session
- When quality drops during a session (re-run diagnostic)
- In settings as a manual test button

---

## Priority Matrix

| Feature | User Impact | Dev Effort | Risk | Do First? |
|---------|-------------|------------|------|-----------|
| Cloud Save & Persistence | High | Medium | Low | ✅ Yes |
| Audio Pipeline (Opus, AEC) | High | Medium | Low | ✅ Yes |
| Quality Dashboard | Medium | Low | Low | ✅ Yes |
| Session Sharing | High | Low | Low | ✅ Yes |
| Anonymous / Guest Sessions | High (acquisition) | Low | Low | ✅ Yes |
| Bandwidth Saver Tiers | High (mobile) | Low | Low | Phase 2 |
| Collaborative Sessions | High | High | Medium | Phase 2 |
| App Ecosystem | Medium | High | Low | Phase 2 |
| Custom VM Images / ISO | High (power users) | Medium | Medium | Phase 2 |
| Game Controller Remapping | Medium (gamers) | Low | Low | Phase 2 |
| Clipboard Sync | Medium | Low | Low | Phase 2 |
| Session Recording | High | Medium | Medium | Phase 2 |
| Speed Test / Connection Check | Medium | Low | Low | Phase 2 |
| Session Templates & Presets | Medium | Low | Low | Phase 2 |
| API Key Management | High (developers) | Low | Low | Phase 2 |
| Integrations (CLI, CI/CD, Ext) | High | Medium | Low | Phase 2 |
| Billing & Credit System | High (monetization) | High | Medium | Phase 3 |
| Container-Native Sessions | High (devs) | Medium | Medium | Phase 3 |
| Scheduled Sessions | Medium | Medium | Low | Phase 3 |
| AI Session Management | Medium | High | Medium | Phase 3 |
| Public API / Embed | High | Medium | Medium | Phase 3 |
| Headless Mode | Low | Low | Low | Phase 3 |
| Passwordless Auth / Passkeys | High | Medium | Low | Phase 3 |
| Desktop App (Tauri) | High | High | Medium | Phase 3 |
| Mobile App | High | High | High | Phase 3 |
| Team Workspaces | High (enterprise) | High | Medium | Phase 3 |
| SSO / OIDC | High (enterprise) | Medium | Low | Phase 3 |
| Two-Factor Authentication | High (security) | Low | Low | Phase 3 |
| Multi-Region Deployments | High | High | High | Phase 4 |
| Community Host Program | High (scale) | High | High | Phase 4 |
