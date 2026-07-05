/**
 * Runtime configuration loaded from environment variables with sensible defaults.
 */
export interface Config {
  port: number;
  jwtSecret: string;
  jwtExpiresIn: string;
  bcryptRounds: number;
  allowedOrigins: string[];
}

function envStr(key: string, fallback: string): string {
  const v = process.env[key];
  return v && v.length > 0 ? v : fallback;
}

function envInt(key: string, fallback: number): number {
  const v = process.env[key];
  if (v && v.length > 0) {
    const n = Number.parseInt(v, 10);
    if (!Number.isNaN(n)) return n;
  }
  return fallback;
}

export function loadConfig(): Config {
  return {
    port: envInt('PORT', 8081),
    jwtSecret: envStr('JWT_SECRET', 'dev-insecure-secret-change-me'),
    jwtExpiresIn: envStr('JWT_EXPIRES_IN', '1h'),
    bcryptRounds: envInt('BCRYPT_ROUNDS', 10),
    allowedOrigins: envStr('ALLOWED_ORIGINS', 'http://localhost:3000')
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean),
  };
}
