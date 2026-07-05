/**
 * Date/time utilities for the FreeCompute platform.
 */

export function timeAgo(date: Date | string | number): string {
  const now = Date.now();
  const then =
    date instanceof Date
      ? date.getTime()
      : typeof date === "string"
      ? new Date(date).getTime()
      : date;

  if (isNaN(then)) return "unknown";

  const diffMs = now - then;
  if (diffMs < 0) return "just now";

  const seconds = Math.floor(diffMs / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);
  const months = Math.floor(days / 30);
  const years = Math.floor(days / 365);

  if (years > 0) return `${years}y ago`;
  if (months > 0) return `${months}mo ago`;
  if (days > 0) return `${days}d ago`;
  if (hours > 0) return `${hours}h ago`;
  if (minutes > 0) return `${minutes}m ago`;
  if (seconds > 0) return `${seconds}s ago`;
  return "just now";
}

export function isExpired(expiresAt: Date | string | number): boolean {
  const expiry =
    expiresAt instanceof Date
      ? expiresAt.getTime()
      : typeof expiresAt === "string"
      ? new Date(expiresAt).getTime()
      : expiresAt;

  if (isNaN(expiry)) return true;
  return Date.now() > expiry;
}

export function addMinutes(date: Date, minutes: number): Date {
  return new Date(date.getTime() + minutes * 60 * 1000);
}

export function addHours(date: Date, hours: number): Date {
  return new Date(date.getTime() + hours * 60 * 60 * 1000);
}

export function addDays(date: Date, days: number): Date {
  return new Date(date.getTime() + days * 24 * 60 * 60 * 1000);
}

export function startOfDay(date: Date): Date {
  const d = new Date(date);
  d.setHours(0, 0, 0, 0);
  return d;
}

export function endOfDay(date: Date): Date {
  const d = new Date(date);
  d.setHours(23, 59, 59, 999);
  return d;
}

export function isSameDay(a: Date, b: Date): boolean {
  return (
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()
  );
}

export function diffInMinutes(a: Date, b: Date): number {
  return Math.abs(a.getTime() - b.getTime()) / (1000 * 60);
}

export function diffInHours(a: Date, b: Date): number {
  return Math.abs(a.getTime() - b.getTime()) / (1000 * 60 * 60);
}

export function formatISODate(date: Date): string {
  return date.toISOString().split("T")[0];
}
