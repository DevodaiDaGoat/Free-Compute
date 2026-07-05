import { Request, Response, NextFunction } from 'express';
import { z, ZodSchema } from 'zod';

// SECURITY: Strict input validation using Zod schemas.
// All schemas use .strict() to reject unexpected fields.

export const registerSchema = z.object({
  email: z.string().email().max(254),
  password: z.string().min(8).max(128),
}).strict();

export const loginSchema = z.object({
  email: z.string().email().max(254),
  password: z.string().min(1).max(128),
}).strict();

export const verifySchema = z.object({
  token: z.string().uuid(),
}).strict();

// Generic validation middleware factory
export function validate(schema: ZodSchema) {
  return (req: Request, res: Response, next: NextFunction): void => {
    const result = schema.safeParse(req.body);

    if (!result.success) {
      const errors = result.error.issues.map((issue) => ({
        field: issue.path.join('.'),
        message: issue.message,
      }));

      res.status(400).json({ error: 'Validation failed', details: errors });
      return;
    }

    // Replace body with parsed (sanitized) data
    req.body = result.data;
    next();
  };
}
