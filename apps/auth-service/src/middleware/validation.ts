import type { NextFunction, Request, Response } from 'express';
import type { ZodTypeAny, infer as ZodInfer } from 'zod';

import { ErrorCode, sendError } from '../utils/response';

/**
 * Returns middleware that validates `req.body` against the given Zod schema.
 * On success the parsed value replaces `req.body`; on failure a 400 is returned.
 */
export function validateBody<S extends ZodTypeAny>(schema: S) {
  return (req: Request, res: Response, next: NextFunction): void => {
    const result = schema.safeParse(req.body);
    if (!result.success) {
      const message = result.error.issues
        .map((i) => `${i.path.join('.') || 'body'}: ${i.message}`)
        .join('; ');
      sendError(res, 400, ErrorCode.ValidationFailed, message);
      return;
    }
    req.body = result.data as ZodInfer<S>;
    next();
  };
}
