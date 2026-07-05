"use client";

import { useDesktopStore } from "@/stores/desktopStore";

export default function SystemTray() {
  const toggleTheme = useDesktopStore((s) => s.toggleTheme);
  const theme = useDesktopStore((s) => s.theme);

  return (
    <div className="flex items-center gap-1 px-1 text-white/70">
      <span className="grid h-7 w-7 place-items-center rounded hover:bg-white/10" title="Network">
        📶
      </span>
      <span className="grid h-7 w-7 place-items-center rounded hover:bg-white/10" title="Volume">
        🔊
      </span>
      <button
        type="button"
        onClick={toggleTheme}
        title="Toggle theme"
        className="grid h-7 w-7 place-items-center rounded hover:bg-white/10"
      >
        {theme === "dark" ? "🌙" : "☀️"}
      </button>
    </div>
  );
}
