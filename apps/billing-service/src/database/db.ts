import { Pool } from 'pg';

// SECURITY: Connection config loaded from environment, never hardcoded
const pool = new Pool({
  connectionString: process.env.DATABASE_URL,
  max: 10,                      // Max connections in pool
  idleTimeoutMillis: 30000,     // Close idle connections after 30s
  connectionTimeoutMillis: 5000, // Fail fast if can't connect in 5s
  statement_timeout: 10000,     // Kill queries running > 10s (DoS protection)
});

export default pool;
