# WebOS Cloud Drive Design

## 1. Goal & Requirements

- **Per-user drive**: Every authenticated user gets a personal file store ("My Drive") isolated by userId.
- **100 GB cap**: Per-user storage is hard-capped at 100 GB (107,374,182,400 bytes). The cap is enforced server-side on every upload.
- **Cloud VM backend**: File data lives on configured cloud VMs rather than only on the local gateway disk. The backend must be pluggable (local FS for dev, remote VM agent or S3-compatible store for production).

## 2. Current State & Gap

### What exists today

| Component | Location | Behavior |
|-----------|----------|----------|
| Storage manager | `apps/gateway/internal/storage/storage.go:34` | `StorageManager` with `basePath`, in-memory `files` map, `quotaFn` / `usageFn` hooks. |
| Endpoints | `apps/gateway/internal/tunnel/server.go:327-330` | `/storage/list`, `/storage/upload`, `/storage/download`, `/storage/delete`. All wrapped in `storageAuth` which injects `userId` from the authenticated session. |
| Quota enforcement | `apps/gateway/internal/tunnel/server.go:212-217` | `SetQuotaCheck(authMgr.CheckStorageQuota)` and `SetUsageFunc(authMgr.AddStorageUsed)` are wired in at startup. |
| Auth quota | `apps/gateway/internal/auth/auth.go:267-289` | `CheckStorageQuota(userID, additionalBytes)` returns `ErrQuotaExceeded` if `StorageUsed + additionalBytes > StorageQuota`. `AddStorageUsed` increments the counter. Default quota = 100 GB (`100 * 1024 * 1024 * 1024`). |
| Frontend UI | `apps/frontend/app/webos/apps/files/Files.tsx` | Minimal file browser: list, upload, delete, download. Shows usage bar. Hard-codes `quota.total = 107374182400`. |
| Startup comment | `run-backend.sh:219` | Prints `Storage: 100GB per user (local: /tmp/freecompute-storage/)`. |

### GAP

1. **Local-only backend**: `StorageManager` writes directly to `os.Create(fullPath)` on the local filesystem. No abstraction to swap in a remote VM or object-store backend.
2. **No persistence of file index**: The `files` map is purely in-memory; a gateway restart loses all metadata.
3. **No pluggable backend selector**: The backend is hard-coded to local FS at server startup (`storage.NewStorageManager(logger, "/tmp/freecompute-storage")`).
4. **Frontend is basic**: The existing `Files.tsx` is functional but lacks folder creation, breadcrumb navigation, and quota-error UX.

## 3. Storage Backend Abstraction

Define a `Backend` interface in a new file `apps/gateway/internal/storage/backend.go`:

```go
// Backend abstracts where file bytes and metadata live.
// Implementations: LocalFS, RemoteVM, S3, etc.
type Backend interface {
    // List returns FileInfo entries for userID under dirPath (empty string = root).
    List(userID, dirPath string) ([]*FileInfo, error)

    // Write stores reader contents as userID/filePath with the given size/mime.
    // It must return ErrQuotaExceeded if the backend's own quota check fails.
    Write(userID, filePath string, reader io.Reader, size int64, mimeType string) (*FileInfo, error)

    // Read returns a reader for userID/filePath.
    Read(userID, filePath string) (io.ReadCloser, *FileInfo, error)

    // Delete removes userID/filePath.
    Delete(userID, filePath string) error
}
```

### LocalFS (default, replaces current logic)

Move the existing local-FS code from `storage.go` into `local.go` implementing `Backend`:
- Prefix all paths with `<basePath>/<userID>/`.
- `os.MkdirAll` before writes.
- Keep the in-memory `files` map in a separate `index` layer so it survives restarts (see §6).

### RemoteVM (target)

A backend that tunnels HTTP calls to a "storage agent" running on a configured cloud VM:

| Method | Protocol |
|--------|----------|
| `List` | `GET /storage/list?userId=...&path=...` |
| `Write` | `POST /storage/write?userId=...&path=...&size=...&mime=...` (body = bytes) |
| `Read` | `GET /storage/read?userId=...&path=...` |
| `Delete` | `DELETE /storage/delete?userId=...&path=...` |

Gateway acts as a reverse proxy / client to the remote VM agent. The VM address is configurable via env var (see §8).

## 4. Per-User Namespace + 100 GB Enforcement

### Namespace

Every backend call is scoped by `userID`. The local FS backend joins `basePath/userID/...`. The RemoteVM backend passes `userId` as a query param. This guarantees isolation: user A cannot list, read, write, or delete user B's files.

### Quota flow (already implemented, no change needed)

