# Session Recording and Temporary Support Links

## 1. Goal & User Stories

Remote Support Mode provides controlled, auditable, temporary access to a user's desktop or VM for the purpose of receiving help from a technician, friend, or support agent. The owner retains consent-gated control, the session is bounded by time, and every action is logged.

| # | User story |
|---|------------|
| 1 | As a device owner, I can generate a temporary link scoped to one session so that a helper can view (and optionally control) my desktop without sharing my account credentials. |
| 2 | As a helper, I can redeem that link to join the session in view-only mode or, if the owner permits, remote-control mode. |
| 3 | As a device owner, I receive an approval prompt when the helper requests control; I can approve or deny, and I can revoke access at any time. |
| 4 | As a device owner, the session can be optionally recorded and stored to my per-user drive, encrypted at rest. |
| 5 | As an auditor, I can inspect a full connection log with actor identity, timestamps, IP address, user agent, and actions (link created, approval granted/denied, input accepted/rejected, session ended, recording saved). |
| 6 | As the system, the session expires automatically after a configurable maximum duration or idle timeout, whichever comes first. |
| 7 | As the system, the helper's client identity is verified against device posture (browser fingerprint, Tailscale node namespace, or host-agent attestation) before the link is honored. |

## 2. Current State

### What exists

| Component | Location | Behavior |
|-----------|----------|----------|
| `SessionManager` | `apps/gateway/internal/session/session.go:14` | In-memory session store with CRUD states. Supports `remote-support` as `SessionType` and `SessionMode`. |
| Approval scaffold | `apps/gateway/internal/session/session.go:80` | `SessionStateWaitingApproval`, `PendingApproval`, `ApproveSession`. Only wired when `req.Type == SessionTypeRemoteSupport && req.Permissions.RequiresUserApproval`. |
| `AuditLogger` | `apps/gateway/internal/session/audit.go:13` | In-memory append-only log with `Log(sessionID, actor, action, metadata)`, `GetSessionAuditLog`, `ExportJSON`. |
| `AuthManager` | `apps/gateway/internal/auth/auth.go:51` | User model with `ID`, `Email`, `Credits`, `StorageUsed`, `StorageQuota` (10 GB). JWT-based access/refresh tokens. No role/claim model beyond `Role: "user"` in DB rows. |
| `TemporaryAccessLink` | `apps/gateway/internal/session/session.go:195` | Struct exists with `ID`, `SessionID`, `CreatedByUserID`, `URL`, `OneTimeUse`, `Permissions`, `ExpiresAt`, `RevokedAt`, `AccessCount`. `CreateTemporaryAccessLink` is implemented, but there is no HTTP endpoint or redemption flow. |
| `SessionRecorder` | `apps/gateway/internal/tunnel/session_recording.go:14` | Start/stop/record-frame/delete. Writes to `os.Create(dir + "/freecompute-recording-" + sessionID + ".raw")` in `os.TempDir()`. No encryption, no per-user drive routing, no quota hygiene. |
| Session recording endpoint | `apps/gateway/internal/tunnel/server.go:296` | `mux.HandleFunc("/sessions/recordings", server.handleListRecordings)`. `handleRecordingOps` at `server.go:1335` is defined but **not registered** on the mux. |
| Config fields | `apps/gateway/internal/tunnel/config.go:80` | `EnableSessionReplay bool` and `RecordingDir string` (env `FREECOMPUTE_ENABLE_SESSION_REPLAY`, `FREECOMPUTE_RECORDING_DIR`). Default `RecordingDir = "/tmp/freecompute-recordings"`. |
| Storage backend | `apps/gateway/internal/storage/storage.go:34` | `StorageManager` with per-user namespacing, quota hooks, and `/storage/*` HTTP handlers. The files map is in-memory only. |
| Input manager | `apps/gateway/internal/input/input.go:194` | `HandleInputEvent` processes events with no approval or session-state gating. |

### Gaps

