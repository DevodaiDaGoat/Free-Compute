export function registerSW() {
  if (!('serviceWorker' in navigator)) return;

  window.addEventListener('load', async () => {
    try {
      const reg = await navigator.serviceWorker.register('/sw.js', {
        scope: '/',
        updateViaCache: 'none',
      });

      reg.addEventListener('updatefound', () => {
        const newSW = reg.installing;
        if (!newSW) return;
        newSW.addEventListener('statechange', () => {
          if (newSW.state === 'installed' && navigator.serviceWorker.controller) {
            console.log('[SW] Update available');
          }
        });
      });

      if (reg.active) {
        console.log('[SW] Registered');
      }

      await subscribePush(reg);
    } catch (err) {
      console.warn('[SW] Registration failed:', err);
    }
  });
}

async function subscribePush(reg: ServiceWorkerRegistration) {
  if (!('PushManager' in window)) return;
  try {
    const sub = await reg.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: urlBase64ToUint8Array(
        typeof process !== 'undefined' && (process as any).env?.NEXT_PUBLIC_VAPID_PUBLIC_KEY
          ? (process as any).env.NEXT_PUBLIC_VAPID_PUBLIC_KEY
          : 'BEl62iUYgUivxIkv69yViEuiBIa-Ib9SMkvMA1kBYn5aVGiX0bMkMwPt9ZxRGRjDm4gp4oLyL2wW8fR3gL7PFAo'
      ) as Uint8Array<ArrayBuffer>,
    });
    console.log('[SW] Push subscribed:', sub.endpoint);
  } catch (err) {
    console.warn('[SW] Push subscription failed:', err);
  }
}

function urlBase64ToUint8Array(base64String: string): Uint8Array {
  const padding = '='.repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/');
  const rawData = atob(base64);
  return Uint8Array.from(rawData, (c) => c.charCodeAt(0));
}
