import type { NextFunction, Request, RequestHandler, Response } from "express";

import type { Session } from "../models/Session";
import type { SessionService } from "../services/sessionService";
import { UnauthorizedError } from "../utils/errors";

declare global {
  // eslint-disable-next-line @typescript-eslint/no-namespace
  namespace Express {
    interface Request {
      session?: Session;
    }
  }
}

function extractBearer(header: string | undefined): string {
  if (!header || !header.startsWith("Bearer ")) {
    throw new UnauthorizedError("Missing or malformed Authorization header");
  }
  const token = header.slice("Bearer ".length).trim();
  if (token.length === 0) {
    throw new UnauthorizedError("Empty bearer token");
  }
  return token;
}

/**
 * Authentication middleware. Rejects unauthenticated requests with a 401 and
 * attaches the resolved session to the request for downstream handlers.
 */
export function requireAuth(sessions: SessionService): RequestHandler {
  return (req: Request, _res: Response, next: NextFunction) => {
    Promise.resolve()
      .then(async () => {
        const token = extractBearer(req.header("authorization") ?? undefined);
        req.session = await sessions.authenticate(token);
        next();
      })
      .catch(next);
  };
}
