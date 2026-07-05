import type { Response } from 'express';

/** Machine-readable error identifiers used across the service. */
export enum ErrorCode {
  BadRequest = 'BAD_REQUEST',
  Unauthorized = 'UNAUTHORIZED',
  Forbidden = 'FORBIDDEN',
  NotFound = 'NOT_FOUND',
  Conflict = 'CONFLICT',
  ValidationFailed = 'VALIDATION_FAILED',
  InternalError = 'INTERNAL_ERROR',
}

/** Consistent success/error envelope shared with the gateway service. */
export interface Envelope<T> {
  success: boolean;
  data?: T;
  error?: { code: ErrorCode; message: string };
}

export function sendSuccess<T>(res: Response, status: number, data: T): void {
  const body: Envelope<T> = { success: true, data };
  res.status(status).json(body);
}

export function sendError(
  res: Response,
  status: number,
  code: ErrorCode,
  message: string,
): void {
  const body: Envelope<never> = { success: false, error: { code, message } };
  res.status(status).json(body);
}
