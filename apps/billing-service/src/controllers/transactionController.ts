import { Request, Response, NextFunction } from 'express';
import { z } from 'zod';

const createTransactionSchema = z.object({
  user_id: z.string().uuid(),
  type: z.enum(['purchase', 'spend', 'reward', 'refund']),
  amount: z.number().int(),
  description: z.string().min(1).max(500),
  idempotency_key: z.string().uuid(),
}).strict();

export async function listTransactions(req: Request, res: Response, next: NextFunction): Promise<void> {
  try {
    const userId = req.params.user_id;
    const uuidResult = z.string().uuid().safeParse(userId);
    if (!uuidResult.success) {
      res.status(400).json({ error: 'Invalid user ID' });
      return;
    }

    // TODO: Query transactions from database with pagination
    // SELECT * FROM transactions WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3
    res.json({ transactions: [] });
  } catch (err) {
    next(err);
  }
}

export async function createTransaction(req: Request, res: Response, next: NextFunction): Promise<void> {
  try {
    const result = createTransactionSchema.safeParse(req.body);
    if (!result.success) {
      res.status(400).json({ error: 'Validation failed', details: result.error.issues });
      return;
    }

    // TODO: Insert transaction record
    res.status(201).json({ message: 'Transaction recorded' });
  } catch (err) {
    next(err);
  }
}
