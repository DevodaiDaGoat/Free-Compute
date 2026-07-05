"use client";

import type { AppWindowProps } from "@/lib/types";
import { DEFAULT_WALLPAPER } from "@/lib/constants";
import { useDesktopStore } from "@/stores/desktopStore";
import { cn } from "@/lib/utils";

const WALLPAPERS: Array<{ label: string; value: string }> = [
  { label: "Midnight", value: DEFAULT_WALLPAPER },
  { label: "Aurora", value: "linear-gradient(135deg, #0a0a0a 0%, #1a1030 50%, #2a0a3f 100%)" },
  { label: "Ocean", value: "linear-gradient(135deg, #04121a 0%, #062b3a 50%, #0a1a1f 100%)" },
];

export default function Settings(_props: AppWindowProps) {
  void _props;
  const theme = useDesktopStore((s) => s.theme);
  const toggleTheme = useDesktopStore((s) => s.toggleTheme);
  const wallpaper = useDesktopStore((s) => s.wallpaper);
  const setWallpaper = useDesktopStore((s) => s.setWallpaper);

  return (
    <div className="h-full overflow-auto bg-[var(--bg-secondary)] p-5 text-white">
      <h2 className="text-lg font-semibold">Settings</h2>

      <section className="mt-4">
        <h3 className="text-sm font-medium text-white/70">Appearance</h3>
        <div className="mt-2 flex items-center justify-between rounded-lg bg-black/30 px-4 py-3">
          <span className="text-sm">Theme</span>
          <button
            type="button"
            onClick={toggleTheme}
            className="rounded-full bg-[var(--accent)] px-4 py-1 text-sm font-medium text-black"
          >
            {theme === "dark" ? "Dark" : "Light"}
          </button>
        </div>
      </section>

      <section className="mt-4">
        <h3 className="text-sm font-medium text-white/70">Wallpaper</h3>
        <div className="mt-2 grid grid-cols-3 gap-3">
          {WALLPAPERS.map((wp) => (
            <button
              key={wp.label}
              type="button"
              onClick={() => setWallpaper(wp.value)}
              className={cn(
                "flex h-20 flex-col justify-end rounded-lg border-2 p-2 text-left text-xs",
                wallpaper === wp.value ? "border-[var(--accent)]" : "border-transparent",
              )}
              style={{ background: wp.value }}
            >
              {wp.label}
            </button>
          ))}
        </div>
      </section>

      <section className="mt-4">
        <h3 className="text-sm font-medium text-white/70">About</h3>
        <div className="mt-2 rounded-lg bg-black/30 px-4 py-3 text-sm text-white/70">
          <p>FreeCompute WebOS</p>
          <p className="text-white/40">Version 0.1.0 — pre-alpha</p>
        </div>
      </section>
    </div>
  );
}
