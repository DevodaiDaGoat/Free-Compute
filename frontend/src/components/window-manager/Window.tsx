"use client";

import { useCallback, useRef, type ReactNode } from "react";

import WindowControls from "@/components/window-manager/WindowControls";
import { TASKBAR_HEIGHT, WINDOW_TITLE_BAR_HEIGHT } from "@/lib/constants";
import type { DesktopWindow } from "@/lib/types";
import { cn } from "@/lib/utils";
import { useWindowStore } from "@/stores/windowStore";

interface WindowProps {
  window: DesktopWindow;
  icon?: string;
  children: ReactNode;
}

export default function Window({ window: win, icon, children }: WindowProps) {
  const focusWindow = useWindowStore((s) => s.focusWindow);
  const closeWindow = useWindowStore((s) => s.closeWindow);
  const minimizeWindow = useWindowStore((s) => s.minimizeWindow);
  const toggleMaximize = useWindowStore((s) => s.toggleMaximize);
  const moveWindow = useWindowStore((s) => s.moveWindow);
  const resizeWindow = useWindowStore((s) => s.resizeWindow);

  const dragState = useRef<{
    pointerId: number;
    offsetX: number;
    offsetY: number;
  } | null>(null);
  const resizeState = useRef<{
    pointerId: number;
    startX: number;
    startY: number;
    startWidth: number;
    startHeight: number;
  } | null>(null);

  const onTitlePointerDown = useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      if (win.maximized) return;
      focusWindow(win.id);
      dragState.current = {
        pointerId: event.pointerId,
        offsetX: event.clientX - win.x,
        offsetY: event.clientY - win.y,
      };
      event.currentTarget.setPointerCapture(event.pointerId);
    },
    [focusWindow, win.id, win.maximized, win.x, win.y],
  );

  const onTitlePointerMove = useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      const state = dragState.current;
      if (!state || state.pointerId !== event.pointerId) return;
      const maxX = window.innerWidth - 40;
      const maxY = window.innerHeight - TASKBAR_HEIGHT - WINDOW_TITLE_BAR_HEIGHT;
      moveWindow(win.id, {
        x: Math.min(Math.max(event.clientX - state.offsetX, -win.width + 80), maxX),
        y: Math.min(Math.max(event.clientY - state.offsetY, 0), maxY),
      });
    },
    [moveWindow, win.id, win.width],
  );

  const endDrag = useCallback((event: React.PointerEvent<HTMLDivElement>) => {
    if (dragState.current?.pointerId === event.pointerId) {
      dragState.current = null;
    }
  }, []);

  const onResizePointerDown = useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      event.stopPropagation();
      focusWindow(win.id);
      resizeState.current = {
        pointerId: event.pointerId,
        startX: event.clientX,
        startY: event.clientY,
        startWidth: win.width,
        startHeight: win.height,
      };
      event.currentTarget.setPointerCapture(event.pointerId);
    },
    [focusWindow, win.height, win.id, win.width],
  );

  const onResizePointerMove = useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      const state = resizeState.current;
      if (!state || state.pointerId !== event.pointerId) return;
      resizeWindow(win.id, {
        width: state.startWidth + (event.clientX - state.startX),
        height: state.startHeight + (event.clientY - state.startY),
      });
    },
    [resizeWindow, win.id],
  );

  const endResize = useCallback((event: React.PointerEvent<HTMLDivElement>) => {
    if (resizeState.current?.pointerId === event.pointerId) {
      resizeState.current = null;
    }
  }, []);

  if (win.minimized) return null;

  const style: React.CSSProperties = win.maximized
    ? {
        left: 0,
        top: 0,
        width: "100vw",
        height: `calc(100vh - ${TASKBAR_HEIGHT}px)`,
        zIndex: win.zIndex,
      }
    : {
        left: win.x,
        top: win.y,
        width: win.width,
        height: win.height,
        zIndex: win.zIndex,
      };

  return (
    <div
      className={cn(
        "absolute flex flex-col overflow-hidden rounded-lg border shadow-2xl",
        win.focused
          ? "border-[var(--accent)]/60 shadow-black/60"
          : "border-[var(--window-border)] shadow-black/40",
      )}
      style={style}
      onPointerDown={() => focusWindow(win.id)}
    >
      <div
        className={cn(
          "flex h-9 shrink-0 cursor-grab items-center justify-between px-3 select-none active:cursor-grabbing",
          win.focused ? "bg-[var(--bg-secondary)]" : "bg-black/60",
        )}
        onPointerDown={onTitlePointerDown}
        onPointerMove={onTitlePointerMove}
        onPointerUp={endDrag}
        onPointerCancel={endDrag}
        onDoubleClick={() => win.resizable && toggleMaximize(win.id)}
      >
        <div className="flex items-center gap-2 truncate text-sm text-white">
          {icon && <span aria-hidden>{icon}</span>}
          <span className="truncate">{win.title}</span>
        </div>
        <WindowControls
          closable={win.closable}
          resizable={win.resizable}
          onMinimize={() => minimizeWindow(win.id)}
          onMaximize={() => toggleMaximize(win.id)}
          onClose={() => closeWindow(win.id)}
        />
      </div>
      <div className="relative flex-1 overflow-hidden bg-[var(--bg-primary)]">
        {children}
      </div>
      {win.resizable && !win.maximized && (
        <div
          className="absolute bottom-0 right-0 h-4 w-4 cursor-nwse-resize"
          onPointerDown={onResizePointerDown}
          onPointerMove={onResizePointerMove}
          onPointerUp={endResize}
          onPointerCancel={endResize}
        >
          <div className="absolute bottom-1 right-1 h-2 w-2 border-b-2 border-r-2 border-white/30" />
        </div>
      )}
    </div>
  );
}
