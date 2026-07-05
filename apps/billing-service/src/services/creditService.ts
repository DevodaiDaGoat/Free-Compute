import { Pool } from 'pg';

// SECURITY: Credit operations MUST be atomic to prevent race conditions.
// Use single-statement UPDATE with WHERE clause for deductions.

const pool = new Pool({
  connectionString: process.env.DATABASE_URL,
  max: 10,
  idleTimeoutMillis: 30000,
  connectionTimeoutMillis: 5000,
});

export class CreditService {
  /**
   * Get current credit balance for a user.
   */
  async getBalance(userId: string): Promise<number> {
    // SECURITY: Parameterized query — never interpolate user input
    const result = await pool.query(
      'SELECT credits FROM users WHERE id = $1',
      [userId]
    );

    if (result.rows.length === 0) {
      return 0;
    }

    return result.rows[0].credits;
  }

  /**
   * Atomically deduct credits from a user's balance.
   * Returns new balance on success, null if insufficient credits.
   *
   * SECURITY: This uses a single atomic UPDATE with a WHERE clause
   * to prevent race conditions (double-spending). The DB enforces
   * that credits >= amount before deducting.
   */
  async deduct(
    userId: string,
    amount: number,
    reason: string,
    idempotencyKey: string
  ): Promise<number | null> {
    const client = await pool.connect();
    try {
      await client.query('BEGIN');

      // Check idempotency — prevent duplicate deductions
      const existing = await client.query(
        'SELECT id FROM transactions WHERE idempotency_key = $1',
        [idempotencyKey]
      );
      if (existing.rows.length > 0) {
        await client.query('ROLLBACK');
        // Return current balance for idempotent response
        return this.getBalance(userId);
      }

      // SECURITY: Atomic deduction — single statement prevents TOCTOU race
      const result = await client.query(
        `UPDATE users
         SET credits = credits - $1, updated_at = NOW()
         WHERE id = $2 AND credits >= $1
         RETURNING credits`,
        [amount, userId]
      );

      if (result.rows.length === 0) {
        // Insufficient credits (the WHERE clause prevented the update)
        await client.query('ROLLBACK');
        return null;
      }

      // Record the transaction
      await client.query(
        `INSERT INTO transactions (id, user_id, type, amount, description, idempotency_key, created_at)
         VALUES (gen_random_uuid(), $1, 'spend', $2, $3, $4, NOW())`,
        [userId, -amount, reason, idempotencyKey]
      );

      await client.query('COMMIT');
      return result.rows[0].credits;
    } catch (err) {
      await client.query('ROLLBACK');
      throw err;
    } finally {
      client.release();
    }
  }

  /**
   * Add credits to a user's balance (purchase, reward, refund).
   */
  async add(
    userId: string,
    amount: number,
    reason: string,
    idempotencyKey: string
  ): Promise<number> {
    const client = await pool.connect();
    try {
      await client.query('BEGIN');

      // Check idempotency
      const existing = await client.query(
        'SELECT id FROM transactions WHERE idempotency_key = $1',
        [idempotencyKey]
      );
      if (existing.rows.length > 0) {
        await client.query('ROLLBACK');
        return this.getBalance(userId);
      }

      // Add credits
      const result = await client.query(
        `UPDATE users
         SET credits = credits + $1, updated_at = NOW()
         WHERE id = $2
         RETURNING credits`,
        [amount, userId]
      );

      if (result.rows.length === 0) {
        await client.query('ROLLBACK');
        throw new Error('User not found');
      }

      // Record transaction
      await client.query(
        `INSERT INTO transactions (id, user_id, type, amount, description, idempotency_key, created_at)
         VALUES (gen_random_uuid(), $1, 'purchase', $2, $3, $4, NOW())`,
        [userId, amount, reason, idempotencyKey]
      );

      await client.query('COMMIT');
      return result.rows[0].credits;
    } catch (err) {
      await client.query('ROLLBACK');
      throw err;
    } finally {
      client.release();
    }
  }
}
