# File Service as Cloud-VM Storage Backend

How `apps/file-service` running on a configured cloud VM becomes the storage backend for WebOS user drives through the universal proxy and agent tunnel.

---

## 1. Goal & Requirements

- **Pluggable storage backend**: User drive files live on a file-service instance running on a cloud VM, reachable through the gateway's universal proxy / agent tunnel, rather than only on the local gateway disk.
- **100 GB per-user quota**: Hard-capped at 107,374,182,400 bytes, enforced server-side on every upload.
- **Consistency & replication**: The file-service is the source of truth for drive bytes; the gateway StorageManager caches metadata and coordinates access.
- **Do not duplicate** `docs/WEBOS_CLOUD_DRIVE.md`; this doc focuses on the cloud-VM deployment path and agent-tunnel integration.

---

## 2. Current State

### File service (`apps/file-service`)

Not a stub. It is a standalone HTTP service with a pluggable `Storage` interface:

| Component | File | Behavior |
|-----------|------|----------|
| Main | `apps/file-service/cmd/file-service/main.go` | Listens on `FREECOMPUTE_FILESERVICE_ADDR` (default `:8082`); mounts `/api/files`, `/api/files/upload`, `/api/files/download/`, `/api/health` |
| Handler | `apps/file-service/internal/handler/handler.go` | Bearer-token auth, upload/download/list/delete/info with pagination |
| Storage interface | `apps/file-service/internal/storage/storage.go:17-23` | `Upload`, `Download`, `Delete`, `List`, `Info` |
| Local backend | `apps/file-service/internal/storage/storage.go:25-176` | Writes to `<basePath>/<userID>/...`; in-memory index with SHA-256 checksums |
| S3 backend (stub) | `apps/file-service/internal/storage/storage.go:178-235` | `NewS3Storage` exists; `Download`/`Delete`/`Info` return "not yet implemented" |
| Config | `apps/file-service/internal/config/config.go` | Env-driven: `FREECOMPUTE_FILESERVICE_STORAGE` (`local` or `s3`), `FREECOMPUTE_FILESERVICE_BASE_PATH`, S3 credentials, auth token |
| Models | `apps/file-service/internal/models/models.go` | `FileInfo` with ID, path, size, MIME, userID, checksum, timestamps |

### Gateway storage (`apps/gateway/internal/storage`)

| Component | File | Behavior |
|-----------|------|----------|
| `StorageManager` | `apps/gateway/internal/storage/storage.go:34` | Local-only backend writing to `basePath`; in-memory `files` map; `quotaFn` / `usageFn` hooks |
| Endpoints | `apps/gateway/internal/tunnel/server.go:327-330` | `/storage/list`, `/storage/upload`, `/storage/download`, `/storage/delete` wrapped in `storageAuth` |

### Agent tunnel mechanism

| Component | File | Behavior |
|-----------|------|----------|
| Route config | `apps/gateway/internal/tunnel/config.go:245-247` | `UsesAgentTunnel()` is true when `Protocol` is TCP/SSH and `Target == "agent"` |
| Agent pool | `apps/gateway/internal/tunnel/agent_pool.go` | Maintains persistent CONNECT tunnels to host agents; routes requests to the correct agent slot |
| Tailscale direct dial | `apps/gateway/internal/tunnel/server.go:1238-1289` | `dialViaTailscale` races TCP connects to all known tailscale hosts; used when route does **not** use agent tunnel |

### Existing quota enforcement

`apps/gateway/internal/auth/auth.go:267-289` (`CheckStorageQuota` / `AddStorageUsed`) enforces the 100 GB cap. `StorageManager.SetQuotaCheck` and `SetUsageFunc` are wired at gateway startup (`server.go:212-217`). No changes are needed to the quota logic itself.

---

## 3. Cloud-VM Architecture

### Deployment layout

```
Cloud VM
├── file-service (listening on localhost:8082)
│   └── LocalStorage or S3 backend
└── host-agent
    └── Route: { id: "file-service", target: "localhost:8082", protocol: "tcp" }

Gateway
├── Route: { id: "file-service", target: "agent", protocol: "tcp" }
├── StorageManager (RemoteVMBackend)
│   └── Proxies /storage/* through agent pool to file-service
└── StorageHandler
    └── /storage/list, /storage/upload, /storage/download, /storage/delete
```

### Data path

1. WebOS frontend calls `/storage/upload?userId=A&path=doc.pdf`.
2. `StorageHandler.Upload` invokes `StorageManager.WriteFile`.
3. `RemoteVMBackend.Write` opens an HTTP request through the gateway's agent pool to the file-service `/api/files/upload?userId=A&path=doc.pdf`.
4. The host agent bridges the gateway CONNECT tunnel to `localhost:8082`.
5. The file-service writes bytes to its local disk or S3 and returns `FileInfo`.
6. Gateway records usage via `usageFn` (`AddStorageUsed`); quota is checked before step 3.

### Why reuse the agent pool

