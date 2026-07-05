import { TASKBAR_HEIGHT } from "@/lib/constants";
import type { Point, Size } from "@/lib/types";

export function generateId(prefix = "id"): string {
  const random = Math.random().toString(36).slice(2, 10);
  return `${prefix}-${Date.now().toString(36)}-${random}`;
}

export function clamp(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max);
}

export function clampPositionToViewport(
  position: Point,
  size: Size,
  viewport: Size,
): Point {
  const maxX = Math.max(0, viewport.width - size.width);
  const maxY = Math.max(0, viewport.height - TASKBAR_HEIGHT - size.height);
  return {
    x: clamp(position.x, 0, maxX),
    y: clamp(position.y, 0, maxY),
  };
}

export function cn(...classes: Array<string | false | null | undefined>): string {
  return classes.filter(Boolean).join(" ");
}

export function formatTime(date: Date): string {
  return date.toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

export function formatDate(date: Date): string {
  return date.toLocaleDateString([], {
    weekday: "short",
    month: "short",
    day: "numeric",
  });
}
