"use client";

import { useDesktopStore } from "@/stores/desktopStore";
import { useAppStore } from "@/stores/appStore";

interface StartMenuProps {
  open: boolean;
  onClose: () => void;
}

export default function StartMenu({ open, onClose }: StartMenuProps) {
  const registry = useAppStore((s) => s.registry);
  const launchApp = useAppStore((s) => s.launchApp);
  const logout = useDesktopStore((s) => s.logout);

  if (!open) return null;

  const handleLaunch = (appId: string) => {
    launchApp(appId);
    onClose();
  };

  return (
    <div className="absolute bottom-14 left-2 z-[9000] w-80 rounded-xl border border-[var(--window-border)] bg-[var(--bg-secondary)]/95 p-4 shadow-2xl backdrop-blur">
      <p className="px-1 pb-2 text-xs uppercase tracking-wide text-white/40">
        Applications
      </p>
      <div className="grid grid-cols-3 gap-2">
        {registry.map((app) => (
          <button
            key={app.id}
            type="button"
            onClick={() => handleLaunch(app.id)}
            className="flex flex-col items-center gap-1 rounded-lg p-3 text-center text-white hover:bg-white/10"
          >
            <span className="text-2xl" aria-hidden>
              {app.icon}
            </span>
            <span className="text-xs">{app.title}</span>
          </button>
        ))}
      </div>
      <div className="mt-3 border-t border-[var(--window-border)] pt-3">
        <button
          type="button"
          onClick={() => {
            logout();
            onClose();
          }}
          className="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm text-white/80 hover:bg-white/10"
        >
          <span aria-hidden>⏻</span>
          Sign Out
        </button>
      </div>
    </div>
  );
}
