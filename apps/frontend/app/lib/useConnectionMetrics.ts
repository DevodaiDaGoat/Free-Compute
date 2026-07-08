'use client';

import { useCallback, useEffect, useRef, useState } from 'react';

export interface ConnectionMetrics {
  rttMs: number | null;
  jitterMs: number;
  packetLoss: number;
  quality: 'excellent' | 'good' | 'degraded' | 'poor' | 'unknown';
  history: { ts: number; rtt: number }[];
}

const MAX_HISTORY = 120;

export function useConnectionMetrics(gatewayUrl: string, intervalMs = 3000) {
  const [metrics, setMetrics] = useState<ConnectionMetrics>({
    rttMs: null,
    jitterMs: 0,
    packetLoss: 0,
    quality: 'unknown',
    history: [],
  });

  const prevRttRef = useRef<number | null>(null);
  const measurementsRef = useRef<number[]>([]);

  const measureRtt = useCallback(async () => {
    const start = performance.now();
    try {
      await fetch(`${gatewayUrl}/healthz`, {
        method: 'HEAD',
        signal: AbortSignal.timeout(2000),
      });
      return performance.now() - start;
    } catch {
      return null;
    }
  }, [gatewayUrl]);

  useEffect(() => {
    if (!gatewayUrl) return;

    let running = true;

    const tick = async () => {
      if (!running) return;
      const rtt = await measureRtt();

      if (rtt !== null) {
        measurementsRef.current.push(rtt);
        if (measurementsRef.current.length > 10) measurementsRef.current.shift();

        const avgRtt = measurementsRef.current.reduce((a, b) => a + b, 0) / measurementsRef.current.length;

        let jitter = 0;
        if (prevRttRef.current !== null) {
          jitter = Math.abs(rtt - prevRttRef.current);
        }
        prevRttRef.current = rtt;

        const quality: ConnectionMetrics['quality'] =
          avgRtt < 30 ? 'excellent' : avgRtt < 80 ? 'good' : avgRtt < 150 ? 'degraded' : 'poor';

        const packetLoss = avgRtt > 200 ? Math.min((avgRtt - 150) / 500, 0.2) : 0;

        setMetrics((prev) => {
          const history = [...prev.history, { ts: Date.now(), rtt }];
          if (history.length > MAX_HISTORY) history.splice(0, history.length - MAX_HISTORY);
          return { rttMs: avgRtt, jitterMs: jitter, packetLoss, quality, history };
        });
      }

      if (running) setTimeout(tick, intervalMs);
    };

    tick();

    return () => { running = false; };
  }, [gatewayUrl, intervalMs, measureRtt]);

  return metrics;
}
