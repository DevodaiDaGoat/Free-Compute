"use client";

import { useEffect } from "react";

import type { NotificationVariant } from "@/lib/types";
import { cn } from "@/lib/utils";
import { useDesktopStore } from "@/stores/desktopStore";

const VARIANT_STYLES: Record<NotificationVariant, string> = {
  info: "border-[var(--accent)]/60",
  success: "border-green-500/60",
  warning: "border-yellow-500/60",
  error: "border-red-500/60",
};

const VARIANT_ICON: Record<NotificationVariant, string> = {
  info: "ℹ️",
  success: "✅",
  warning: "⚠️",
  error: "⛔",
};

export default function NotificationCenter() {
  const notifications = useDesktopStore((s) => s.notifications);
  const dismiss = useDesktopStore((s) => s.dismissNotification);

  useEffect(() => {
    if (notifications.length === 0) return;
    const timers = notifications.map((notification) =>
      setTimeout(() => dismiss(notification.id), 5000),
    );
    return () => timers.forEach(clearTimeout);
  }, [notifications, dismiss]);

  return (
    <div className="pointer-events-none absolute right-3 top-3 z-[9500] flex w-80 flex-col gap-2">
      {notifications.map((notification) => (
        <div
          key={notification.id}
          className={cn(
            "pointer-events-auto flex gap-3 rounded-lg border-l-4 bg-[var(--bg-secondary)]/95 p-3 shadow-xl backdrop-blur",
            VARIANT_STYLES[notification.variant],
          )}
        >
          <span aria-hidden>{VARIANT_ICON[notification.variant]}</span>
          <div className="flex-1">
            <p className="text-sm font-medium text-white">{notification.title}</p>
            <p className="text-xs text-white/60">{notification.message}</p>
          </div>
          <button
            type="button"
            aria-label="Dismiss notification"
            onClick={() => dismiss(notification.id)}
            className="text-white/40 hover:text-white"
          >
            ✕
          </button>
        </div>
      ))}
    </div>
  );
}
