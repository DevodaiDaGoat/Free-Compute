# Progressive Web App (PWA) Support

## Overview

PWA support enables FreeCompute to be installed as a native-class application on desktop and mobile, with offline capability, background sync, and push notifications — critical for remote desktop and gaming sessions that need persistent connectivity.

## Capabilities

| Feature | Implementation | Priority |
|---------|---------------|----------|
| Installable | Web App Manifest + Service Worker | P0 |
| Offline Dashboard | Cache console UI shell via SW | P1 |
| Background Sync | Retry failed requests on reconnect | P2 |
| Push Notifications | Session state changes / auth requests | P2 |
| Periodic Sync | Refresh capabilities in background | P3 |

## Web App Manifest

Generated at `apps/frontend/public/manifest.json`:

```json
{
  "name": "FreeCompute Console",
  "short_name": "FreeCompute",
  "description": "Remote access, game streaming, and universal proxy operations console.",
  "start_url": "/",
  "display": "standalone",
  "background_color": "#f3f5ef",
  "theme_color": "#17211c",
  "icons": [
    { "src": "/icons/icon-192.png", "sizes": "192x192", "type": "image/png" },
    { "src": "/icons/icon-512.png", "sizes": "512x512", "type": "image/png" },
    { "src": "/icons/icon-512.png", "sizes": "512x512", "type": "image/png", "purpose": "maskable" }
  ],
  "categories": ["productivity", "utilities"],
  "scope": "/"
}
```

## Service Worker Strategy

### Cache-First for App Shell

The console UI shell (HTML, CSS, JS, fonts) uses a cache-first strategy:

```
SW Install → Precache shell assets
SW Activate → Clean old caches
SW Fetch → 
  ├── Shell assets → Cache-first (instant load)
  ├── API calls → Network-first (fresh data)
  └── Third-party → Network-only
```

### Network-First for API & Streaming

Gateway API calls and WebSocket connections should never be cached:

- `/api/gateway/*` — Always network-first, fallback to stale if offline
- `/webrtc/*`, `/signal/*`, `/ws/*` — Never intercepted by SW
- `/proxy/*` — Streamed directly, bypass SW

### Offline Fallback

When offline:
- Show cached dashboard with "offline" indicator
- Queue session creation requests for background sync
- Display last-known gateway status from IndexedDB

## Registration

```typescript
// app/sw/register.ts
export function registerSW() {
  if ('serviceWorker' in navigator) {
    window.addEventListener('load', async () => {
      const reg = await navigator.serviceWorker.register('/sw.js', {
        scope: '/',
        updateViaCache: 'none',
      });

      reg.addEventListener('updatefound', () => {
        const newSW = reg.installing;
        newSW?.addEventListener('statechange', () => {
          if (newSW.state === 'installed' && navigator.serviceWorker.controller) {
            showUpdateToast('Update available');
          }
        });
      });
    });
  }
}
```

## Background Sync

Unsynchronized session creations or input events are queued in IndexedDB and retried when connectivity resumes:

```typescript
async function queueBackgroundSync(tag: string, request: Request) {
  const cache = await caches.open('sync-queue');
  await cache.put(tag, request);
  const sync = self.registration?.sync;
  if (sync) await sync.register(tag);
}
```

## Push Notifications

Gateway sends push events for:
- Session ready / error
- Auth request (remote support approval)
- Resource limit warnings
- Idle timeout warning

## Performance Impact

| Asset | Without SW | With SW | Improvement |
|-------|-----------|---------|-------------|
| Dashboard load (repeat) | ~1.2s | ~80ms | 15x |
| Capabilities fetch | ~200ms | ~15ms (stale) | 13x |
| Offline resilience | None | Full shell | ∞ |

## Implementation

1. Generate manifest.json and icon assets
2. Write service worker with cache-first shell strategy
3. Register SW in root layout with update prompt
4. Implement background sync for critical requests
5. Add push notification subscription UI in Settings
