"use client";

import { useEffect, useState } from "react";

import { BIOS_LINE_INTERVAL_MS } from "@/lib/constants";

const BIOS_LINES = [
  "FreeCompute BIOS v0.1.0 (Community Edition)",
  "Copyright (C) FreeCompute Project",
  "",
  "Detecting virtualized hardware...",
  "CPU: vCPU x8 @ 3.20GHz ... OK",
  "Memory Test: 16384MB ... OK",
  "GPU: Virtio-GPU (VRAM 4096MB) ... OK",
  "Network: Cloudflare Tunnel adapter ... OK",
  "Initializing WebRTC transport ... OK",
  "Mounting remote volume /dev/vda1 ... OK",
  "",
  "Booting FreeCompute WebOS...",
];

interface BIOSScreenProps {
  onComplete: () => void;
}

export default function BIOSScreen({ onComplete }: BIOSScreenProps) {
  const [visibleCount, setVisibleCount] = useState(0);

  useEffect(() => {
    if (visibleCount >= BIOS_LINES.length) {
      const timeout = setTimeout(onComplete, 500);
      return () => clearTimeout(timeout);
    }
    const timeout = setTimeout(
      () => setVisibleCount((count) => count + 1),
      BIOS_LINE_INTERVAL_MS,
    );
    return () => clearTimeout(timeout);
  }, [visibleCount, onComplete]);

  return (
    <div className="flex h-full w-full flex-col bg-black p-8 font-mono text-sm text-green-400">
      {BIOS_LINES.slice(0, visibleCount).map((line, index) => (
        <div key={index} className="whitespace-pre">
          {line || "\u00a0"}
        </div>
      ))}
      {visibleCount < BIOS_LINES.length && (
        <span className="mt-1 inline-block h-4 w-2 animate-pulse bg-green-400" />
      )}
    </div>
  );
}
