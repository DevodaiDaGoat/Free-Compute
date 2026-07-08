const CACHE_NAME = 'freecompute-v2';
const SHELL_ASSETS = [
  '/',
  '/webos',
];

const NEVER_CACHE = [
  /\/api\//,
  /\/proxy\//,
  /\/ws\//,
  /\/webrtc\//,
  /\/signal\//,
  /\/connect\//,
  /\/agent\//,
  /\/prewarm/,
];

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => {
      return cache.addAll(SHELL_ASSETS);
    }).then(() => self.skipWaiting()),
  );
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(keys.map((k) => { if (k !== CACHE_NAME) return caches.delete(k); })),
    ).then(() => self.clients.claim()),
  );
});

self.addEventListener('fetch', (event) => {
  const url = new URL(event.request.url);

  if (NEVER_CACHE.some((p) => p.test(url.pathname))) {
    return;
  }

  if (event.request.mode === 'navigate') {
    event.respondWith(
      fetch(event.request).catch(() => caches.match('/')),
    );
    return;
  }

  event.respondWith(
    caches.match(event.request).then((cached) => {
      const fetchPromise = fetch(event.request).then((response) => {
        if (response.ok) {
          const clone = response.clone();
          caches.open(CACHE_NAME).then((cache) => cache.put(event.request, clone));
        }
        return response;
      }).catch(() => cached);
      return cached || fetchPromise;
    }),
  );
});

self.addEventListener('message', (event) => {
  if (event.data?.type === 'SKIP_WAITING') {
    self.skipWaiting();
  }
});

self.addEventListener('sync', (event) => {
  if (event.tag === 'session-sync') {
    event.waitUntil(syncSessions());
  }
});

async function syncSessions() {
  const cache = await caches.open('sync-queue');
  const requests = await cache.keys();
  for (const request of requests) {
    try {
      await fetch(request);
      await cache.delete(request);
    } catch (e) {
      console.warn('[SW] Sync failed, will retry:', e);
    }
  }
}

self.addEventListener('push', (event) => {
  if (!event.data) return;

  let data;
  try {
    data = event.data.json();
  } catch {
    data = { title: 'FreeCompute', body: event.data.text() };
  }

  const title = data.title || 'FreeCompute';
  const options = {
    body: data.body || '',
    icon: '/icons/icon-192.png',
    badge: '/icons/icon-192.png',
    tag: data.tag || 'default',
    data: data.url ? { url: data.url } : {},
  };

  event.waitUntil(self.registration.showNotification(title, options));
});

self.addEventListener('notificationclick', (event) => {
  event.notification.close();
  if (event.notification.data?.url) {
    event.waitUntil(clients.openWindow(event.notification.data.url));
  }
});
