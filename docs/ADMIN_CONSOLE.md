# Admin Console

Admin console features, existing endpoints, and the WebOS admin UI.

---

## 1. Backend admin surface

### Manager + handler (`apps/gateway/internal/admin/admin.go`)
`AdminManager` (admin.go:22) holds auth, the `security.SecurityDetector`, settings, and an auto-detected domain. `AdminHandler` (admin.go:115) implements each endpoint. `RequireAdmin` (admin.go:129) gates on `user.Email == "admin"`. Routes are mounted in `server.go` under `adminWrap` = `auth.RequireAuth` + `RequireAdmin` (server.go:253-264).

**Existing endpoints:**

| Method | Path | Handler | Notes |
|--------|------|---------|-------|
| GET | `/admin/dashboard` | `Dashboard` (admin.go:140) | totals: threats, paused/flagged VMs, active threats, settings |
| GET | `/admin/users` | `ListUsers` (admin.go:164) | email, displayName, storage, Tailscale IP |
| DELETE | `/admin/users/delete` | `DeleteUser` (admin.go:189) | `?userId=` |
| GET | `/admin/threats` | `ListThreats` (admin.go:202) | `?resolved=true` |
| POST | `/admin/threats/review` | `ReviewThreat` (admin.go:208) | `{threatId,action,resolved}`; `resume` calls `ResumeVM` |
| GET | `/admin/vm/pause` | `PauseVM` (admin.go:229) | `?vmId=` |
| GET | `/admin/vm/resume` | `ResumeVM` (admin.go:239) | `?vmId=` |
| GET/POST | `/admin/settings` | `Settings` (admin.go:249) | `SystemSettings` GET/POST |
| GET | `/admin/auto-detect` | `AutoDetect` (admin.go:266) | **not** wrapped in `RequireAuth` (server.go:264) |

### Security (`apps/gateway/internal/security/detector.go`)
`SecurityDetector` tracks `ThreatEvent`s (level `low|medium|high|critical`) and per-VM `VMThreatState` (state `clean|flagged|paused|quarantine`, `Score`, CPU/GPU/network). `AnalyzeMetrics`/`AnalyzeProcessList` score crypto-mining and malware signatures; `PauseVM`/`ResumeVM`/`ReviewThreat`/`ThreatCount`/`ListVMStates` back the admin endpoints. Stats at `/security/stats`.

### Firewall (`apps/gateway/internal/firewall/firewall.go`)
`Manager` with `Rule` and `SecurityGroup`. Routes `/firewall/rules` (GET/POST), `/firewall/rules/{id}` (GET/PUT/DELETE), `/firewall/groups`, `/firewall/assign` (server.go:303-306). **Registered without admin auth** — any caller may read/edit rules.

### Usage & billing (`apps/gateway/internal/usage/usage.go`)
`Tracker` records resource usage; routes `/usage`, `/quota`, `/invoice` (server.go:309-311). `UsageSummary`, `Quota`, `Invoice` types exist. **Not admin-protected** — user-scoped only by `?userId=`.

---

## 2. What the dashboard should show

- **Overview:** active threats, total threats, paused/flagged VMs, sessions active, agents, error rate (from `/health/detail` + metrics).
- **Users:** list with storage used/quota, Tailscale IP, created date; delete action; quota edit.
- **Threats:** severity-coded cards with evidence, optional screenshot (`ThreatEvent.ScreenShot`), review actions (false-positive / confirm+quarantine / resume).
- **VMs:** per-VM state, score, CPU/GPU/network; pause/resume controls.
- **Firewall:** rule table (port range, CIDR, action, priority, enabled) + security groups; create/edit/delete/assign.
- **Usage/Billing:** per-user usage, quota, and generated invoice (`GenerateInvoice`).
- **Settings:** editable `SystemSettings` (max users, quotas, threat/AI-review toggles, session limits).
- **Domain:** `AutoDetect` result.

---

## 3. Gaps

- **Hardcoded admin creds** (`admin` / `w@t3rm3n`, admin.go:16-20) seeded in `SeedAdmin`.
- **`/admin/auto-detect` is unauthenticated** — leaks host/remoteAddr.
- **Firewall and usage endpoints are not admin-gated** — privilege gap.
- **Settings POST is a stub** (admin.go:254-261) — accepts body, returns `saved`, never persists.
- **No firewall, usage, or VM-pause UI** in the frontend admin app.
- **No pagination** on users/threats lists.
- **Screenshots/evidence** not surfaced in UI.

---

## 4. WebOS admin UI (`apps/frontend/app/webos/apps/admin/Admin.tsx`)

Existing `AdminApp` has tabs `dashboard | users | threats | settings | domain` and calls `/admin/*` via `apiFetch`. It is **read-only** for settings and has no firewall/usage/VM controls.

**Proposed additions:**
1. **VM tab:** list `security` VM states via a new `/admin/vms` endpoint; wire Pause/Resume to `/admin/vm/pause|resume`.
2. **Firewall tab:** CRUD against `/firewall/rules` and group assignment.
3. **Usage/Billing tab:** render `/usage`, `/quota`, `/invoice` for a selected user.
4. **Settings edit:** POST to `/admin/settings` once persistence is implemented.
5. **Threat evidence:** render screenshot + per-threat evidence JSON; review actions already exist.
6. **Protect auto-detect:** move under admin auth or strip remoteAddr.

---

## 5. Implementation steps

1. Gate `/firewall/*` and `/usage`,`/quota`,`/invoice` behind `adminWrap`; gate `/admin/auto-detect`.
2. Persist `SystemSettings` (store in `AdminManager`, honor in `RequireAdmin`/session logic).
3. Add `/admin/vms` returning `ListVMStates()`.
4. Extend `Admin.tsx` with VM, Firewall, Usage tabs; make Settings editable.
5. Add pagination + screenshot rendering to existing tabs.
6. Remove hardcoded creds; externalize admin bootstrap via env/`FREECOMPUTE_*`.

---

## 6. Acceptance criteria

- Firewall/usage endpoints reject non-admin callers; `/admin/auto-detect` requires auth or omits remoteAddr.
- Settings POST writes and GET reflects persisted values.
- WebOS Admin shows VM, Firewall, and Usage tabs; pause/resume and rule CRUD work end-to-end.
- No hardcoded credentials in source; admin seeded from env.
- `go build ./...` and `npm run build` (frontend) succeed.