1. **No temporary-link endpoint or redemption flow.** `CreateTemporaryAccessLink` exists only as a struct method; callers have no REST surface to issue or redeem links.
2. **No approval gate on input.** `handleInputEvent` (`server.go:685`) calls `inputManager.HandleInputEvent` without checking session state, permissions, or approval status.
3. **No scheduler awareness.** `SessionScheduler.calculatePriority` (`scheduler.go:164`) gives remote-support priority 75, but sessions waiting approval are never enriched with `vmID` or routed to a host until after approval.
4. **No recording persistence or encryption.** `SessionRecorder` writes raw files to `/tmp` with no relationship to the authenticated user's drive, no AES key wrapping, and no quota check.
5. **No connection log enrichment.** `AuditEntry` lacks `IPAddress`, `UserAgent`, `ApproverDeviceID`, `InputRejectedReason`, and `RecordingPath`.
6. **No expiration enforcement loop.** `Session.ExpiresAt` is set at creation, but there is no sweeper or idle-timeout watcher that transitions sessions to `SessionStateExpired` and cleans up recordings.
7. **No device verification.** A link can be redeemed from any client with the token; there is no check that the redeemer is using a verified Tailscale node, host-agent, or registered browser fingerprint.
8. **Admin/role model is flat.** `AuthManager.User.Role` is always `"user"`. There is no `admin` or `support` gating on approval actions.

## 3. Temporary Access Links

### Token model

A temporary link is a one-time or multi-use bearer token bound to exactly one session. It carries its own `SessionPermissions` scope, a short TTL, and a redemption constraint.

```go
type TemporaryAccessLink struct {
    ID               string
    SessionID        string
    CreatedByUserID  string
    URL              string            // e.g. /access/<linkID>
    OneTimeUse       bool
    Permissions      SessionPermissions // may be narrower than session permissions
    CreatedAt        time.Time
    ExpiresAt        time.Time
    RevokedAt        *time.Time
    AccessCount      int
    MaxAccessCount   int               // >0 enforces multi-use cap; 0 = unlimited until revoked
    DeviceBinding    *DeviceBinding    // nil = no device verification required
}

type DeviceBinding struct {
    RequireTailscaleIP bool
    AllowedTailscaleIP string
    RequireHostAgent   bool
    AllowedAgentID     string
    BrowserFingerprintHash string   // SHA-256 of navigator.userAgent + platform
}
```

### Scope

- **Session (not account)**: The link is invalid if the session is ended or expired.
- **Permissions subset**: `AllowRemoteControl`, `AllowClipboardRead`, `AllowFileUpload`, `AllowFileDownload`, `AllowSessionRecording` can be enabled or disabled independently of the session's own permissions. The effective permission for an operation is `session.Permissions.X && link.Permissions.X`.
- **TTL**: 15 minutes default (configurable via env). A link can be revoked by the session owner at any time.

### Issuance

New endpoint or extension of `/sessions/`:

```
POST /sessions/{sessionID}/links
Authorization: Bearer <owner-token>
```

Request body:

```json
{
  "oneTimeUse": true,
  "maxAccessCount": 0,
  "ttlSeconds": 900,
  "permissions": {
    "allowRemoteControl": false,
    "allowSessionRecording": false,
    "maxDurationSeconds": 300,
    "idleTimeoutSeconds": 120
  },
  "deviceBinding": {
    "requireTailscaleIP": false,
    "requireHostAgent": false,
    "browserFingerprintHash": ""
  }
}
```

Response:

```json
{
  "linkId": "link_<hex>",
  "url": "/access/link_<hex>",
  "expiresAt": "2025-01-01T00:15:00Z"
}
```

### Redemption

```
POST /access/{linkID}
```

Server resolves the link by `linkID`, rejects if `ExpiresAt < now` or `RevokedAt != nil`. On success, returns an ephemeral connection token scoped to the session and honoring the link's permission subset. If `OneTimeUse` is true, `AccessCount` is incremented and the link is invalidated after the first redeemed request.

## 4. Approval Gate

Remote-control input must be rejected for any session whose permissions require approval until an explicit approval is granted. This is enforced at the WebSocket / HTTP input boundary, not by client honor.

### Flow

1. Helper redeems a link and connects. The session enters `SessionStateConnecting` with `RequiresUserApproval == true` and `ApprovedByUserID == ""`.
2. Video/audio streams are allowed; input events from the helper are accepted by the transport layer but **dropped at the gateway** with an audit event and an HTTP `403`-style error on control channels.
3. Owner receives an approval notification (new `/sessions/{sessionID}/approve-request` poll or WebSocket event).
4. Owner approves via:

```
POST /sessions/{sessionID}/approve
Authorization: Bearer <owner-token>
```

