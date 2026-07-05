import type { Request, Response, NextFunction } from "express";

export class AppError extends Error {
  constructor(
    public readonly statusCode: number,
    message: string,
    public readonly detail?: string,
  ) {
    super(message);
    this.name = "AppError";
  }

  static badRequest(msg: string, detail?: string): AppError {
    return new AppError(400, msg, detail);
  }

  static unauthorized(msg = "unauthorized"): AppError {
    return new AppError(401, msg);
  }

  static forbidden(msg = "forbidden"): AppError {
    return new AppError(403, msg);
  }

  static notFound(msg = "not found"): AppError {
    return new AppError(404, msg);
  }

  static conflict(msg: string): AppError {
    return new AppError(409, msg);
  }

  static internal(msg = "internal server error"): AppError {
    return new AppError(500, msg);
  }
}

export function errorHandler(
  err: Error,
  _req: Request,
  res: Response,
  _next: NextFunction,
): void {
  if (err instanceof AppError) {
    res.status(err.statusCode).json({
      code: err.statusCode,
      message: err.message,
      detail: err.detail,
    });
    return;
  }

  console.error("unhandled error:", err);
  res.status(500).json({
    code: 500,
    message: "internal server error",
  });
}
