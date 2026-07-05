/**
 * Formatting utilities for the FreeCompute platform.
 */

export function formatBytes(bytes: number): string {
  if (typeof bytes !== "number" || !Number.isFinite(bytes) || bytes < 0)
    return "0 B";

  const units = ["B", "KB", "MB", "GB", "TB", "PB"];
  if (bytes === 0) return "0 B";

  const exp = Math.min(
    Math.floor(Math.log(bytes) / Math.log(1024)),
    units.length - 1
  );
  const value = bytes / Math.pow(1024, exp);
  const formatted = exp === 0 ? value.toString() : value.toFixed(2);

  return `${formatted} ${units[exp]}`;
}

export function formatDuration(ms: number): string {
  if (typeof ms !== "number" || !Number.isFinite(ms) || ms < 0) return "0s";

  const seconds = Math.floor(ms / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);

  if (days > 0) return `${days}d ${hours % 24}h`;
  if (hours > 0) return `${hours}h ${minutes % 60}m`;
  if (minutes > 0) return `${minutes}m ${seconds % 60}s`;
  return `${seconds}s`;
}

export function formatCredits(amount: number): string {
  if (typeof amount !== "number" || !Number.isFinite(amount)) return "0.00";
  return amount.toFixed(2);
}

export function formatCPU(cores: number): string {
  if (typeof cores !== "number" || cores < 0) return "0 cores";
  if (cores === 1) return "1 core";
  return `${cores} cores`;
}

export function formatRAM(gb: number): string {
  if (typeof gb !== "number" || gb < 0) return "0 GB";
  if (gb === 0) return "0 GB";
  if (gb < 1) return `${Math.round(gb * 1024)} MB`;
  return `${gb} GB`;
}

export function formatPercent(value: number, decimals: number = 1): string {
  if (typeof value !== "number" || !Number.isFinite(value)) return "0%";
  return `${value.toFixed(decimals)}%`;
}

export function truncate(str: string, maxLength: number): string {
  if (typeof str !== "string") return "";
  if (str.length <= maxLength) return str;
  if (maxLength <= 3) return str.slice(0, maxLength);
  return str.slice(0, maxLength - 3) + "...";
}

export function slugify(text: string): string {
  if (typeof text !== "string") return "";
  return text
    .toLowerCase()
    .trim()
    .replace(/[^\w\s-]/g, "")
    .replace(/[\s_]+/g, "-")
    .replace(/-+/g, "-")
    .replace(/^-|-$/g, "");
}