5. Gateway transitions the session to `SessionStateActive`, sets `Permissions.ApprovedByUserID` and `Permissions.ApprovedAt`, and begins forwarding `HandleInputEvent` without filtering.

### Hook point

In `apps/gateway/internal/tunnel/server.go`, `handleInputEvent` at line 685 must consult `session.Permissions.RequiresUserApproval` and `session.Permissions.ApprovedByUserID`:

```go
session, err := s.sessionManager.GetSession(sessionID)
if err != nil { ... }
if session.Permissions.RequiresUserApproval && session.Permissions.ApprovedByUserID == "" {
    http.Error(w, `{"error":"approval required"}`, http.StatusForbidden)
    return
}
```

If input arrives from a caller whose user ID does not match `session.UserID` (the owner) and does not match the approved helper chain, reject and log `input-rejected-requires-approval`.

## 5. Session Recording

### Capture target

Recordings must land in the authenticated owner's per-user drive namespace, not in `/tmp`. The `SessionRecorder` should accept a `storage.StorageManager` (or the planned `Backend`) and write to `<userID>/recordings/<sessionID>.<format>` instead of a temp path.

### Start / stop

Recording lifecycle is mediated by the session state machine:

- `start` is allowed only if `session.Permissions.AllowSessionRecording == true` and the link (if any) also has the flag.
- `stop` is allowed by owner, approved helper, or admin.
- `max recording size`: 2 GB (matches `sessionRecordingMaxSize` in `session_recording.go:12`).

### Encryption at rest

Each recording file is encrypted with a per-recording AES-256-GCM key. The key is wrapped with the owner's public key (or a gateway-level master key stored in `apps/gateway/internal/keys`) and stored in `files/<recordingID>.key`. The existing `keys.Manager` (`apps/gateway/internal/keys`) is the intended key source.

### Quota integration

Before starting a recording, estimate the max final size (`MaxDurationSeconds` × bitrate). Reserve that quota via `storageManager.WriteFile`'s `quotaFn`. If the owner's drive is full, reject the recording start.

## 6. Audit & Expiration

### Event log fields

```go
type AuditEntry struct {
    ID           string
    SessionID    string
    ActorUserID  string
    Action       string                  // created, link-created, approved, denied, input-rejected, input-accepted, ended, recording-started, recording-stopped, expired, link-redeemed, link-revoked
    IPAddress    string
    UserAgent    string
    DeviceID     string                  // Tailscale IP, host-agent ID, or browser fingerprint hash
    RecordingPath string
    Metadata     map[string]interface{}   // originalRequester, linkId, oneTimeUse, bytesWritten, reason
    CreatedAt    time.Time
}
```

Expiration actions to log: `expired` at max duration, `expired` at idle timeout, `ended` at user request, `recording-stopped` on session end or expiration.

### Automatic end

A background sweeper in `SessionManager` (new file `apps/gateway/internal/session/expiry.go`) ticks every 30 seconds:

- If `time.Now().After(session.ExpiresAt)` → transition to `SessionStateExpired`, log `expired`, stop any active recording, delete the link token map, clean up the `pendingApprovals` entry.
- If `session.UpdatedAt` is older than `IdleTimeoutSeconds` and the session is `active` → transition to `SessionStateExpired`, reason `idle-timeout`.

## 7. Implementation Steps

