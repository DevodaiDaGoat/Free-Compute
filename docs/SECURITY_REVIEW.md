# Architecture Security Review

This document identifies security concerns in the planned FreeCompute architecture based on the current design documents (`BACKEND_STRUCTURE.md`, `DESKTOP_STRUCTURE.md`).

---

## Critical Issues

### 1. Admin Endpoints Lack Documented Auth Guard

**Location:** `apps/gateway/` — API endpoints  
**Concern:** The admin routes (`GET /admin/hosts`, `POST /admin/hosts/:id/restart`) are listed alongside user routes with no indication of role-based access control (RBAC) or separate admin middleware.

**Risk:** Without explicit admin-only middleware, any authenticated user could potentially manage host infrastructure.

**Recommendation:**
- Add dedicated `adminAuth` middleware that verifies admin role claims in the JWT.
- Separate admin routes into their own router group with stricter rate limits.
- Log all admin actions to an audit trail.

---

### 2. No Rate Limiting on Authentication Endpoints

**Location:** `apps/gateway/internal/middleware/ratelimit.go`, `apps/auth-service/`  
**Concern:** While `ratelimit.go` exists in the gateway middleware, the auth endpoints (`POST /auth/register`, `POST /auth/login`, `POST /auth/verify`) have no documented rate-limiting strategy.

**Risk:** Credential stuffing, brute-force attacks, and account enumeration.

**Recommendation:**
- Apply aggressive rate limits on `/auth/login` (e.g., 5 attempts/minute per IP, 10/hour per account).
- Rate limit `/auth/register` to prevent mass account creation.
- Implement exponential backoff or account lockout after repeated failures.
- Use separate, stricter rate-limit tiers for auth vs. general API routes.

---

### 3. WebSocket Stream Authentication Gap

**Location:** `WS /stream/:vm_id`  
**Concern:** WebSocket connections are listed as a top-level endpoint with no documented auth flow. WebSocket upgrades bypass standard HTTP middleware in many frameworks.

**Risk:** Unauthorized users could connect to and view/control another user's VM stream.

**Recommendation:**
- Require a short-lived, signed token in the WebSocket handshake (query param or first message).
- Validate VM ownership (`vm.user_id == request.user_id`) before upgrading the connection.
- Implement connection-level heartbeat with session validation.
- Drop idle connections and re-validate tokens periodically.

---

### 4. Host Agent Registration Trust Model

**Location:** `host-agent/internal/agent/register.go`, `host-agent/internal/security/`  
**Concern:** The host agent registers with the gateway, but the trust establishment between donated hardware and the platform is not clearly defined. The `sign.go`/`verify.go` files suggest mutual authentication, but the registration flow itself could be a vector.

**Risk:** Malicious actors could register rogue hosts that serve compromised VM images, intercept user data, or launch attacks from within the network.

**Recommendation:**
- Require manual admin approval for new host registrations (or a verified invite code).
- Use mutual TLS (mTLS) between host agents and the gateway.
- Implement host attestation (verify hardware/software integrity at registration).
- Hosts should never receive user credentials — only opaque session tokens.

---

## High-Priority Issues

### 5. SQL Injection Surface in Scheduler

**Location:** `apps/scheduler/internal/database/queries.go`  
**Concern:** A dedicated `queries.go` file suggests hand-written SQL. Without parameterized queries or an ORM, this is a prime injection target — especially since scheduler queries likely filter by user-supplied VM/queue IDs.

**Risk:** Data exfiltration, privilege escalation, data corruption.

