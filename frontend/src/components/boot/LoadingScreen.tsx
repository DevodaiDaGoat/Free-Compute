"use client";

import { useEffect, useState } from "react";

import { LOADING_DURATION_MS } from "@/lib/constants";

interface LoadingScreenProps {
  onComplete: () => void;
}

export default function LoadingScreen({ onComplete }: LoadingScreenProps) {
  const [progress, setProgress] = useState(0);

  useEffect(() => {
    const start = performance.now();
    let frame = 0;

    const tick = (now: number) => {
      const elapsed = now - start;
      const pct = Math.min(100, (elapsed / LOADING_DURATION_MS) * 100);
      setProgress(pct);
      if (pct < 100) {
        frame = requestAnimationFrame(tick);
      } else {
        onComplete();
      }
    };

    frame = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(frame);
  }, [onComplete]);

  return (
    <div className="flex h-full w-full flex-col items-center justify-center gap-8 bg-[var(--bg-primary)]">
      <div className="flex flex-col items-center gap-3">
        <div className="text-4xl font-bold tracking-tight text-white">
          Free<span className="text-[var(--accent)]">Compute</span>
        </div>
        <p className="text-sm text-white/50">Community Powered Cloud</p>
      </div>
      <div className="h-1.5 w-64 overflow-hidden rounded-full bg-white/10">
        <div
          className="h-full rounded-full bg-[var(--accent)] transition-[width] duration-100"
          style={{ width: `${progress}%` }}
        />
      </div>
      <p className="text-xs text-white/40">Loading desktop environment…</p>
    </div>
  );
}
