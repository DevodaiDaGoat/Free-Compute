"use client";

import { useState } from "react";

import type { AppWindowProps } from "@/lib/types";

export default function Browser(_props: AppWindowProps) {
  void _props;
  const [address, setAddress] = useState("https://free-compute.local/welcome");

  return (
    <div className="flex h-full flex-col bg-[var(--bg-secondary)]">
      <div className="flex items-center gap-2 border-b border-[var(--window-border)] p-2">
        <div className="flex gap-1">
          <button
            type="button"
            className="rounded px-2 py-1 text-sm text-white/70 hover:bg-white/10"
            aria-label="Back"
          >
            ←
          </button>
          <button
            type="button"
            className="rounded px-2 py-1 text-sm text-white/70 hover:bg-white/10"
            aria-label="Forward"
          >
            →
          </button>
        </div>
        <input
          value={address}
          onChange={(event) => setAddress(event.target.value)}
          className="flex-1 rounded-full bg-black/40 px-4 py-1.5 text-sm text-white outline-none focus:ring-1 focus:ring-[var(--accent)]"
          spellCheck={false}
        />
      </div>
      <div className="flex flex-1 flex-col items-center justify-center gap-2 text-center text-white/60">
        <div className="text-4xl">🌐</div>
        <p className="text-lg font-medium text-white">Browser</p>
        <p className="max-w-xs text-sm">
          A sandboxed browser stub. Remote page rendering will stream over WebRTC.
        </p>
      </div>
    </div>
  );
}
