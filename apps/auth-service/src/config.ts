import { AppError } from "./utils/errors";

export interface Config {
  port: number;
  nodeEnv: "development" | "production" | "test";
  jwtSecret: string;
  accessTokenTtlSeconds: number;
  sessionTtlSeconds: number;
}

function requireSecret(name: string, fallbackForDev: string, isProd: boolean): string {
  const value = process.env[name];
  if (value && value.length > 0) return value;
  if (isProd) {
    // Fail fast at boot rather than starting with an insecure default that
    // would silently weaken auth in production.
    throw new AppError(`Missing required environment variable ${name}`, {
      code: "INTERNAL",
      isOperational: false,
    });
  }
  return fallbackForDev;
}

function parseIntEnv(name: string, fallback: number): number {
  const raw = process.env[name];
  if (raw === undefined || raw === "") return fallback;
  const parsed = Number(raw);
  if (!Number.isInteger(parsed) || parsed <= 0) {
    throw new AppError(`Environment variable ${name} must be a positive integer`, {
      code: "INTERNAL",
      isOperational: false,
    });
  }
  return parsed;
}

export function loadConfig(): Config {
  const nodeEnv = (process.env.NODE_ENV as Config["nodeEnv"]) ?? "development";
  const isProd = nodeEnv === "production";
  return {
    port: parseIntEnv("PORT", 4001),
    nodeEnv,
    jwtSecret: requireSecret("JWT_SECRET", "dev-insecure-secret", isProd),
    accessTokenTtlSeconds: parseIntEnv("ACCESS_TOKEN_TTL_SECONDS", 900),
    sessionTtlSeconds: parseIntEnv("SESSION_TTL_SECONDS", 60 * 60 * 24 * 7),
  };
}
