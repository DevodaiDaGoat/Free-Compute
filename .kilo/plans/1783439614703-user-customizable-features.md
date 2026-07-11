# FreeCompute — Ordered Implementation Plan (personalization, trust & safety, DB + VM ergonomics)

> Make the platform more **user-customizable** and **trustworthy**, **optimize the DB scripts**,
> and make **VMs easy to test/configure**. Workstreams run **one after the other**; each is
> self-contained so an agent can implement + validate it alone. Plan only — no source edits.
> **Order (user-chosen):** 0 DB → 1 VM → **2 Personalization (A)** → 3 Moderator (B) →
> 4 Abuse/traffic detection (D) → 5 Reporting + conditional AI (C) →
> **6 Integration, verify, optimize & fix bugs** (final pass).

## Foundations (verified)
- DB: `apps/gateway/internal/database/database.go` (`Open` L138, `migrate` L160; tables L162–292;
  queries parameterized with `?`). `users.storage_quota` default = 10 GB (`10737418240`).
- Route mounts (server.go): `/auth/*` at L267–270 use `auth.RequireAuth(authMgr, h)`;
  `/admin/*` at L256–264 use `adminWrap` = `auth.RequireAuth(authMgr, server.adminHandler.RequireAdmin(h))`.
  `RequireAdmin` (admin.go:129) hardcodes `user.Email=="admin"`. No `/auth/preferences`,
  `/reports`, or `/admin/personalization` exist yet (safe to add).
- VM: `host-agent/vm-setup.go` (`VMAgent`, `main` L893 hardcodes config),
  `host-agent/cmd/host-agent/vm_manager.go` (`LaunchVM`/`buildQEMUArgs` run real QEMU),
  `run-backend.sh` L94–98 (builds vm-setup, never starts it), `docker-compose.yml` (no vm-agent).
- Usage: `apps/gateway/internal/usage/usage.go` `Tracker` has `Track/GetUsage/GetQuota/CheckQuota`
  — usable for per-user traffic in Phase 4.
- Quality: `apps/gateway/internal/tunnel/quality.go` `ConnQuality.Update(rtt, loss, bandwidthBps)`
  — bandwidth signal for Phase 4.

---

## Phase 0 — Optimize DB scripts (first)
1. **Migration runner.** In `migrate()` (L160), after the `CREATE TABLE IF NOT EXISTS` block, run a
   `[]string` of forward migrations; wrap each `db.Exec(stmt)` so **"duplicate column"** /
   **"already exists"** errors are ignored (match substring). Add the DDL in §Spec DDL.
2. **Indexes** (idempotent `CREATE INDEX IF NOT EXISTS`) — see §Spec DDL.
3. Keep parameterized queries + `SetMaxOpenConns(1)`; optionally `PRAGMA optimize` on open.
- **Test:** `cd apps/gateway && go build ./...`; add `internal/database/migrate_test.go` asserting
  `migrate()` is idempotent (run twice on a temp DB, no error, columns present).

## Phase 1 — Easy VM test & configure (second)
1. **Config-driven VM agent.** Move `VMAgentConfig` + routes out of `main()` (vm-setup.go:893) into
   env loading (mirror `host-agent/cmd/host-agent/main.go` `loadConfig`): `FREECOMPUTE_VM_ID`,
   `FREECOMPUTE_VM_REGION`, `FREECOMPUTE_VM_GPU`, `FREECOMPUTE_VM_ROUTES` (JSON); defaults = current
   hardcoded test values.
2. **Share code.** Put `VMAgent` in importable `host-agent/internal/vmagent`; delete root
   `package main` duplicate so `cmd/vm-setup` reuses it.
