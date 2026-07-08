#!/usr/bin/env bash
set -euo pipefail

ROOT="/workspaces/Free-Compute"
DOCS="$ROOT/docs"
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

log() { echo "[docs-sync] $(date '+%H:%M:%S') $*"; }

gen_tree() {
  local dir="$1"
  tree "$dir" --charset=utf-8 -I 'node_modules|dist|.next|.turbo|go.sum|*.test.go|_test.go' \
    | tail -n +2 | head -n -1
}

sync_backend_structure() {
  log "updating BACKEND_STRUCTURE.md ..."

  local tmp=$(mktemp)
  cat > "$tmp" << 'HEADER'
# Backend Services Structure

## Gateway (`apps/gateway/`)

```
gateway/
HEADER

  gen_tree "$ROOT/apps/gateway/cmd" >> "$tmp"
  echo "" >> "$tmp"
  gen_tree "$ROOT/apps/gateway/internal" >> "$tmp"
  cat >> "$tmp" << 'FOOTER'
```

## Scheduler (`apps/scheduler/`)

```
scheduler/
FOOTER
  gen_tree "$ROOT/apps/scheduler" >> "$tmp"

  cat >> "$tmp" << 'FILESRV'
```

## File Service (`apps/file-service/`)

```
file-service/
FILESRV
  gen_tree "$ROOT/apps/file-service" >> "$tmp"

  cat >> "$tmp" << 'AGENT'
```

## Host Agent (`host-agent/`)

```
host-agent/
AGENT
  gen_tree "$ROOT/host-agent" >> "$tmp"

  cat >> "$tmp" << 'PACKAGES'
```

## Package Structures
PACKAGES

  for pkg in api-types ui utils; do
    if [ -d "$ROOT/packages/$pkg/src" ]; then
      echo -e "\n### \`packages/$pkg/\`\n" >> "$tmp"
      echo '```' >> "$tmp"
      echo "$pkg/" >> "$tmp"
      gen_tree "$ROOT/packages/$pkg/src" >> "$tmp"
      echo '```' >> "$tmp"
    fi
  done

  echo "" >> "$tmp"
  cp "$tmp" "$DOCS/BACKEND_STRUCTURE.md"
  rm "$tmp"
  log "BACKEND_STRUCTURE.md synced"
}

sync_desktop_structure() {
  log "updating DESKTOP_STRUCTURE.md ..."

  local tmp=$(mktemp)
  cat > "$tmp" << 'HEADER'
# Desktop/WebOS Structure

## Location

The WebOS desktop lives in `apps/frontend/app/webos/` (not a top-level `desktop/` directory).

## File Tree

```
webos/
HEADER

  gen_tree "$ROOT/apps/frontend/app/webos" >> "$tmp"

  cat >> "$tmp" << 'FOOTER'

## State Management

Zustand stores for:
- Desktop state (theme, wallpaper, settings)
- Window positions and focus
- App state and data
- System metrics (CPU, RAM, disk)

## Window Data Structure

```typescript
interface Window {
  id: string;
  title: string;
  app: string;
  x: number;
  y: number;
  width: number;
  height: number;
  zIndex: number;
  minimized: boolean;
  maximized: boolean;
  focused: boolean;
}
```

## Input Handling

All user input (mouse, keyboard) is captured and sent to backend via WebSocket.

## Streaming

Desktop frames received from backend as WebRTC video stream:
- VP9 or H.264 codec
- Adaptive bitrate based on connection
- Target: 60 FPS, <100ms latency
FOOTER

  cp "$tmp" "$DOCS/DESKTOP_STRUCTURE.md"
  rm "$tmp"
  log "DESKTOP_STRUCTURE.md synced"
}

sync_roadmap() {
  log "updating ROADMAP.md ..."
  # ROADMAP is manually curated; just note it should reflect current status
  log "ROADMAP.md is manually curated — update it when phases change"
}

full_sync() {
  sync_backend_structure
  sync_desktop_structure
}

case "${1:-watch}" in
  sync)
    full_sync
    ;;
  watch)
    log "starting watcher on codebase ..."
    full_sync
    while inotifywait -q -r -e modify,create,delete,move \
      "$ROOT/apps/gateway/internal" \
      "$ROOT/apps/gateway/cmd" \
      "$ROOT/apps/frontend/app/webos" \
      "$ROOT/host-agent" \
      "$ROOT/packages" \
      --exclude 'node_modules|dist|.next|.turbo'; do
      full_sync
    done
    ;;
  *)
    echo "Usage: $0 {sync|watch}"
    exit 1
    ;;
esac
