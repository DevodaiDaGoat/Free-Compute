import { NextRequest } from 'next/server';

export const dynamic = 'force-dynamic';
export const runtime = 'nodejs';

const gatewayUrl = process.env.FREECOMPUTE_GATEWAY_URL ?? 'http://127.0.0.1:8080';

const MIN_INTERVAL = 1000;
const MAX_INTERVAL = 30000;
const BACKOFF_MULTIPLIER = 1.5;
const THROTTLE_AFTER = 5;

export async function GET(req: NextRequest) {
  const encoder = new TextEncoder();

  const stream = new ReadableStream({
    start(controller) {
      let running = true;
      let consecutiveFailures = 0;
      let pollTimer: ReturnType<typeof setTimeout> | null = null;

      const send = (event: string, data: unknown) => {
        if (!running) return;
        try {
          controller.enqueue(encoder.encode(`event: ${event}\ndata: ${JSON.stringify(data)}\n\n`));
        } catch {
          running = false;
        }
      };

      const sendKeepalive = () => {
        send('keepalive', { ts: Date.now() });
      };

      let keepaliveTimer: ReturnType<typeof setInterval> | null = null;

      const getInterval = () => {
        if (consecutiveFailures === 0) return MIN_INTERVAL;
        const interval = Math.min(
          MIN_INTERVAL * Math.pow(BACKOFF_MULTIPLIER, consecutiveFailures),
          MAX_INTERVAL,
        );
        return Math.round(interval);
      };

      const measureRtt = async (): Promise<number | null> => {
        const start = performance.now();
        try {
          await fetch(new URL('/healthz', gatewayUrl), {
            signal: AbortSignal.timeout(2000),
            method: 'HEAD',
          });
          return performance.now() - start;
        } catch {
          return null;
        }
      };

      const poll = async () => {
        while (running) {
          const pollStart = Date.now();

          try {
            const [healthRes, capsRes] = await Promise.allSettled([
              fetch(new URL('/healthz', gatewayUrl), { signal: AbortSignal.timeout(2000) }),
              fetch(new URL('/capabilities', gatewayUrl), { signal: AbortSignal.timeout(2000) }),
            ]);

            if (healthRes.status === 'fulfilled') {
              consecutiveFailures = 0;
              const h = healthRes.value;
              send('health', { ok: h.ok, status: h.status, rtt: await measureRtt() });
            } else {
              consecutiveFailures++;
              send('health', { ok: false, status: 0, error: healthRes.reason?.message });
            }

            if (capsRes.status === 'fulfilled') {
              const c = await capsRes.value.json();
              send('capabilities', {
                live: true,
                protocols: c.protocols ?? [],
                transports: c.transports ?? [],
                routeModes: c.routeModes ?? [],
              });
            } else {
              send('capabilities', { live: false, error: capsRes.reason?.message });
            }
          } catch {
            consecutiveFailures++;
            send('health', { ok: false, status: 0 });
          }

          if (!running) break;

          const elapsed = Date.now() - pollStart;
          const interval = getInterval();
          const delay = Math.max(0, interval - elapsed);

          await new Promise((r) => { pollTimer = setTimeout(r, delay); });
        }
      };

      const startKeepalive = () => {
        keepaliveTimer = setInterval(sendKeepalive, 15000);
      };

      req.signal.addEventListener('abort', () => {
        running = false;
        if (pollTimer) clearTimeout(pollTimer);
        if (keepaliveTimer) clearInterval(keepaliveTimer);
        controller.close();
      });

      startKeepalive();
      poll().catch(() => {
        running = false;
        if (keepaliveTimer) clearInterval(keepaliveTimer);
        controller.close();
      });
    },
  });

  return new Response(stream, {
    headers: {
      'Content-Type': 'text/event-stream',
      'Cache-Control': 'no-cache, no-store',
      'Connection': 'keep-alive',
      'X-Accel-Buffering': 'no',
    },
  });
}
