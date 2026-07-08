# Per-User Tailscale IPs with Device-Identity WebOS and Proxy Fallback

Implementation-ready design for assigning a **distinct, persistent Tailscale IP per user**, a **device-identity (no-login) Tailscale IP for the WebOS desktop**, and **admin accounts mapped to their own Tailscale IP**, with a **proxy fallback** when a direct Tailscale path is blocked.

Status: Design / pre-alpha. Grounded in the code that already exists in
`apps/gateway/internal/tunnel/server.go`, `apps/gateway/internal/auth/auth.go`, and
`host-agent/cmd/host-agent/tailscale.go`.

---

## 1. Goal & User Stories

Assign Tailscale IPs at three distinct identity layers:

| Identity layer | Who | Login required? | Requirement |
|---|---|---|---|
| **Per-user** | End users | Yes (saved in account) | A **different** Tailscale IP per user, persisted in their login/account record, returned on every login. |
| **WebOS device** | The WebOS desktop client | No | The WebOS desktop has its **own** Tailscale IP via **device identity** (a client/device ID), usable without a user session. |
| **Admin** | Operators | Yes (admin role) | Admins use their **own account** for Tailscale (same per-user allocation, but with `role=admin`). |
| **Proxy fallback** | All of the above | n/a | When a direct Tailscale connection is impossible (NAT/firewall), reach the IP via a **proxy fallback**. |

### Scenarios

- **Alice (end user).** Alice logs in. The gateway returns IP `100.64.1.7`. She logs out and back in
  the next day; she gets the **same** `100.64.1.7`. Bob gets a **different** IP `100.64.1.19`.
- **WebOS desktop (guest).** A user opens the WebOS desktop but is not logged in. WebOS still obtains a
  working Tailscale IP (`100.64.2.3`) bound to the **device**, not the user, and can reach the desktop
  without authenticating. When the user later logs in, the desktop keeps its device IP and the user
  separately gets their own per-user IP.
- **Admin Carol.** Carol logs in with an admin account; she receives her own Tailscale IP like any user,
  but it is flagged/transported as an admin allocation (could be on a separate CIDR or tagged route).
- **NAT/firewall blocked.** A direct dial to `100.64.1.7` times out. The gateway transparently returns a
  proxy route (`/tailscale/proxy`) so the client still reaches Alice's target via the fallback.

---

## 2. Data Model

### 2.1 Per-user record (persisted in account)

The `auth.User` struct (`apps/gateway/internal/auth/auth.go:30`) already carries Tailscale fields.
Extend/pin them and persist to `database.UserRow` (`apps/gateway/internal/database/database.go:21`):

| Field | Type | Notes |
|---|---|---|
| `User.ID` | string | Already the key; `user_`-prefixed. |
| `User.TailscaleIP` | string | Per-user **stable** Tailscale IP. Already present (`auth.go:39`). |
| `User.TailscaleProxy` | string | Proxy mode: `"direct"`, `"relay"`, `"disabled"`. Already present (`auth.go:40`). Default `relay` on first allocation (`auth.go:263`). |
| `User.TailscaleCIDR` | string | Optional subnet/route tag (admin vs user vs device pools). Present in `UserRow` (`database.go:28`). |
| `User.TailscaleKey` | string | Pre-auth device/auth key used to bring the IP up. Present in `UserRow` (`database.go:29`), secret (`json:"-"`). |

Persist-on-write already happens via `AuthManager.persistUser` (`auth.go:229`) and the DB `UserRow`
columns (`database.go:324,356,407`). **No new migration is required**; the columns exist. The key change
is that the **IP must be allocated from a managed pool, not random** (see §5).

The in-memory `Server.userTailscaleIPs` map (`server.go:79`, type `UserTailscaleIP` at `server.go:46`)
is **ephemeral (24h expiry, in-memory only)**. It duplicates `auth.User` data and is the source of
instability. The design moves the authoritative store to `auth.User`/DB and demotes the map to an
optional hot cache.

### 2.2 WebOS device identity (no login)

A **device** is a distinct identity from a **user**. Introduce a new entity:

```go
// New: apps/gateway/internal/tunnel/device.go (or extend server.go)
type DeviceTailscale struct {
    DeviceID    string    `json:"deviceId"`    // stable client/device UUID, generated client-side
    TailscaleIP string    `json:"tailscaleIp"` // distinct from any user IP
    TailscaleKey string   `json:"-"`           // pre-auth key; secret
    ProxyMode   string    `json:"proxyMode"`   // "direct" | "relay" | "disabled"
    CreatedAt   time.Time `json:"createdAt"`
    LastSeen    time.Time `json:"lastSeen"`
    UserID      string    `json:"userId,omitempty"` // bound later if the device logs in
}
```

