# Session Persistence & State Recovery

## Overview

Session persistence enables cloud saves, session migration between hosts, and seamless recovery from network interruptions. Users never lose work — even when switching devices or networks.

## Architecture

```
Session State
    │
    ├── Ephemeral (in-memory)
    │   ├── Input state (keyboard/mouse/gamepad)
    │   ├── Clipboard content
    │   ├── Audio pipeline state
    │   └── Connection quality history
    │
    └── Persistent (Redis + S3)
        ├── Desktop layout (window positions, open apps)
        ├── Open files and editor state
        ├── Terminal history and scrollback
        ├── Browser tabs and navigation state
        ├── GPU frame buffer checkpoint
        └── SSH connection state
```

## Checkpoint System

### Periodic Snapshots

The gateway creates session snapshots at configurable intervals:

```go
type SessionCheckpoint struct {
    SessionID    string    `json:"sessionId"`
    Timestamp    time.Time `json:"timestamp"`
    Delta        bool      `json:"delta"` // full vs incremental
    DesktopState []byte    `json:"desktopState,omitempty"`
    Clipboard    string    `json:"clipboard,omitempty"`
    InputState   InputState `json:"inputState,omitempty"`
    Checksum     string    `json:"checksum"`
}
```

- **Full snapshot**: Every 60s (configurable via `FREECOMPUTE_SESSION_SNAPSHOT_INTERVAL`)
- **Delta snapshot**: Every 5s (only changed state)
- **On-demand snapshot**: Triggered by suspend/hibernate/sigterm

### Storage

| Tier | Store | Latency | Retention |
|------|-------|---------|-----------|
| Hot | Redis | <5ms | 5 minutes |
| Warm | Local SSD | <20ms | 1 hour |
| Cold | S3/R2 | <200ms | 30 days |

## Reconnection Recovery Flow

```
Client reconnects
    │
    ▼
GET /sessions/{id}/checkpoint/latest
    │
    ├── Found → Resume from checkpoint
    │   ├── Restore desktop state (windows, cursor pos)
    │   ├── Restart media pipeline (codec, resolution)
    │   ├── Replay buffered input events
    │   └── Set new ICE/STUN candidates
    │
    └── Not found → Start fresh session with same ID
```

## Frontend Recovery Hook

```typescript
function useSessionPersistence(sessionId: string) {
  // On mount: check for stored checkpoint
  // On visibility change: trigger checkpoint
  // On beforeunload: create final checkpoint
  // On reconnect: resume from latest checkpoint
}
```

## Cloud Save API

```
GET  /sessions/{id}/saves           → list saves
POST /sessions/{id}/saves           → create save
POST /sessions/{id}/saves/{saveId}/restore → restore save
DELETE /sessions/{id}/saves/{saveId} → delete save
```

## Gaming Save Sync

For gaming sessions, save data is synced automatically:

1. Game writes save file → monitored by host-agent
2. Host-agent detects file change → uploads delta to gateway
3. Gateway stores in S3/R2
4. On session reconnect: latest save is pre-staged before desktop starts
5. Steam Cloud / platform saves are also mirrored

## Implementation Phases

1. **Phase 1** — Ephemeral in-memory session state (current)
2. **Phase 2** — Redis-backed checkpoint with 5s deltas
3. **Phase 3** — S3/R2 cold storage for cross-session persistence
4. **Phase 4** — Cloud save API + gaming save sync
