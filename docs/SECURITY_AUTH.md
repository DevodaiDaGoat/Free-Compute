# Security & Auth Deep Design

Authentication and authorization foundation for gateway, host-agent, and all tunnel/session surfaces. Current state: operational but several gaps remain.

## Auth Model

| Layer | Mechanism | Location |
|-------|-----------|----------|
| API auth | JWT HS256 bearer token (24h expiry, refresh token) | `apps/gateway/internal/auth/auth.go:312` `generateTokens` |
| Route auth | Per-route `RequireAuth` flag + global `FREECOMPUTE_TUNNEL_TOKEN` | `apps/gateway/internal/tunnel/auth.go:9` `authorize` |
| Host-agent auth | Bearer `FREECOMPUTE_AGENT_TOKEN` sent over TCP CONNECT | `host-agent/cmd/host-agent/main.go:266` |
| Admin auth | `RequireAdmin` checks `user.Email == AdminEmail` (hardcoded `admin/w@t3rm3n`) | `apps/gateway/internal/admin/admin.go:129` |

**Current gaps:**
- Admin is a single hardcoded user with no RBAC.
- Tokens are 24h with fixed expiry; no shorter-lived scoped tokens for sensitive actions.
- `FREECOMPUTE_AGENT_TOKEN` travels as plain Bearer over a TCP connection (HTTP CONNECT) — plaintext in transit if gateway is HTTP, or only TLS-protected if `https://` scheme is used. No mTLS.
- No device verification, no brute-force/IP rate limiting on auth endpoints (`/auth/register`, `/auth/login`).
- Security events are logged to `logger.Printf` but not persisted to a structured audit log.

## mTLS Between Gateway ↔ Host-Agent

`host-agent/main.go` dials gateway with `net.Dialer` or `tls.Dialer`. When using HTTPS, it sets `InsecureSkipVerify` from env. Replace with mTLS:

1. Add `FREECOMPUTE_AGENT_CERT`, `FREECOMPUTE_AGENT_KEY`, `FREECOMPUTE_GATEWAY_CA_CERT`.
2. In `connectToGateway` (`host-agent/cmd/host-agent/main.go:241`), load client cert and set `tls.Config.Certificates`.
3. In `withCommonHeaders` (`apps/gateway/internal/tunnel/server.go:416`), the gateway TLS config is external (ListenAndServe); use `http.Server.TLSConfig` with `ClientCAs` to require client cert on `/agent/*` routes.
4. If `tls.ClientAuthType == tls.RequireAndVerifyClientCert`, the token becomes redundant for agent connections (mutual auth already authenticates the agent). Keep token for session-scoped route selection.

## End-to-End Encryption

- WebRTC media (DTLS-SRTP) is already encrypted in transit between peers.
- TCP/UDP streams through `/connect/`, `/ws/`, `/ssh/` are encrypted at the transport layer if mTLS is enabled.
- File transfer chunks (`/transfer/`) travel inside the tunnel — encrypt at the application layer if the upstream target is not TLS.
- Signaling (`/signal/`) should use WSS via BunnyCDN edge; add HSTS and CORS restrictions.

## Token Hardening

1. Replace the 24h fixed token lifetime with configurable expiry (`JWTExpirySeconds` env).
2. Add scope claims (`"scopes": ["proxy", "agent", "admin"]`) in `generateJWT` (`auth.go:325`); routes gate on scope, not just presence.
3. Reduce refresh token to 7 days; revoke on password change.
4. Use `subtle.ConstantTimeCompare` (already in `tunnel/auth.go:22`) for all token comparisons.

## Device Verification

- After login, present the current device's Tailscale IP / host fingerprint.
- Record `lastLoginIP`, `lastLoginUserAgent` in `User` model.
- Alert via `SecurityHandler` if login from a new device or region.

## Brute-Force Protection

No rate limiting exists on `/auth/login` or `/auth/register`. Add a per-IP failure counter in `AuthManager`:

- After 5 failed logins in 5 minutes, require a 30s cooldown (use a token-bucket per remote address).
- Return `429 Too Many Requests` with `Retry-After`.
- Accept via `X-Forwarded-For` when `CDNHostname` is set.

## Audit Auth Events

Current: `SecurityDetector` only logs to stdout. Add an `AuditLogger` in `apps/gateway/internal/session/audit.go` (already exists for sessions) or extend it to auth:

- Events: `login_success`, `login_failure`, `register`, `logout`, `token_refresh`, `admin_action`, `route_access_denied`.
- Fields: `eventType`, `userID`, `remoteAddr`, `userAgent`, `resource`, `decision`, `timestamp`.
- Persist to `database` via `INSERT INTO audit_log` (add `audit_logs` table to `database/schema.sql`).

## References

- `docs/REMOTE_ACCESS_STREAMING.md:68-73` — Remote Support requires approval, audit logging, and device verification.
- `apps/gateway/internal/admin/admin.go:129` `RequireAdmin` implementation.
- `apps/gateway/internal/auth/auth.go:120` `Register` and `auth.go:171` `Login` — endpoints without brute-force guard.
