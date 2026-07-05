import type { ErrorRequestHandler, NextFunction, Request, Response } from "express";

import { AppError, NotFoundError } from "../utils/errors";
import { logger } from "../utils/logger";

interface ErrorBody {
  error: {
    code: string;
    message: string;
    details?: unknown;
    requestId?: string;
  };
}

/** 404 handler for unmatched routes; hands off to the error pipeline. */
export function notFoundHandler(req: Request, _res: Response, next: NextFunction): void {
  next(new NotFoundError(`Route not found: ${req.method} ${req.path}`));
}

/**
 * Central error handler — the single place errors are turned into responses.
 *
 * - Normalizes any thrown value into an AppError (nothing reaches the client raw).
 * - Logs operational errors at warn and unexpected faults at error, always with
 *   the full cause chain, so nothing is silently swallowed.
 * - Never leaks internal messages/stacks: 5xx responses return a generic message
 *   in production while the real detail stays in the logs.
 */
export const errorHandler: ErrorRequestHandler = (err, req, res, _next) => {
  const appError = AppError.from(err);
  const requestId = (req as { id?: string }).id;

  const logMeta = {
    err: appError,
    requestId,
    method: req.method,
    path: req.path,
    statusCode: appError.statusCode,
    code: appError.code,
  };
  if (appError.statusCode >= 500 || !appError.isOperational) {
    logger.error("Request failed", logMeta);
  } else {
    logger.warn("Request rejected", logMeta);
  }

  // If headers were already sent, delegate to Express's default handler so we
  // don't attempt to write a second response.
  if (res.headersSent) return;

  const exposeMessage = appError.statusCode < 500;
  const body: ErrorBody = {
    error: {
      code: appError.code,
      message: exposeMessage ? appError.message : "Internal server error",
      requestId,
    },
  };
  if (exposeMessage && appError.details !== undefined) {
    body.error.details = appError.details;
  }

  res.status(appError.statusCode).json(body);
};
