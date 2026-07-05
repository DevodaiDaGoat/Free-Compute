"use client";

import { useState } from "react";

import Clock from "@/components/taskbar/Clock";
import StartMenu from "@/components/taskbar/StartMenu";
import SystemTray from "@/components/taskbar/SystemTray";
import { getApp } from "@/components/apps/registry";
import { TASKBAR_HEIGHT } from "@/lib/constants";
import { cn } from "@/lib/utils";
import { useWindowStore } from "@/stores/windowStore";

export default function Taskbar() {
  const [startOpen, setStartOpen] = useState(false);
  const windows = useWindowStore((s) => s.windows);
  const focusWindow = useWindowStore((s) => s.focusWindow);
  const minimizeWindow = useWindowStore((s) => s.minimizeWindow);

  const handleTaskClick = (id: string, focused: boolean, minimized: boolean) => {
    if (focused && !minimized) {
      minimizeWindow(id);
    } else {
      focusWindow(id);
    }
  };

  return (
    <>
      <StartMenu open={startOpen} onClose={() => setStartOpen(false)} />
      <div
        className="absolute bottom-0 left-0 right-0 z-[8000] flex items-center gap-2 border-t border-[var(--window-border)] bg-[var(--taskbar-bg)] px-2 backdrop-blur"
        style={{ height: TASKBAR_HEIGHT }}
      >
        <button
          type="button"
          onClick={() => setStartOpen((open) => !open)}
          className={cn(
            "flex items-center gap-2 rounded-lg px-3 py-1.5 text-sm font-semibold text-white transition-colors",
            startOpen ? "bg-[var(--accent)] text-black" : "hover:bg-white/10",
          )}
        >
          <span aria-hidden>⬢</span>
          Start
        </button>

        <div className="flex flex-1 items-center gap-1 overflow-x-auto">
          {windows.map((win) => {
            const app = getApp(win.app);
            return (
              <button
                key={win.id}
                type="button"
                onClick={() => handleTaskClick(win.id, win.focused, win.minimized)}
                className={cn(
                  "flex max-w-40 items-center gap-2 rounded-lg px-3 py-1.5 text-sm text-white transition-colors",
                  win.focused && !win.minimized
                    ? "bg-white/20"
                    : "bg-white/5 hover:bg-white/10",
                )}
              >
                <span aria-hidden>{app?.icon}</span>
                <span className="truncate">{win.title}</span>
              </button>
            );
          })}
        </div>

        <SystemTray />
        <Clock />
      </div>
    </>
  );
}
