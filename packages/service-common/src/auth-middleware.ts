import type { Request, Response, NextFunction } from "express";
import jwt from "jsonwebtoken";
import type { JWTPayload } from "@freecompute/api-types";
import { AppError } from "./error-handler";

declare module "express" {
  interface Request {
    userId?: string;
    userClaims?: JWTPayload;
  }
}

export function authMiddleware(jwtSecret: string) {
  return (req: Request, _res: Response, next: NextFunction): void => {
    const header = req.headers.authorization;
    if (!header) {
      next(AppError.unauthorized("missing authorization header"));
      return;
    }

    const parts = header.split(" ");
    if (parts.length !== 2 || parts[0] !== "Bearer") {
      next(AppError.unauthorized("invalid authorization format"));
      return;
    }

    try {
      const payload = jwt.verify(parts[1], jwtSecret) as JWTPayload;
      req.userId = payload.user_id;
      req.userClaims = payload;
      next();
    } catch {
      next(AppError.unauthorized("invalid or expired token"));
    }
  };
}

export function getUserId(req: Request): string {
  if (!req.userId) throw AppError.unauthorized();
  return req.userId;
}

export function getUserClaims(req: Request): JWTPayload {
  if (!req.userClaims) throw AppError.unauthorized();
  return req.userClaims;
}
