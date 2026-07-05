/**
 * Validation utilities for the FreeCompute platform.
 */

export function isValidEmail(email: string): boolean {
  if (!email || typeof email !== "string") return false;
  const re = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
  return re.test(email.trim());
}

export function isValidPassword(password: string): boolean {
  if (!password || typeof password !== "string") return false;
  if (password.length < 8) return false;
  if (password.length > 128) return false;
  if (!/[A-Z]/.test(password)) return false;
  if (!/[a-z]/.test(password)) return false;
  if (!/[0-9]/.test(password)) return false;
  return true;
}

export function isValidUUID(id: string): boolean {
  if (!id || typeof id !== "string") return false;
  const re =
    /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
  return re.test(id);
}

export function isValidHostname(hostname: string): boolean {
  if (!hostname || typeof hostname !== "string") return false;
  if (hostname.length > 253) return false;
  const re = /^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)*[a-zA-Z]{2,}$/;
  return re.test(hostname);
}

export interface VMConfig {
  cpuCores: number;
  ramGB: number;
  storageGB: number;
  gpuVramGB?: number;
}

export function validateVMConfig(config: VMConfig): string[] {
  const errors: string[] = [];

  if (!Number.isInteger(config.cpuCores) || config.cpuCores < 1) {
    errors.push("cpuCores must be a positive integer");
  }
  if (config.cpuCores > 64) {
    errors.push("cpuCores cannot exceed 64");
  }
  if (typeof config.ramGB !== "number" || config.ramGB <= 0) {
    errors.push("ramGB must be a positive number");
  }
  if (config.ramGB > 512) {
    errors.push("ramGB cannot exceed 512");
  }
  if (typeof config.storageGB !== "number" || config.storageGB < 0) {
    errors.push("storageGB cannot be negative");
  }
  if (config.storageGB > 2048) {
    errors.push("storageGB cannot exceed 2048");
  }
  if (config.gpuVramGB !== undefined) {
    if (typeof config.gpuVramGB !== "number" || config.gpuVramGB < 0) {
      errors.push("gpuVramGB cannot be negative");
    }
    if (config.gpuVramGB > 80) {
      errors.push("gpuVramGB cannot exceed 80");
    }
  }

  return errors;
}

export function sanitizeInput(input: string): string {
  if (typeof input !== "string") return "";
  return input
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#x27;");
}

export function isValidCreditAmount(amount: number): boolean {
  if (typeof amount !== "number" || !Number.isFinite(amount)) return false;
  if (amount <= 0) return false;
  if (amount > 10000) return false;
  // Max 2 decimal places
  const rounded = Math.round(amount * 100) / 100;
  return amount === rounded;
}
