import type { NextFunction, Request, Response } from 'express';

import { ErrorCode, sendError } from '../utils/response';

/** 404 handler for unmatched routes. */
export function notFoundHandler(_req: Request, res: Response): void {
  sendError(res, 404, ErrorCode.NotFound, 'resource not found');
}

/**
 * Central error handler. Express identifies this by its four-argument
 * signature, so `next` must remain in the parameter list.
 */
export function errorHandler(
  err: unknown,
  _req: Request,
  res: Response,
  _next: NextFunction,
): void {
  const message = err instanceof Error ? err.message : 'unexpected error';
  console.error('[error]', message);
  sendError(res, 500, ErrorCode.InternalError, 'internal server error');
}
