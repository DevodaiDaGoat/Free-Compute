# Security Guidelines

This document defines security requirements and patterns for the FreeCompute platform. All contributors must follow these guidelines when implementing services.

---

## Table of Contents

1. [Authentication & Authorization](#authentication--authorization)
2. [Input Validation](#input-validation)
3. [Database Security](#database-security)
4. [API Security](#api-security)
5. [File Handling](#file-handling)
6. [Secrets Management](#secrets-management)
7. [WebSocket & Streaming Security](#websocket--streaming-security)
8. [Host Agent Security](#host-agent-security)
9. [Dependency Management](#dependency-management)
10. [Logging & Monitoring](#logging--monitoring)
11. [Development Practices](#development-practices)

---

## Authentication & Authorization

### Password Storage

```typescript
// REQUIRED: Use Argon2id for password hashing
import { hash, verify } from '@node-rs/argon2';

const ARGON2_OPTIONS = {
  memoryCost: 65536,  // 64 MB
  timeCost: 3,
  parallelism: 4,
};

async function hashPassword(password: string): Promise<string> {
  return hash(password, ARGON2_OPTIONS);
}

async function verifyPassword(hash: string, password: string): Promise<boolean> {
  return verify(hash, password);
}
```

**Requirements:**
- Use Argon2id (preferred) or bcrypt with cost ≥ 12.
- Enforce minimum 8 characters; check against [Have I Been Pwned](https://haveibeenpwned.com/API/v3) breached password list.
- Never log, return, or store plaintext passwords.

### JWT Configuration

```typescript
// REQUIRED: Use asymmetric signing
const JWT_CONFIG = {
  algorithm: 'RS256',           // Asymmetric — never use 'none' or 'HS256' in production
  accessTokenExpiry: '15m',     // Short-lived access tokens
  refreshTokenExpiry: '7d',     // Refresh tokens with rotation
  issuer: 'freecompute.io',
  audience: 'freecompute-api',
};
```

**Requirements:**
- Validate `alg` header server-side; reject `none` and unexpected algorithms.
- Implement refresh token rotation (invalidate old refresh token on use).
- Maintain a Redis-backed revocation list for logout and compromise.
- Include minimal claims: `sub`, `iat`, `exp`, `role`. Never include passwords or secrets.

### Rate Limiting on Auth Routes

```go
// REQUIRED: Separate rate limit tiers
var authLimiter = middleware.RateLimit{
    Login:    rate.Every(12*time.Second), // 5/min per IP
    Register: rate.Every(30*time.Second), // 2/min per IP
    Verify:   rate.Every(10*time.Second), // 6/min per IP
}
```

**Requirements:**
- Rate limit by IP and by account (dual key).
- Implement exponential backoff after 5 failed login attempts.
- Lock accounts after 10 consecutive failures (require email verification to unlock).
- Return consistent error messages to prevent account enumeration (e.g., always "Invalid credentials").

### Role-Based Access Control (RBAC)

```go
// REQUIRED: Admin routes must use adminAuth middleware
func SetupAdminRoutes(r *mux.Router) {
    admin := r.PathPrefix("/admin").Subrouter()
    admin.Use(middleware.RequireAuth)
    admin.Use(middleware.RequireRole("admin"))
    admin.Use(middleware.AuditLog)
    // ... routes
}
```

**Requirements:**
- All `/admin/*` routes require both authentication and admin role verification.
- Implement principle of least privilege — don't grant admin for convenience.
- Log all admin actions to an audit trail.

---

## Input Validation

### Schema Validation (TypeScript Services)

```typescript
// REQUIRED: Validate all request bodies with Zod
import { z } from 'zod';

const LaunchVMSchema = z.object({
  name: z.string().min(1).max(64).regex(/^[a-zA-Z0-9_-]+$/),
  cpu_cores: z.number().int().min(1).max(16),
  ram_gb: z.number().int().min(1).max(64),
  storage_gb: z.number().int().min(10).max(500),
}).strict(); // Reject unexpected fields

// In route handler:
app.post('/vm/launch', validate(LaunchVMSchema), vmController.launch);
```

### Schema Validation (Go Services)

```go
// REQUIRED: Validate all request bodies
import "github.com/go-playground/validator/v10"

type LaunchVMRequest struct {
    Name      string `json:"name" validate:"required,min=1,max=64,alphanum_hyphen"`
    CPUCores  int    `json:"cpu_cores" validate:"required,min=1,max=16"`
    RAMGB     int    `json:"ram_gb" validate:"required,min=1,max=64"`
    StorageGB int    `json:"storage_gb" validate:"required,min=10,max=500"`
}
```

**Requirements:**
- Validate ALL inputs: path params, query strings, headers, and bodies.
- Use allowlists (not denylists) for acceptable values.
- Reject requests with unexpected fields (strict mode).
- Sanitize strings: strip null bytes, trim whitespace, normalize Unicode.
- Set maximum request body size at the HTTP server level (e.g., 1 MB for JSON, configurable for file uploads).

### Path Parameter Validation

```go
// REQUIRED: Validate UUIDs in path parameters
func validateUUID(param string) error {
    if _, err := uuid.Parse(param); err != nil {
        return ErrInvalidID
    }
    return nil
}
```

---

## Database Security

### Parameterized Queries Only

```go
// CORRECT — parameterized query
row := db.QueryRow("SELECT * FROM vms WHERE id = $1 AND user_id = $2", vmID, userID)

// FORBIDDEN — string interpolation
row := db.QueryRow(fmt.Sprintf("SELECT * FROM vms WHERE id = '%s'", vmID)) // SQL INJECTION
```

**Requirements:**
- Use parameterized queries (`$1`, `$2`) exclusively — NEVER interpolate user input.
- Use `sqlc` or a query builder for type-safe, generated queries.
- Apply least-privilege database roles (app user should not have `DROP`/`ALTER` permissions).
- Enable `statement_timeout` to prevent long-running query DoS.
- Use row-level security (RLS) in PostgreSQL where applicable.

### Transaction Safety for Credits

```sql
-- REQUIRED: Atomic credit deduction
UPDATE users
SET credits = credits - $1
WHERE id = $2 AND credits >= $1
RETURNING credits;
-- If 0 rows affected → insufficient credits (no race condition)
```

**Requirements:**
- All credit operations must be atomic (single statement or serializable transaction).
- Use optimistic locking (`version` column) for complex multi-step transactions.
- Add idempotency keys to payment endpoints to prevent double-charging.

---

## API Security

### CORS Configuration

```go
// REQUIRED: Strict CORS policy
cors := middleware.CORSConfig{
    AllowedOrigins:   []string{"https://app.freecompute.io"},  // Explicit origin only
    AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
    AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Request-ID"},
    AllowCredentials: true,
    MaxAge:           3600,
}
// FORBIDDEN: AllowedOrigins: []string{"*"} with credentials
```

**Requirements:**
- Never use wildcard (`*`) origin with credentials.
- Whitelist only the production frontend domain(s).
- Restrict methods and headers to what's actually needed.
- Include `X-Request-ID` for request tracing.

### Response Headers

```go
// REQUIRED: Security headers on all responses
func securityHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("X-XSS-Protection", "0")  // Rely on CSP instead
        w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
        w.Header().Set("Content-Security-Policy", "default-src 'self'")
        w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
        w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
        next.ServeHTTP(w, r)
    })
}
```

### No Debug Endpoints in Production

```go
// FORBIDDEN in production:
// - /debug/pprof/*
// - /debug/vars
// - /healthz with internal details
// - Any endpoint returning stack traces or env vars

// REQUIRED: Gate debug endpoints behind build tags or env checks
if os.Getenv("ENVIRONMENT") == "development" {
    r.HandleFunc("/debug/pprof/", pprof.Index)
}
```

---

## File Handling

### Upload Security

```go
// REQUIRED: File upload validation
const (
    MaxFileSize     = 100 * 1024 * 1024 // 100 MB
    MaxFilenameLen  = 255
)

var AllowedMIMETypes = map[string]bool{
    "image/png":  true,
    "image/jpeg": true,
    "application/pdf": true,
    // ... explicit allowlist
}

func validateUpload(file multipart.File, header *multipart.FileHeader) error {
    // 1. Check file size
    if header.Size > MaxFileSize {
        return ErrFileTooLarge
    }

    // 2. Validate MIME type by reading magic bytes (not Content-Type header)
    buf := make([]byte, 512)
    file.Read(buf)
    mime := http.DetectContentType(buf)
    if !AllowedMIMETypes[mime] {
        return ErrInvalidFileType
    }

    // 3. Sanitize filename — but don't use it for storage
    // Store with UUID; original name in metadata only
    return nil
}
```

**Requirements:**
- Store files with random UUIDs — never use user-supplied filenames as paths.
- Validate MIME type by reading file magic bytes (not trusting the `Content-Type` header).
- Strip path traversal sequences (`../`, `..\\`, null bytes).
- Set per-user storage quotas.
- Serve downloads with `Content-Disposition: attachment`.
- Consider antivirus scanning for shared file storage.

---

## Secrets Management

### Environment & Configuration

```yaml
# FORBIDDEN: Never commit to git
DATABASE_URL=postgres://user:password@host:5432/db
JWT_SECRET=my-secret-key
STRIPE_SECRET_KEY=sk_live_...
```

**Requirements:**
- Use a secrets manager (HashiCorp Vault, AWS Secrets Manager, or sealed Kubernetes Secrets).
- Never commit secrets to git — enforce with pre-commit hooks (see [Development Practices](#development-practices)).
- Use separate secrets per environment (dev/staging/prod).
- Rotate secrets on a schedule (90 days max for database passwords, 30 days for API keys).
- Application config should reference secret paths, not values:
  ```go
  // CORRECT: Load from secrets manager at runtime
  dbURL := vault.GetSecret("database/creds/app-role")
  
  // FORBIDDEN: Hardcoded or plain env var
  dbURL := "postgres://admin:password123@localhost/freecompute"
  ```

### .gitignore Requirements

```gitignore
# Secrets — never commit
.env
.env.*
*.pem
*.key
credentials.json
service-account.json
```

---

## WebSocket & Streaming Security

### Connection Authentication

```typescript
// REQUIRED: Authenticate WebSocket connections
wss.on('connection', async (ws, req) => {
  const token = new URL(req.url, 'http://localhost').searchParams.get('token');

  // 1. Validate token (short-lived, single-use connection token)
  const claims = await verifyConnectionToken(token);
  if (!claims) {
    ws.close(4001, 'Unauthorized');
    return;
  }

  // 2. Verify VM ownership
  const vm = await getVM(claims.vmId);
  if (vm.userId !== claims.sub) {
    ws.close(4003, 'Forbidden');
    return;
  }

  // 3. Invalidate the connection token (single use)
  await invalidateToken(token);

  // 4. Set up periodic re-validation
  const revalidateInterval = setInterval(async () => {
    const sessionValid = await checkSession(claims.sub);
    if (!sessionValid) ws.close(4001, 'Session expired');
  }, 60_000);
});
```

**Requirements:**
- Use short-lived, single-use tokens for WebSocket handshakes (not the main JWT).
- Verify resource ownership before upgrading the connection.
- Implement idle timeouts and periodic session re-validation.
- Drop connections immediately on user logout or session revocation.

### WebRTC Configuration

```typescript
// REQUIRED: Secure ICE configuration
const rtcConfig: RTCConfiguration = {
  iceServers: [
    { urls: 'turn:turn.freecompute.io:443?transport=tcp', username: '...', credential: '...' }
  ],
  iceTransportPolicy: 'relay',  // Force TURN relay — prevents IP leakage
};
```

**Requirements:**
- Use `iceTransportPolicy: 'relay'` to prevent direct peer connections that leak IPs.
- Use TURN over TLS (port 443) for firewall traversal and encryption.
- Generate short-lived TURN credentials per session.
- Enforce DTLS-SRTP (WebRTC default, but verify configuration).

---

## Host Agent Security

### Registration & Trust

**Requirements:**
- New hosts must be approved by an admin before receiving workloads.
- Use mutual TLS (mTLS) between host agents and the gateway.
- Host agents must prove identity with hardware-bound certificates.
- Hosts should never receive user credentials — only opaque, scoped session tokens.
- Implement runtime attestation: verify the host is running expected software versions.

### Isolation

**Requirements:**
- Each VM must run in a separate cgroup and network namespace.
- Hosts must not be able to access VM memory/disk outside of QEMU's sandbox.
- Disable host-to-VM clipboard sharing by default.
- Network traffic between VMs on the same host must be isolated (separate bridge interfaces).
- Implement egress filtering: VMs should not be able to reach the host's local network or other VMs.

---

## Dependency Management

### Requirements

- Pin all dependencies to exact versions (lockfiles committed).
- Run `npm audit` / `go mod verify` / `govulncheck` in CI — fail on high/critical vulnerabilities.
- When adding a new dependency:
  - Prefer packages published ≥ 7 days ago.
  - Check maintenance status (last commit, open issues, bus factor).
  - Verify the package does what it claims (check source, not just README).
- Update dependencies at least monthly (automated via Dependabot or Renovate).
- Never use floating version ranges (`*`, `latest`, unbounded `>=`).

### CI Integration

```yaml
# REQUIRED: Security checks in CI pipeline
security:
  steps:
    - name: Dependency Audit
      run: npm audit --audit-level=high && govulncheck ./...

    - name: Secret Scanning
      uses: trufflesecurity/trufflehog@v3
      with:
        path: .

    - name: SAST
      uses: github/codeql-action/analyze@v3
```

---

## Logging & Monitoring

### What to Log

| Event | Required Fields |
|-------|----------------|
| Login success/failure | `user_id`, `ip`, `user_agent`, `timestamp` |
| Password change | `user_id`, `ip`, `timestamp` |
| Admin action | `admin_id`, `action`, `target`, `timestamp` |
| VM lifecycle | `user_id`, `vm_id`, `action`, `host_id`, `timestamp` |
| Credit transaction | `user_id`, `amount`, `type`, `balance_after`, `timestamp` |
| Rate limit triggered | `ip`, `endpoint`, `count`, `timestamp` |
| Auth token revoked | `user_id`, `reason`, `timestamp` |

### What NEVER to Log

- Passwords (plaintext or hashed)
- Full credit card numbers
- JWT tokens or session secrets
- Personal data beyond what's needed for debugging

### Alerting

Set up alerts for:
- More than 10 failed logins from a single IP in 5 minutes.
- Any admin action from an unrecognized IP.
- Host agent registration from unknown hardware.
- Credit balance going negative (indicates a race condition bug).
- Unusual egress traffic from VMs.

---

## Development Practices

### Pre-Commit Hooks

```yaml
# .pre-commit-config.yaml
repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.5.0
    hooks:
      - id: detect-private-key
      - id: check-added-large-files
        args: ['--maxkb=500']

  - repo: https://github.com/zricethezav/gitleaks
    rev: v8.18.0
    hooks:
      - id: gitleaks
```

### Code Review Checklist (Security)

Before approving any PR, verify:
- [ ] No hardcoded secrets, keys, or credentials.
- [ ] All user input is validated and sanitized.
- [ ] Database queries are parameterized.
- [ ] New endpoints have appropriate auth middleware.
- [ ] Error responses don't leak internal details (stack traces, DB errors).
- [ ] New dependencies are audited and version-pinned.
- [ ] File operations use safe paths (no user-controlled path components).
- [ ] Logging doesn't include sensitive data.

### Error Handling

```go
// CORRECT: Generic error to client, detailed error in logs
func handleError(w http.ResponseWriter, err error, statusCode int) {
    log.Error("request failed", "error", err, "stack", debug.Stack())
    http.Error(w, http.StatusText(statusCode), statusCode)
}

// FORBIDDEN: Exposing internals
http.Error(w, fmt.Sprintf("DB error: %v", err), 500)  // Leaks DB details
http.Error(w, string(debug.Stack()), 500)              // Leaks code paths
```

---

## Incident Response

1. **Detection** — Automated alerts + manual monitoring.
2. **Containment** — Revoke compromised tokens, isolate affected hosts, disable affected accounts.
3. **Investigation** — Use audit logs to determine scope and root cause.
4. **Recovery** — Rotate affected secrets, patch vulnerability, restore from backup if needed.
5. **Post-mortem** — Document timeline, root cause, and preventive measures.

---

## Contact

Report security vulnerabilities via GitHub Security Advisories (private disclosure) or email the maintainers directly. Do not open public issues for security bugs.
