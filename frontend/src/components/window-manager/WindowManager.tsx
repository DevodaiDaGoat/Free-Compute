"use client";

import Window from "@/components/window-manager/Window";
import { getApp } from "@/components/apps/registry";
import { useWindowStore } from "@/stores/windowStore";

export default function WindowManager() {
  const windows = useWindowStore((s) => s.windows);

  return (
    <div className="pointer-events-none absolute inset-0">
      <div className="pointer-events-auto h-full w-full">
        {windows.map((win) => {
          const app = getApp(win.app);
          if (!app) return null;
          const AppComponent = app.component;
          return (
            <Window key={win.id} window={win} icon={app.icon}>
              <AppComponent window={win} />
            </Window>
          );
        })}
      </div>
    </div>
  );
}