The agent pool (`agent_pool.go`) already manages persistent, authenticated tunnels to host agents. Reusing it avoids opening new outbound connections per request and gives us:
- Connection pooling and backpressure.
- Existing reconnect/backoff logic (`computeBackoff` in `host-agent/cmd/host-agent/main.go:198-205`).
- Tailscale direct-dial fallback (`dialViaTailscale` in `apps/gateway/internal/tunnel/server.go:1238`) when the agent tunnel is unavailable.

---

## 4. Backend Abstraction

`docs/WEBOS_CLOUD_DRIVE.md` §3 defines the `Backend` interface (`List`, `Write`, `Read`, `Delete`). The file-service already implements an equivalent `Storage` interface. The integration work is:

1. **Gateway side**: Add a `RemoteVMBackend` in `apps/gateway/internal/storage/remote.go` that translates `Backend` calls into HTTP requests forwarded through the agent pool to the file-service.
2. **File-service side**: Add a `RemoteVMServer` handler in `apps/file-service/internal/handler/remote.go` that accepts proxied requests from the gateway, validates the gateway's service token (not the end-user token), and forwards to the underlying `Storage` implementation with the user's `userId` from the query string.

The file-service continues to own:
- Byte storage and retrieval.
- Per-user directory layout (`<basePath>/<userID>/`).
- SHA-256 checksums.
- Its own local/S3 backend selection.

The gateway continues to own:
- End-user authentication and `userId` injection (`storageAuth` in `server.go:314-326`).
- 100 GB quota enforcement via `quotaFn` / `usageFn`.
- Session-scoped routing and the agent pool.

---

## 5. Consistency & Replication

### Single-writer, multi-reader

The file-service on the VM is the sole writer for its backend (local disk or S3). The gateway's in-memory `files` index is a cache only; it is rebuilt on restart by listing the file-service (or re-initialized empty for local mode).

### Sync model

- **Write path**: Gateway → file-service (synchronous). If the file-service returns an error, the gateway surfaces it to the client. No eventual-consistency window for writes.
- **Read path**: Gateway → file-service (synchronous per request). No stale reads.
- **List path**: Gateway → file-service (synchronous). The file-service's `List` already walks the local directory or queries the index.
- **Failover**: If the VM's file-service is unreachable, the agent pool returns an error; the gateway returns `502 Bad Gateway`. No automatic failover to a second VM is planned in this design — that requires a multi-backend `StorageManager` (future work).

### Replication (future)

If multi-region redundancy is required, the file-service's S3 backend provides object-level replication. For local-disk backends, the host-agent or a sidecar can rsync/snapshot to a secondary VM, but this is outside the current scope.

---

## 6. Implementation Steps

| Step | File(s) | Action |
|------|---------|--------|
| 1 | `apps/file-service/internal/handler/remote.go` | New handler that accepts gateway-proxied requests, validates service token, and delegates to `Storage` with `userId` from query |
| 2 | `apps/file-service/internal/config/config.go` | Add `ServiceToken` and `ProxyAuthOnly` fields; when `ProxyAuthOnly=true`, skip end-user Bearer auth and trust the gateway's service token |
| 3 | `apps/gateway/internal/storage/remote.go` | New `RemoteVMBackend` implementing `Backend`; uses the gateway's agent pool to send HTTP requests to the file-service |
| 4 | `apps/gateway/internal/storage/storage.go` | Refactor `StorageManager` to hold a `Backend` interface; add `NewStorageManagerWithBackend(backend Backend, quotaFn, usageFn)` constructor |
| 5 | `apps/gateway/internal/tunnel/server.go:134` | Add a route entry for `file-service` with `Target: "agent"` and `Protocol: ProtocolTCP` so the agent pool bridges traffic |
| 6 | `run-backend.sh` | Add env vars: `FREECOMPUTE_STORAGE_BACKEND=remote`, `FREECOMPUTE_STORAGE_VM_ADDR` (route ID), `FREECOMPUTE_FILESERVICE_SERVICE_TOKEN` |
| 7 | `apps/gateway/internal/storage/storage.go` | Wire `RemoteVMBackend` when `FREECOMPUTE_STORAGE_BACKEND=remote`; keep local backend as default |
| 8 | `apps/frontend/app/webos/apps/files/Files.tsx` | Add `502` error UX for VM-unavailable state; reuse existing `507` quota-error handling |

---

## 7. Acceptance Criteria

- [ ] Upload a 1 KB file as user A through the WebOS Files app → file lands on the cloud VM's local disk (or S3), not on the gateway's `/tmp/freecompute-storage`.
- [ ] List files as user A → returns only user A's files (userId isolation preserved through the proxy).
- [ ] Upload until total reaches 100 GB → subsequent upload returns `507 Insufficient Storage` with `{"error":"storage quota exceeded"}` (quota enforced by gateway before proxying).
- [ ] Stop the file-service process on the VM → next upload returns `502 Bad Gateway` from the gateway; restarting the file-service restores normal operation.
- [ ] Gateway restart with `FREECOMPUTE_STORAGE_BACKEND=remote` → file-service is reachable through the existing agent tunnel; no new tunnel setup required.