**Recommendation:**
- Use parameterized queries exclusively (`$1`, `$2` placeholders in Go's `database/sql`).
- Consider using `sqlc` or `pgx` with prepared statements for type-safe query generation.
- Never interpolate user input into SQL strings.
- Add SQL injection tests to CI.

---

### 6. File Service Upload Vulnerabilities

**Location:** `apps/file-service/internal/handler/upload.go`  
**Concern:** File upload without documented validation is a classic attack vector.

**Risk:** Path traversal, malicious file execution, storage exhaustion, serving malware to other users.

**Recommendation:**
- Validate and sanitize filenames (strip `../`, null bytes, special chars).
- Enforce file size limits per user and per request.
- Validate MIME types server-side (don't trust `Content-Type` header).
- Store files with random UUIDs, not user-supplied names.
- Scan uploads with antivirus/malware detection.
- Serve downloads with `Content-Disposition: attachment` and appropriate CSP headers.

---

### 7. Credit/Billing Race Conditions

**Location:** `apps/billing-service/src/services/creditService.ts`  
**Concern:** Credit purchase and spending are separate operations. Without transactional guarantees, concurrent requests could double-spend or overdraw credits.

**Risk:** Users could exploit race conditions to get free compute time, causing financial loss.

**Recommendation:**
- Use database-level transactions with row-level locking for credit operations.
- Implement optimistic locking with version fields on the `credits` column.
- Validate credit balance atomically with deduction (single `UPDATE ... WHERE credits >= cost`).
- Add idempotency keys to payment endpoints to prevent double-charging.

---

### 8. Missing Input Validation Architecture

**Location:** `apps/auth-service/src/middleware/validation.ts`  
**Concern:** Only the auth service lists a `validation.ts` middleware. Other services (gateway, scheduler, billing, file-service) have no documented input validation layer.

**Risk:** Injection attacks, buffer overflows, type confusion, denial of service via malformed payloads.

**Recommendation:**
- Implement validation middleware in every service (not just auth).
- Use schema validation libraries (Zod/Joi for TypeScript, go-playground/validator for Go).
- Validate all path parameters, query strings, headers, and request bodies.
- Reject requests with unexpected fields (strict mode).

---

## Medium-Priority Issues

### 9. CORS Policy Undefined

**Location:** `apps/gateway/internal/middleware/cors.go`  
**Concern:** The file exists but the policy is not documented. Overly permissive CORS (e.g., `Access-Control-Allow-Origin: *` with credentials) is a common misconfiguration.

**Risk:** Cross-site request forgery, credential theft via malicious sites.

**Recommendation:**
- Whitelist specific allowed origins (the frontend domain only).
- Never use `*` with `Access-Control-Allow-Credentials: true`.
- Restrict allowed methods and headers to what's actually needed.
- Set appropriate `Access-Control-Max-Age` to reduce preflight overhead.

---

### 10. JWT Configuration Risks

**Location:** `apps/auth-service/src/utils/jwt.ts`  
**Concern:** JWT implementation details (algorithm, secret management, expiration) are not specified.

**Risk:** Algorithm confusion attacks (`alg: none`), weak secrets, overly long token lifetimes, no revocation mechanism.

**Recommendation:**
- Use RS256 (asymmetric) to separate signing from verification.
- Set short access token lifetimes (15 minutes) with refresh token rotation.
- Implement a token revocation list (Redis-backed) for logout/compromise.
- Validate `alg` header server-side; reject `none` and `HS*` if using RS256.
- Store signing keys in a secrets manager, not in code or env files.

---

### 11. WebRTC Security Considerations

**Location:** `desktop/src/system/api/websocket.ts`  
**Concern:** WebRTC streams carry the full desktop — including potentially sensitive content (passwords being typed, private documents).

**Risk:** Stream interception, unauthorized recording, ICE candidate leakage exposing internal IPs.

**Recommendation:**
- Enforce DTLS-SRTP encryption (mandatory in WebRTC spec, but verify configuration).
- Use TURN servers over TLS for relay (don't expose direct peer connections).
- Implement stream watermarking for forensic tracing.
- Terminate streams immediately on session invalidation.
- Filter ICE candidates to prevent local IP leakage.

---

### 12. Password Handling

**Location:** `apps/auth-service/src/utils/password.ts`, DB schema (`password_hash` field)  
**Concern:** Password hashing algorithm and configuration not specified.

**Risk:** Weak hashing (MD5/SHA-256 without salt) enables offline cracking.

**Recommendation:**
- Use Argon2id (preferred) or bcrypt with cost factor ≥ 12.
- Never store plaintext or reversibly encrypted passwords.
- Implement password complexity requirements (min 8 chars, check against breached password lists).
- Add pepper (application-level secret) in addition to per-user salt.

---

### 13. Missing Audit Logging

**Concern:** No audit/logging service appears in the architecture.

**Risk:** Cannot detect, investigate, or attribute security incidents.

**Recommendation:**
- Log all authentication events (login, logout, failed attempts, password changes).
- Log all admin actions with actor identity and timestamp.
- Log VM lifecycle events (create, connect, destroy).
- Ship logs to a tamper-resistant store (separate from application DB).
- Implement alerting on suspicious patterns.

---

### 14. Secrets Management

**Concern:** The architecture mentions config files (`config/config.go`, service configs) but no secrets management strategy.

**Risk:** Hardcoded secrets in source code, secrets in environment variables accessible to all processes.

**Recommendation:**
- Use a dedicated secrets manager (HashiCorp Vault, AWS Secrets Manager, or Kubernetes Secrets with encryption at rest).
- Never commit secrets to git (enforce with pre-commit hooks + `.gitignore`).
- Rotate secrets automatically on a schedule.
- Use separate secrets per environment (dev/staging/prod).

---

## Summary Table

| # | Issue | Severity | Category |
|---|-------|----------|----------|
| 1 | Admin endpoints no RBAC | Critical | Auth |
| 2 | No auth rate limiting | Critical | Auth |
| 3 | WebSocket auth gap | Critical | Auth |
| 4 | Host registration trust | Critical | Infrastructure |
| 5 | SQL injection surface | High | Injection |
| 6 | File upload vulnerabilities | High | Input Validation |
| 7 | Credit race conditions | High | Logic |
| 8 | Missing input validation | High | Input Validation |
| 9 | CORS undefined | Medium | Configuration |
| 10 | JWT risks | Medium | Auth |
| 11 | WebRTC security | Medium | Encryption |
| 12 | Password handling | Medium | Auth |
| 13 | No audit logging | Medium | Observability |
| 14 | No secrets management | Medium | Configuration |
