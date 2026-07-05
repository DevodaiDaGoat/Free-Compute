import { Request, Response, NextFunction } from 'express';
import { CreditService } from '../services/creditService';
import { z } from 'zod';

const creditService = new CreditService();

const deductSchema = z.object({
  user_id: z.string().uuid(),
  amount: z.number().int().positive().max(10000),
  reason: z.string().min(1).max(255),
  idempotency_key: z.string().uuid(),
}).strict();

const addSchema = z.object({
  user_id: z.string().uuid(),
  amount: z.number().int().positive().max(100000),
  reason: z.string().min(1).max(255),
  idempotency_key: z.string().uuid(),
}).strict();

export async function getBalance(req: Request, res: Response, next: NextFunction): Promise<void> {
  try {
    const userId = req.params.user_id;

    // Validate UUID format
    const uuidResult = z.string().uuid().safeParse(userId);
    if (!uuidResult.success) {
      res.status(400).json({ error: 'Invalid user ID' });
      return;
    }

    const balance = await creditService.getBalance(userId);
    res.json({ credits: balance });
  } catch (err) {
    next(err);
  }
}

export async function deductCredits(req: Request, res: Response, next: NextFunction): Promise<void> {
  try {
    const result = deductSchema.safeParse(req.body);
    if (!result.success) {
      res.status(400).json({ error: 'Validation failed', details: result.error.issues });
      return;
    }

    const { user_id, amount, reason, idempotency_key } = result.data;

    // SECURITY: Atomic deduction with idempotency
    const newBalance = await creditService.deduct(user_id, amount, reason, idempotency_key);
    if (newBalance === null) {
      res.status(402).json({ error: 'Insufficient credits' });
      return;
    }

    res.json({ credits: newBalance, deducted: amount });
  } catch (err) {
    next(err);
  }
}

export async function addCredits(req: Request, res: Response, next: NextFunction): Promise<void> {
  try {
    const result = addSchema.safeParse(req.body);
    if (!result.success) {
      res.status(400).json({ error: 'Validation failed', details: result.error.issues });
      return;
    }

    const { user_id, amount, reason, idempotency_key } = result.data;
    const newBalance = await creditService.add(user_id, amount, reason, idempotency_key);
    res.json({ credits: newBalance, added: amount });
  } catch (err) {
    next(err);
  }
}