1. `StorageHandler.Upload` calls `StorageManager.WriteFile` (`storage.go:78`).
2. `WriteFile` invokes `s.quotaFn(userID, size)` before writing.
3. `quotaFn` is set to `authMgr.CheckStorageQuota` (`server.go:212`), which checks `user.StorageUsed + size > user.StorageQuota` (100 GB) and returns `ErrQuotaExceeded`.
4. If quota is OK, bytes are written and `s.usageFn(userID, written)` is called, which invokes `authMgr.AddStorageUsed` to increment the counter.

**No changes are required to enforce the 100 GB cap** — the hooks are already wired. The only addition needed: the new `RemoteVM` backend should also surface quota errors (e.g., the remote agent returns `507 Insufficient Storage`, which the gateway maps to `ErrQuotaExceeded`).

### Metadata persistence gap

The `files` map is lost on restart. A simple fix: serialize to JSON on shutdown and reload on startup, or use the existing `database` package. This is orthogonal to the backend abstraction but required for production durability.

## 5. WebOS Integration

### Frontend entry point

`apps/frontend/app/webos/apps/files/Files.tsx` already exists and is wired into the WebOS desktop as the "Files" app. It calls `/storage/list`, `/storage/upload`, `/storage/download`, `/storage/delete`.

### Enhancements needed

1. **Quota error UX**: Parse `507` responses on upload and show `"Drive full — 100 GB limit reached"`.
2. **Folder navigation**: Add breadcrumb trail and `mkdir` support (new `/storage/mkdir` endpoint or reuse `Upload` with empty body + `isDir` flag).
3. **Download button**: Wire the existing `Download` handler to a visible download action.
4. **Settings display**: `apps/frontend/app/webos/apps/settings/Settings.tsx:32` already shows `storageUsed / storageQuota` — keep this.

## 6. Implementation Steps

| Step | File | Action |
|------|------|--------|
| 1 | `apps/gateway/internal/storage/backend.go` | New file. Define `Backend` interface and `ErrQuotaExceeded` constant. |
| 2 | `apps/gateway/internal/storage/local.go` | New file. Implement `LocalFSBackend` with current FS logic. Extract from `storage.go`. |
| 3 | `apps/gateway/internal/storage/storage.go` | Refactor `StorageManager` to hold a `Backend` instead of `basePath` + raw `os` calls. Keep `SetQuotaCheck` / `SetUsageFunc` and the `files` index. |
| 4 | `apps/gateway/internal/storage/remote.go` | New file. Implement `RemoteVMBackend` that proxies to a configurable VM agent URL. |
| 5 | `apps/gateway/internal/storage/index.go` | New file. JSON-serialized file index for persistence across restarts. |
| 6 | `apps/gateway/internal/tunnel/server.go:134` | Replace `storage.NewStorageManager(logger, "/tmp/freecompute-storage")` with a factory that reads `FREECOMPUTE_STORAGE_BACKEND` and returns the correct `Backend`. |
| 7 | `apps/gateway/internal/tunnel/server.go:314-330` | Add `/storage/mkdir` endpoint (optional, for folder creation). |
| 8 | `run-backend.sh:219` | Update echo to reflect new env-driven backend. |
| 9 | `apps/frontend/app/webos/apps/files/Files.tsx` | Add quota-error toast, folder breadcrumbs, mkdir button. |

## 7. Acceptance Criteria

- [ ] Upload a 1 KB file as user A → visible under `/storage/list?userId=A`.
- [ ] Upload the same file as user B → NOT visible under user A's list.
- [ ] Upload until total reaches 100 GB → subsequent upload returns `507 Insufficient Storage` with `{"error":"storage quota exceeded"}`.
- [ ] Gateway restart with `FREECOMPUTE_STORAGE_BACKEND=local` → previously uploaded files still list and download (index persisted).
- [ ] Switch `FREECOMPUTE_STORAGE_BACKEND=remote` and point to a running VM agent → uploads land on the VM, not local `/tmp`.

## 8. Configuration

```bash
# Select backend: "local" (default) or "remote"
FREECOMPUTE_STORAGE_BACKEND=local

# Local root (default: /tmp/freecompute-storage)
FREECOMPUTE_STORAGE_ROOT=/tmp/freecompute-storage

# Remote VM agent URL (used when backend=remote)
FREECOMPUTE_STORAGE_VM_ADDR=http://10.0.0.5:9090
```

### Updated run-backend.sh snippet

```bash
echo "Storage: 100GB per user (backend=${FREECOMPUTE_STORAGE_BACKEND:-local}, root=${FREECOMPUTE_STORAGE_ROOT:-/tmp/freecompute-storage})"
```