Key concept: the **device ID is generated and persisted by the WebOS client** (e.g. `localStorage` /
IndexedDB), sent on the register call. The gateway allocates an IP **keyed by device ID**, never requiring
a user token. Optionally bind `UserID` to the device once a login happens, so a logged-in user's device
and per-user IP can be correlated by admins without merging the two IPs.

Store devices in a new persisted table `DeviceTailscaleRow` (add `database` CRUD alongside `UserRow`),
keyed by `DeviceID`. This keeps guest WebOS IPs stable across reloads **without** a user session.

### 2.3 Allocation pools

| Pool | CIDR range (example) | Used for |
|---|---|---|
| User/admin | `100.64.0.0/18` | Per-user `auth.User.TailscaleIP` |
| WebOS device | `100.64.64.0/18` | `DeviceTailscale.TailscaleIP` (no login) |

Splitting ranges prevents a guest device IP from colliding with a user IP and makes firewall/route rules
trivial to express.

---

## 3. Auth / Session Flow

### 3.1 Logged-in user gets their saved IP on next login

Today `GET/POST /auth/tailscale-ip` is wired to `auth.RequireAuth(authMgr, server.authHandler.AllocateIP)`
(`server.go:270`) → `AuthHandler.AllocateIP` (`auth.go:425`) → `AuthManager.AllocateTailscaleIP`
(`auth.go:251`).

Current behavior is **buggy for stability**: `AllocateTailscaleIP` generates a random
`100.<rand>.<rand>.<rand>` IP on first call and returns the stored one after (`auth.go:258-264`). The
random generator can collide, and there is no pool/exhaustion tracking.

Desired flow:

1. Client calls `POST /auth/tailscale-ip` (with bearer token).
2. `AllocateTailscaleIP(userID)`:
   - If `user.TailscaleIP != ""` → return it (stable across logins). ✅ already correct.
   - Else → **draw the next free IP from the user/admin pool**, set `user.TailscaleIP`, set default
     `user.TailscaleProxy = "relay"`, persist via `persistUser`, and (if `TailscaleKey` unset) mint a
     pre-auth key.
3. Response `{"tailscaleIp": "100.64.0.12", "proxyMode": "relay"}`.

This already returns the same IP each login because it is persisted in `UserRow`; the fix is to make the
allocation **deterministic from a pool** instead of random (`auth.go:261`).

### 3.2 WebOS obtains an IP via device identity (no login)

New unauthenticated endpoint (mirrors the host register pattern which is also unauthenticated):

```
POST /tailscale/device/register
  body: { "deviceId": "dev_abc123", "hostName": "webos-<id>", "userAgent": "..." }
  resp: { "deviceId": "dev_abc123", "tailscaleIp": "100.64.64.5", "proxyMode": "relay", "tailscaleKey": "..." }
```

Flow:

1. WebOS client generates/persists a `deviceId` locally (first run only).
2. On boot, calls `POST /tailscale/device/register` **without** a user token.
3. Gateway: if `DeviceTailscale[deviceId]` exists → return stored IP (stable across reloads). Else draw
   from the **device pool**, mint a key, persist, return.
4. WebOS uses `tailscaleIp` + `tailscaleKey` to bring up its Tailscale node (host-agent-style
   `tailscale up --authkey`).
5. When the user later logs in, the WebOS frontend may call `POST /tailscale/device/bind`
   `{ "deviceId", "userId" }` so admins can correlate; the device keeps its **own** IP.

Security note: device registration must be rate-limited and the device pool sized so anonymous guests
cannot exhaust it (see §7). Consider a short-lived device token returned at register time to gate later
`/tailscale/proxy` use.

### 3.3 Admins map their account

Admins are ordinary `auth.User` rows with `Role == "admin"` (`database.go:33`). They get their IP through
the **same** `POST /auth/tailscale-ip` path — no separate code. The only difference:

- Their IP is drawn from the **user/admin pool** (same as users), or (optional) a dedicated admin CIDR for
  route/firewall separation via `User.TailscaleCIDR`.
- The admin UI (`apps/frontend/app/webos/apps/admin/Admin.tsx:19,162`) already displays
  `u.tailscaleIp`; no change needed beyond surfacing `proxyMode`.

---

## 4. Proxy Fallback

### 4.1 Current state (partial)

- `Server.dialViaTailscale(route)` (`server.go:1238`) races registered `TailscaleHost`s and returns a raw
  `net.Conn`, or an error if none reachable. It currently ignores `UserTailscaleIP.ProxyMode` and the
  per-user/device IP entirely — it dials *any* Tailscale host, not the user's specific IP.
