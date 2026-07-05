/**
 * Application error types.
 *
 * Every error that flows out of the service is normalized into an `AppError`
 * so the central error handler can map it to a stable HTTP status, an
 * error code the frontend can switch on, and a safe client message — while
 * still preserving the original cause for logging.
 */

export type ErrorCode =
  | "BAD_REQUEST"
  | "VALIDATION_FAILED"
  | "UNAUTHORIZED"
  | "INVALID_CREDENTIALS"
  | "FORBIDDEN"
  | "NOT_FOUND"
  | "CONFLICT"
  | "RATE_LIMITED"
  | "INTERNAL";

export interface AppErrorOptions {
  /** Machine-readable code the client can branch on. */
  code?: ErrorCode;
  /** HTTP status to respond with. */
  statusCode?: number;
  /** The underlying error, preserved for logging (never sent to clients). */
  cause?: unknown;
  /** Structured details safe to expose (e.g. which fields failed validation). */
  details?: unknown;
  /**
   * Whether this represents an expected, handled condition (true) versus an
   * unexpected programming/infra fault (false). Non-operational errors are
   * logged at a higher severity and never leak their message to clients.
   */
  isOperational?: boolean;
}

export class AppError extends Error {
  readonly code: ErrorCode;
  readonly statusCode: number;
  readonly details?: unknown;
  readonly isOperational: boolean;
  readonly cause?: unknown;

  constructor(message: string, options: AppErrorOptions = {}) {
    super(message);
    this.name = new.target.name;
    this.code = options.code ?? "INTERNAL";
    this.statusCode = options.statusCode ?? 500;
    this.details = options.details;
    this.isOperational = options.isOperational ?? true;
    this.cause = options.cause;
    // Preserve a proper prototype chain when targeting ES5-ish runtimes.
    Object.setPrototypeOf(this, new.target.prototype);
    Error.captureStackTrace?.(this, new.target);
  }

  /**
   * Normalize any thrown value into an AppError without discarding context.
   * Unknown errors become non-operational 500s that keep the original as `cause`.
   */
  static from(err: unknown): AppError {
    if (err instanceof AppError) return err;
    if (err instanceof Error) {
      return new AppError(err.message, {
        code: "INTERNAL",
        statusCode: 500,
        cause: err,
        isOperational: false,
      });
    }
    return new AppError("Unexpected error", {
      code: "INTERNAL",
      statusCode: 500,
      cause: err,
      isOperational: false,
    });
  }
}

export class BadRequestError extends AppError {
  constructor(message = "Bad request", options: AppErrorOptions = {}) {
    super(message, { code: "BAD_REQUEST", statusCode: 400, ...options });
  }
}

export class ValidationError extends AppError {
  constructor(message = "Validation failed", options: AppErrorOptions = {}) {
    super(message, { code: "VALIDATION_FAILED", statusCode: 422, ...options });
  }
}

export class UnauthorizedError extends AppError {
  constructor(message = "Unauthorized", options: AppErrorOptions = {}) {
    super(message, { code: "UNAUTHORIZED", statusCode: 401, ...options });
  }
}

export class InvalidCredentialsError extends AppError {
  constructor(message = "Invalid email or password", options: AppErrorOptions = {}) {
    super(message, { code: "INVALID_CREDENTIALS", statusCode: 401, ...options });
  }
}

export class ForbiddenError extends AppError {
  constructor(message = "Forbidden", options: AppErrorOptions = {}) {
    super(message, { code: "FORBIDDEN", statusCode: 403, ...options });
  }
}

export class NotFoundError extends AppError {
  constructor(message = "Not found", options: AppErrorOptions = {}) {
    super(message, { code: "NOT_FOUND", statusCode: 404, ...options });
  }
}

export class ConflictError extends AppError {
  constructor(message = "Conflict", options: AppErrorOptions = {}) {
    super(message, { code: "CONFLICT", statusCode: 409, ...options });
  }
}
