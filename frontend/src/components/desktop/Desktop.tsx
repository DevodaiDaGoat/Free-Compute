"use client";

import { useCallback } from "react";

import ContextMenu from "@/components/desktop/ContextMenu";
import NotificationCenter from "@/components/desktop/NotificationCenter";
import Wallpaper from "@/components/desktop/Wallpaper";
import Taskbar from "@/components/taskbar/Taskbar";
import WindowManager from "@/components/window-manager/WindowManager";
import type { ContextMenuItem } from "@/lib/types";
import { useAppStore } from "@/stores/appStore";
import { useDesktopStore } from "@/stores/desktopStore";

export default function Desktop() {
  const openContextMenu = useDesktopStore((s) => s.openContextMenu);
  const pushNotification = useDesktopStore((s) => s.pushNotification);
  const launchApp = useAppStore((s) => s.launchApp);
  const registry = useAppStore((s) => s.registry);

  const handleContextMenu = useCallback(
    (event: React.MouseEvent) => {
      event.preventDefault();
      const items: ContextMenuItem[] = [
        ...registry.map((app) => ({
          id: `launch-${app.id}`,
          label: `Open ${app.title}`,
          onClick: () => launchApp(app.id),
        })),
        {
          id: "about",
          label: "About FreeCompute",
          separatorAfter: false,
          onClick: () =>
            pushNotification({
              title: "FreeCompute WebOS",
              message: "Community powered cloud desktop — pre-alpha.",
              variant: "info",
            }),
        },
      ];
      openContextMenu({ x: event.clientX, y: event.clientY, items });
    },
    [launchApp, openContextMenu, pushNotification, registry],
  );

  return (
    <div
      className="relative h-screen w-screen overflow-hidden"
      onContextMenu={handleContextMenu}
    >
      <Wallpaper />
      <WindowManager />
      <NotificationCenter />
      <ContextMenu />
      <Taskbar />
    </div>
  );
}