- `Server.handleTailscaleProxy` (`server.go:1140`) accepts `{targetIp, targetPort, protocol, userId}`,
  builds a route ID and proxy URLs, and returns `{"fallback": true}`. **Gap:** it does **not** actually
  proxy — it just returns instructions; the gateway never opens the connecting tunnel, and it does not key
  off the user's saved IP or `proxyMode`.

### 4.2 Precise fallback logic

Define the decision in **one place**, called by the tunnel dialer and by the proxy handler:

```
function resolveTailscalePath(targetIP, proxyMode, userID/deviceID):
  if proxyMode == "disabled":
      return DIRECT_ONLY            // dial targetIP directly; fail if blocked
  if proxyMode == "relay":
      return ALWAYS_PROXY           // never dial direct; use /tailscale/proxy
  // proxyMode == "direct" (default)
  try dial targetIP directly with 5s timeout (dialViaTailscale-style probe)
  if success: return DIRECT
  else:       return PROXY_FALLBACK // upgrade to /tailscale/proxy
```

Changes:

1. **`dialViaTailscale` (`server.go:1238`).** Add a `targetIP` parameter (the user's or device's specific
   Tailscale IP). Remove the "dial any host" behavior. Probe **only** `targetIP:targetPort`. If the probe
   fails within the 5s budget, return a sentinel `errTailscaleUnreachable` instead of `firstErr`.
2. **`handleTailscaleProxy` (`server.go:1140`).** Make it a **working fallback**, not just metadata:
   - Resolve `targetIP` from `userId`/`deviceId` (look up `auth.User.TailscaleIP` or
     `DeviceTailscale.TailscaleIP`) when `targetIp` is omitted.
   - Actually register a temporary proxy route in the `Registry` (like `handleReverseProxy` does) pointing
     at `targetIP:targetPort`, and return the real `/ws/<routeID>` and `/connect/<routeID>` endpoints the
     client can use immediately.
   - Set the created route's transport to `tailscale` so metrics/quality tracking recognize it.
3. **Call site.** Wherever a tunnel is established for a user/device target, call `resolveTailscalePath`;
   on `PROXY_FALLBACK`/`ALWAYS_PROXY`, redirect the client to the `/tailscale/proxy` endpoints.

---

## 5. Implementation Steps

### 5.1 `apps/gateway/internal/auth/auth.go`

- **`AllocateTailscaleIP(userID)` (`auth.go:251`).** Replace the random `fmt.Sprintf` generator
  (`auth.go:261`) with a pool allocator:
  - Maintain a free-list / bitmap of the user/admin CIDR (e.g. `100.64.0.0/18`).
  - On first allocation, pop the next free IP, set `user.TailscaleIP`, default
    `user.TailscaleProxy = "relay"`, mint `user.TailscaleKey` if empty, set `user.TailscaleCIDR`, then
    `persistUser`.
  - If pool exhausted → return `""` + a new `ErrTailscalePoolExhausted`.
- Add `AllocateDeviceTailscaleIP(deviceID string) (string, string, error)` that operates on a
  **device pool** (`100.64.64.0/18`) and returns `(ip, key, err)`. Reuse the same free-list machinery.
- Add `BindDeviceToUser(deviceID, userID)` for the optional correlation.

### 5.2 `apps/gateway/internal/database/database.go`

- Add `DeviceTailscaleRow` (fields per §2.2) and CRUD: `CreateDeviceTailscale`, `GetDeviceTailscale`,
  `UpdateDeviceTailscale`, `ListDeviceTailscale`. Mirror the `UserRow` pattern (`database.go:21-38`,
  `324`, `356`, `407`).

### 5.3 `apps/gateway/internal/tunnel/server.go`

- Add `deviceTailscale map[string]*DeviceTailscale` + `deviceMu` (mirror `userTailscaleIPs` at
  `server.go:79`).
- New handler `handleTailscaleDeviceRegister` (unauthenticated) at a new mux route
  `/tailscale/device/register` (next to `server.go:239-242`); calls `AllocateDeviceTailscaleIP`.
- New handler `handleTailscaleDeviceBind` (authenticated) at `/tailscale/device/bind`.
- Modify `dialViaTailscale` (`server.go:1238`) to take `targetIP string` and probe only that IP; return a
  typed unreachable error.
- Rewrite `handleTailscaleProxy` (`server.go:1140`) to resolve `targetIp` from `userId`/`deviceId`, create
  a real Registry route, and return working `/ws/` + `/connect/` endpoints.
