import { Pool, type PoolConfig as PgPoolConfig } from "pg";

export interface PoolConfig {
  connectionString: string;
  max?: number;
  min?: number;
  idleTimeoutMillis?: number;
  connectionTimeoutMillis?: number;
}

const DEFAULT_CONFIG: Partial<PoolConfig> = {
  max: 20,
  min: 5,
  idleTimeoutMillis: 30_000,
  connectionTimeoutMillis: 5_000,
};

let pool: Pool | null = null;

export function createPool(config: PoolConfig): Pool {
  if (pool) return pool;

  const pgConfig: PgPoolConfig = {
    connectionString: config.connectionString,
    max: config.max ?? DEFAULT_CONFIG.max,
    min: config.min ?? DEFAULT_CONFIG.min,
    idleTimeoutMillis:
      config.idleTimeoutMillis ?? DEFAULT_CONFIG.idleTimeoutMillis,
    connectionTimeoutMillis:
      config.connectionTimeoutMillis ?? DEFAULT_CONFIG.connectionTimeoutMillis,
  };

  pool = new Pool(pgConfig);

  pool.on("error", (err) => {
    console.error("unexpected database pool error", err);
  });

  return pool;
}