| Step | File | Action |
|------|------|--------|
| 1 | `apps/gateway/internal/session/session.go:519` | Extend `CreateTemporaryAccessLink` to accept `maxAccessCount`, `TTLSeconds`, `deviceBinding`, and persist to a new `temporaryLinks` map keyed by `linkID` for O(1) redemption. |
| 2 | `apps/gateway/internal/session/link.go` | New file. `ResolveLink(linkID)`, `RedeemLink(linkID, deviceInfo)`, `RevokeLink(linkID, byUserID)`. Validate `ExpiresAt`, `RevokedAt`, `OneTimeUse`/`AccessCount`, and `DeviceBinding`. |
| 3 | `apps/gateway/internal/tunnel/server.go` | Add `POST /sessions/{id}/links` and `POST /access/{linkID}` handlers. Wire through `auth.RequireAuth` where appropriate; redemption of a link is authenticated by the link token itself (Bearer or query param). |
| 4 | `apps/gateway/internal/tunnel/server.go:685` | In `handleInputEvent`, read `session.Permissions.RequiresUserApproval` and `ApprovedByUserID`. Return `403` and log `input-rejected-requires-approval` if approval is required but missing. |
| 5 | `apps/gateway/internal/tunnel/server.go:650` | In `handleEndSession`, stop any active recording via `sessionRecorder.StopRecording(sessionID)` and log `recording-stopped-by-end`. |
| 6 | `apps/gateway/internal/session/expiry.go` | New file. `SessionManager.StartExpirySweeper(ctx, interval)` and `SessionManager.checkExpiry()`. Transition to `SessionStateExpired` and trigger cleanup. |
| 7 | `apps/gateway/internal/tunnel/session_recording.go:63` | Change `StartRecording` to accept `userID`, `storageBackend`, and `encryptionKey`. Write to `userID/recordings/...`. Remove the `/tmp` hard-coded path. |
| 8 | `apps/gateway/internal/session/audit.go:14` | Add `IPAddress`, `UserAgent`, `DeviceID`, `RecordingPath` to `AuditEntry`. Update `Log` signature to accept these fields. Add `action` constants for new events. |
| 9 | `apps/gateway/internal/auth/auth.go:30` | Add `Role string` to `User` struct (it exists in DB row but is not marshaled). Expose `RequireRole(role)` middleware for future admin/support flows. |
| 10 | `apps/gateway/internal/storage/backend.go` | Define `Backend` interface as described in `WEBOS_CLOUD_DRIVE.md`. `SessionRecorder` consumes `Backend` instead of raw `os.Create`. |

## 8. Acceptance Criteria

- [ ] A session owner can `POST /sessions/{id}/links` and receive a URL that redeems to a valid connection token.
- [ ] A helper can `POST /access/{linkID}` and receive a connection token scoped to the session and respecting the link's permissions.
- [ ] Input from the helper is rejected with an audit event until the owner `POST /sessions/{id}/approve`.
- [ ] Owner can deny approval, which transitions the session to `failed` and ends it.
- [ ] A `OneTimeUse` link cannot be redeemed twice.
- [ ] A link with `DeviceBinding` rejects a redeem attempt from a mismatched Tailscale IP or missing host-agent attestation.
- [ ] Session recording written to the owner's drive is listed under `/storage/list?userId=<owner>` and accounts for storage quota.
- [ ] Session recording file is encrypted at rest; key is wrapped and stored in `keys.Manager`.
- [ ] A session exceeds `MaxDurationSeconds` → state becomes `expired`, recording stops, link is revoked, audit entry is logged.
- [ ] A session exceeds `IdleTimeoutSeconds` while active → state becomes `expired`, recording stops.
- [ ] Audit log for a session contains entries for: link created, link redeemed, approval granted/denied, input accepted/rejected, recording started/stopped, session ended/expired, with actor, IP, user-agent, and device ID.
- [ ] `EnableSessionReplay = true` and a valid `RecordingDir`/storage backend are present; otherwise recording endpoints return `501`.

## 9. Risks

| Risk | Mitigation |
|------|------------|
| **Consent and privacy** | Approval gate is mandatory for remote-control input; view-only stream can be permitted without approval only if explicitly requested by the owner. All link creation and approval events are audited. |
| **Key management** | Per-recording AES keys are generated via `crypto/rand` and wrapped with the owner's public key or a gateway master key. Master key rotation requires re-wrapping; do not store plaintext keys alongside recordings. |
| **Link leakage** | Short TTL (15 min default), one-time-use option, and device binding reduce replay risk. Links should be delivered via out-of-band channels (email, in-app notification) not logged in plaintext gateway logs. |
| **Storage exhaustion** | Recordings are quota-checked before start. `SessionRecorder` enforces a 2 GB cap per recording. Expired sessions automatically delete their temp artifacts. |
| **Approval bypass** | `handleInputEvent` is the only gate; any future input transport (gaming, clipboard, touch) must also reference `session.Permissions`. |
| **In-memory state loss** | `AuditLogger`, `SessionManager`, `SessionRecorder`, and `temporaryLinks` are all in-memory. A gateway restart drops all active sessions, in-progress recordings, and links. This is acceptable for pre-alpha but must be flagged for production (file-backed journal or database). |
