import type { NextFunction, Request, RequestHandler, Response } from "express";

/**
 * Wraps an async route handler so rejected promises are forwarded to Express's
 * error pipeline via `next(err)` instead of becoming unhandled rejections.
 *
 * Without this, a `throw`/`reject` inside an `async` handler is silently
 * swallowed by Express (which only catches synchronous throws), leaving the
 * request hanging until it times out. This is the single most common way
 * errors get lost in Express services.
 */
export function asyncHandler(
  handler: (req: Request, res: Response, next: NextFunction) => Promise<unknown>,
): RequestHandler {
  return (req, res, next) => {
    handler(req, res, next).catch(next);
  };
}