- Add `resolveTailscalePath(targetIP, proxyMode, id)` helper (§4.2) and wire it into tunnel establishment.
- Remove dependence on the ephemeral `userTailscaleIPs` map for authoritative data; keep it only as an
  optional cache, or delete it and the `/tailscale/user` GET/POST duplication (`server.go:1077`) in favor
  of the persisted `auth.User` + `DeviceTailscale` stores.

### 5.4 `host-agent/cmd/host-agent/tailscale.go`

- `TailscaleManager` (`tailscale.go:16`) already does `Discover`/`HostIP`/`RegisterWithGateway`. No change
  needed for user/device IPs (those are client-side). If the host agent should also honor `proxyMode`,
  pass it through `TailscaleHostInfo` (`tailscale.go:26`) in the register body — optional.

### 5.5 `apps/frontend/app/webos/...`

- `Settings.tsx:19-29` already calls `POST /auth/tailscale-ip` and shows `tailscaleIp`. Add display of
  `proxyMode`.
- Add a device bootstrap (e.g. in `boot/BootSequence.tsx` or a new `webos/system/tailscale.ts`) that
  generates a `deviceId`, calls `POST /tailscale/device/register` **before/without login**, and stores the
  returned IP + key so the desktop has connectivity as a guest.

---

## 6. Acceptance Criteria

| # | Criterion | How to verify |
|---|---|---|
| 1 | **Stable per-user IP** | `POST /auth/tailscale-ip` with Alice's token → `100.64.0.12`. Repeat after logout/login → same `100.64.0.12`. Bob → a **different** IP. |
| 2 | **No collision** | Allocate N users (N > pool/2); assert all returned IPs are unique and within `100.64.0.0/18`. |
| 3 | **WebOS no-login IP** | From a logged-out WebOS session, `POST /tailscale/device/register` with a `deviceId` → returns a `100.64.64.x` IP. Reload WebOS (same `deviceId`) → same IP, **no token required**. |
| 4 | **Device vs user separation** | A device IP and a user IP never overlap; WebOS guest IP differs from the user's per-user IP after login. |
| 5 | **Admin mapping** | Admin logs in → receives own IP via `/auth/tailscale-ip`; `proxyMode` returned correctly; admin UI shows it. |
| 6 | **Direct path works** | With Tailscale reachable, dialing the user IP succeeds; `dialViaTailscale` returns a conn to the **specific** user IP. |
| 7 | **Proxy fallback works** | Block direct reachability to the user IP (firewall/DROP). Request returns a working `/tailscale/proxy` route; `/ws/<routeID>` and `/connect/<routeID>` actually reach `targetIP:targetPort`. |
| 8 | **`proxyMode=disabled`** | User with `proxyMode=disabled` never gets a proxy route; direct-only failures return an error, not a fallback. |
| 9 | **Persistence** | Restart gateway (DB-backed). User/device IPs survive because they are in `UserRow` / `DeviceTailscaleRow`. |

---

## 7. Open Questions / Risks

- **Tailscale auth token provisioning.** Where do pre-auth keys (`TailscaleKey`) come from? Options:
  (a) gateway calls the Tailscale Admin API with a stored `TS_API_KEY`; (b) host-agent already joined the
  tailnet and advertises routes for user/device IPs (`tailscale up --advertise-routes`). The current code
  only runs `tailscale ip -4` / `tailscale status` (`tailscale.go:132,147`) — there is **no key minting
  yet**. This is the biggest unknown.
- **IP exhaustion.** The user/admin pool (`/18` ≈ 16k IPs) and device pool (`/18`) are finite. Need:
  reclaim on user delete (`DeleteUser`, `auth.go:299`), TTL/reap for idle device IPs, and a clear
  `ErrTailscalePoolExhausted` path.
- **Route advertisement.** For a user/device IP to be *routable* over Tailscale, the tailnet must
  `--advertise-routes` those IPs and the tailnet must approve them. Who advertises — host-agent, or a
  dedicated gateway node? Not implemented today.
- **Guest abuse.** Unauthenticated `/tailscale/device/register` is an attack surface (IP exhaustion,
  tailnet spam). Mitigate with rate limiting, device-token gating, and pool caps.
- **`userTailscaleIPs` double source of truth.** The in-memory 24h map (`server.go:79`) duplicates
  `auth.User`. Plan to retire it to avoid drift; the doc currently preserves it as optional cache only.
- **`dialViaTailscale` caller.** The function exists but its call site in the route dial path must pass the
  resolved per-user/device `targetIP` and honor `proxyMode`; confirm the dial path integration point.
- **CIDR split vs single pool.** Whether to use separate user/device CIDRs (recommended for firewall clarity)
  or one shared pool with tags is a product decision.