3. **`--dry-run`**: resolve config + print QEMU (`vm_manager.buildQEMUArgs`) + ffmpeg args without
   executing; add `DryRun` bool to `vm_manager.LaunchVM` (print, don't `exec.Command`).
4. **`--self-test`**: register with gateway (or print payload if down) + emit metrics (replace
   constant `getCPUUsage()` stubs with real reads when available; fake behind flag for CI).
5. **Compose + script.** Add `vm-agent` service to `docker-compose.yml` (depends_on gateway
   healthy, env-driven) + `test-vm.sh`; update `.env.example` with `FREECOMPUTE_VM_*`.
- **Test:** `cd host-agent && go build ./...`; `./bin/freecompute-vm-setup --dry-run` prints args
  and exits 0 with no KVM; `docker compose up vm-agent` registers a VM visible in `/hosts/metrics`.

## Phase 2 — Pillar A: Personalization (FIRST FEATURE PILLAR)
> Storage shared per-user (10 GB); personalization per-user, NOT shared by default; cross-device
> sync is OPT-IN (user requests, moderator accepts).
1. **K1** (column added Phase 0): `GET/PUT /auth/preferences` → `AuthManager.Get/SavePreferences`
   → `DB.Get/SetUserPreferences`. Blob JSON-validated, capped 64 KB. Frontend
   `Settings.tsx`/`ConnectionSettings.tsx` hydrate on mount, `PUT` on change.
2. **Opt-in sync:** `POST /auth/personalization/sync-request` → `personalization_sync_requests`;
   `POST /admin/personalization/sync-approve` (moderator+) copies blob across device entries;
   audit-logged.
3. Follow-ups: saved stream presets (`/auth/presets`), default session mode/class pre-filling
   `CreateSessionRequest` (`session.go:216`), theme/wallpaper, pinned apps, keybindings, a11y,
   notification prefs.
- **Test:** `curl -XPUT /auth/preferences -d '{"connection":{"streamPreset":"fast"}}'` then GET
  returns it; restart gateway → persists; cross-device copy only after `/admin/.../sync-approve`.

## Phase 3 — Pillar B: Moderator role & team management
1. Add `Role string` to `User` (`auth.go:30`); load/save via `UserRow` (has `Role`).
2. Replace `RequireAdmin` (admin.go:129) with `RequireRole(min int, authMgr, next)` (see §Spec).
   Roles `user=0, moderator=1, admin=2`. Seed bootstrap admin via `FREECOMPUTE_ADMIN_EMAIL`
   (keep `SeatAdmin()` behavior). `adminWrap` becomes `RequireAuth(authMgr, RequireRole(2, authMgr, h))`.
3. **Moderator perms:** CAN review/resolve reports, view/pause/flag VMs, approve personalization
   sync, suspend users (reversible). CANNOT edit `SystemSettings`, firewall rules, audit logs, or
   issue infra creds. `POST /admin/role` (admin-only) promote/demote; audit-logged.
- **Test:** `RequireRole(1)` admits moderator+admin, 403s user; bootstrap admin works; moderator
  `POST /admin/settings` → 403.

## Phase 4 — Pillar D: Traffic/abuse detection (build on `security/detector.go`)
1. **Wire live data.** Feed `host-agent` `runStatusReporter` (`/hosts/metrics`) + per-user traffic
   (`usage.Tracker.GetUsage(userID, since)` bytes) into `AnalyzeMetrics`/`AnalyzeProcessList`/new
   `AnalyzeTraffic(userID string, bytesIn, bytesOut, conns int, dur time.Duration) *ThreatEvent`.
2. **Extend signatures** (`initSignatures`): cryptomining pool/miner + illegal-activity heuristics;
   make the signature table updatable (load from DB/`settings` not only hardcoded).
3. On hit → `ThreatEvent` + moderator notice (Phase 3 surface); severe → `PauseVM` (moderator-confirmed).
4. **Privacy:** metadata + sampled process names only; no file/keystroke inspection.
- **Test:** unit-test `AnalyzeProcessList` with a known miner process name → non-nil `ThreatEvent`;
  simulated traffic spike → anomaly flagged; `PauseVM` only after moderator confirm.

## Phase 5 — Pillar C: Reporting + conditional AI moderation
1. **Reports:** `POST /reports` (auth) → `reports` table; `GET /reports/list` (moderator+);
   `POST /reports/action` (moderator+: warn/ban/pause/resolve); audit-logged.
2. **Conditional AI.** `ModerationAI` interface `Triage(ctx, ReportInput) Action`. Default =
   **local heuristic** (severity/keyword/pattern). Optional **LLM adapter invoked ONLY when**
   `ai_moderation_active` is set: (a) resource spike via `AnalyzeMetrics`, or (b) suspicious
   traffic/signature hit from Phase 4. AI output **advisory**; moderator confirms.
3. Trigger plumbing: `SecurityDetector` flag sets `ai_moderation_active` for the next triage window;
   heuristic always runs, LLM only when flag set.
- **Test:** `POST /reports` → pending; moderator action resolves + logs; with flag OFF, assert no
  external LLM call; with flag ON (simulated spike), LLM path invoked.

## Phase 6 — Integration, verification, optimization & bugfix (FINAL)
Run **after** Phases 0–5 are complete. This is the "does it actually work, make it fast, fix bugs"
pass the whole effort has been building toward (and the original ask: run the backend, confirm it
works, then optimize and fix bugs).
1. **Build & run end-to-end.** `cd apps/gateway && go build ./...`; `cd host-agent && go build ./...`;
   `npm run build` (frontend). Start via `./run-backend.sh` (gateway + host-agent) and
   `./start-website.sh` (WebOS on :3000), plus `./test-vm.sh` (Phase 1) to register a VM. Confirm
   `/healthz`, `/capabilities`, and a live VM in `/hosts/metrics`.
2. **Smoke test the features.** Personalization round-trips (`/auth/preferences`); a moderator can
   resolve a `/reports` item but is blocked from `/admin/settings`; a simulated miner process name
   raises a `ThreatEvent`; `ai_moderation_active` flips on only under a resource/spike trigger.
3. **Optimize (the "make it fastest" goal).** Re-apply `docs/FASTEST_PATH.md`: transport
   auto-negotiation (WebRTC P2P > WS > CONNECT > proxy), connection coalescing/multiplexing
   (`connection_fusion.go`), wire `FREECOMPUTE_TCP_BUFFER_SIZE` into `applyTCPSocketOptions`
   (currently ignored — see `buffer_pool.go`), use `dnsCacheDialer` for proxy upstreams, BBR +
   NOTSENT_LOWAT already set. Also apply `docs/OPTIMIZATION_TIPS.md` and the cache-bypass rules in
   `docs/EDGE_BUNNYCDN_DEPLOYMENT.md`.
4. **Fix bugs.** Capture any build/runtime errors and fix them; re-confirm no reconnect storms
   (gateway/agent token match, `vm-ssh` route target `"agent"`), no premature connection closes
   (`agent_pool.go`), and a clean compile. Re-run every per-phase validation block.
5. **Light load/soak.** Exercise `/proxy`, `/connect`, `/ws` concurrently; watch `metrics` /
   `health/detail`; verify no goroutine/FD leaks and p95 latency within `docs/CONNECTION_QUALITY.md`
   SLOs.
- **Test:** full stack builds + runs; all Phase 0–5 smoke checks pass; `go vet ./...` clean; a
  concurrent tunnel soak shows stable FD count and p95 latency within target.

---

## §Spec — Concrete DDL (Phase 0)
```sql
ALTER TABLE users ADD COLUMN preferences TEXT NOT NULL DEFAULT '{}';

CREATE TABLE IF NOT EXISTS reports (
  id TEXT PRIMARY KEY,
  reporter_id TEXT NOT NULL,
  target_type TEXT NOT NULL,
  target_id TEXT NOT NULL,
  reason TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  reviewer_id TEXT NOT NULL DEFAULT '',
  action TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY (reporter_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS personalization_sync_requests (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  reviewer_id TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_sessions_user       ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_vms_user           ON vms(user_id);
CREATE INDEX IF NOT EXISTS idx_threats_user       ON threats(user_id);
CREATE INDEX IF NOT EXISTS idx_threats_vm         ON threats(vm_id);
CREATE INDEX IF NOT EXISTS idx_queue_user_status  ON queue(user_id, status);
CREATE INDEX IF NOT EXISTS idx_storage_files_user ON storage_files(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_user         ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_session      ON audit_logs(session_id);
CREATE INDEX IF NOT EXISTS idx_reports_target     ON reports(target_id, status);
```

## §Spec — Endpoint mounts (server.go)
```go
mux.HandleFunc("/auth/preferences",                    auth.RequireAuth(authMgr, server.authHandler.Preferences))
mux.HandleFunc("/auth/personalization/sync-request",  auth.RequireAuth(authMgr, server.authHandler.RequestPersonalizationSync))
mux.HandleFunc("/admin/personalization/sync-approve", adminWrap(server.adminHandler.ApprovePersonalizationSync))
mux.HandleFunc("/admin/role",                         adminWrap(server.adminHandler.SetRole))
mux.HandleFunc("/reports",        auth.RequireAuth(authMgr, server.reportHandler.Create))        // POST
mux.HandleFunc("/reports/list",   auth.RequireRole(1, authMgr, server.reportHandler.List))       // moderator+
mux.HandleFunc("/reports/action", auth.RequireRole(1, authMgr, server.reportHandler.Action))     // moderator+
```

## §Spec — Key signatures
```go
// auth.go
func RequireRole(min int, auth *AuthManager, next http.HandlerFunc) http.HandlerFunc
func (m *AuthManager) GetPreferences(userID string) (json.RawMessage, error)
func (m *AuthManager) SavePreferences(userID string, blob json.RawMessage) error
func (m *AuthManager) SetRole(userID, role string) error   // admin-only caller

// database.go
func (db *DB) GetUserPreferences(userID string) ([]byte, error)
func (db *DB) SetUserPreferences(userID string, blob []byte) error
func (db *DB) CreateReport(r *ReportRow) error
func (db *DB) ListReports() ([]*ReportRow, error)
func (db *DB) UpdateReport(id, reviewerID, status, action string) error
func (db *DB) CreateSyncRequest(userID string) (string, error)
func (db *DB) ApproveSyncRequest(id, reviewerID string) error

// security/detector.go
func (d *SecurityDetector) AnalyzeTraffic(userID string, bytesIn, bytesOut int64, conns int, dur time.Duration) *ThreatEvent

// moderation (new pkg, e.g. internal/moderation)
type ModerationAI interface { Triage(ctx context.Context, in ReportInput) (Action, error) }
type HeuristicModerator struct{}
type LLMModerator struct { URL, Key string }   // only called when ai_moderation_active
```

## §Resolved decisions
- **Moderator matrix:** CAN = review/resolve reports, view/pause/flag VMs, approve personalization
  sync, suspend users (reversible). CANNOT = edit SystemSettings, firewall, audit logs, infra creds.
- **ModerationAI LLM:** OpenAI-compatible HTTP via `FREECOMPUTE_MODERATION_LLM_URL` /
  `FREECOMPUTE_MODERATION_LLM_KEY`; default `HeuristicModerator` always on; `LLMModerator` invoked
  ONLY when `ai_moderation_active` flag set by Phase 4 trigger. AI output advisory.
- **Sync granularity:** whole-blob copy (simplest; per-section deferred).
- **VMAgent package:** `host-agent/internal/vmagent` (importable by `cmd/vm-setup` and `cmd/host-agent`).

## Global validation
- `go build ./...` (gateway + host-agent) and `npm run typecheck` (frontend) green after each phase.
- Each phase has its own validation block above; run phases strictly in order.
- Add a `migrate_test.go` (Phase 0) and `RequireRole` auth test (Phase 3) so regressions are caught.
- **Phase 6 is the final gate:** full stack builds, runs, all smoke checks pass, `go vet ./...` clean,
  and a concurrent soak is stable before calling the work done.
