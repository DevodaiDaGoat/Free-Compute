import { Request, Response, NextFunction } from 'express';

// SECURITY: Global error handler that never exposes internal details.
// Internal errors are logged server-side; clients get generic messages.
export function errorHandler(err: Error, _req: Request, res: Response, _next: NextFunction): void {
  // Log full error internally
  console.error('Unhandled error:', {
    message: err.message,
    stack: err.stack,
    timestamp: new Date().toISOString(),
  });

  // SECURITY: Never send stack traces or internal error details to clients
  res.status(500).json({ error: 'Internal server error' });
}
