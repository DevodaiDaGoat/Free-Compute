"use client";

import { useEffect } from "react";

import { useDesktopStore } from "@/stores/desktopStore";

export default function ContextMenu() {
  const contextMenu = useDesktopStore((s) => s.contextMenu);
  const close = useDesktopStore((s) => s.closeContextMenu);

  useEffect(() => {
    if (!contextMenu) return;
    const handler = () => close();
    window.addEventListener("click", handler);
    window.addEventListener("blur", handler);
    return () => {
      window.removeEventListener("click", handler);
      window.removeEventListener("blur", handler);
    };
  }, [contextMenu, close]);

  if (!contextMenu) return null;

  return (
    <ul
      className="absolute z-[9800] min-w-48 rounded-lg border border-[var(--window-border)] bg-[var(--bg-secondary)]/95 p-1 shadow-2xl backdrop-blur"
      style={{ left: contextMenu.x, top: contextMenu.y }}
      role="menu"
    >
      {contextMenu.items.map((item) => (
        <li key={item.id} className={item.separatorAfter ? "border-b border-[var(--window-border)] pb-1 mb-1" : undefined}>
          <button
            type="button"
            role="menuitem"
            disabled={item.disabled}
            onClick={() => {
              item.onClick();
              close();
            }}
            className="flex w-full items-center rounded px-3 py-1.5 text-left text-sm text-white hover:bg-white/10 disabled:cursor-not-allowed disabled:text-white/30"
          >
            {item.label}
          </button>
        </li>
      ))}
    </ul>
  );
}
