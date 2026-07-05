import jwt from "jsonwebtoken";

import { UnauthorizedError } from "./errors";

export interface AccessTokenClaims {
  sub: string;
  sid: string;
}

export function signAccessToken(
  claims: AccessTokenClaims,
  secret: string,
  ttlSeconds: number,
): string {
  return jwt.sign(claims, secret, { expiresIn: ttlSeconds });
}

/**
 * Verify and decode an access token. `jsonwebtoken` throws on invalid/expired
 * tokens; we translate those into a 401 `UnauthorizedError` (preserving the
 * original as `cause`) instead of letting a raw library error escape as a 500.
 */
export function verifyAccessToken(token: string, secret: string): AccessTokenClaims {
  try {
    const decoded = jwt.verify(token, secret);
    if (
      typeof decoded !== "object" ||
      decoded === null ||
      typeof (decoded as Record<string, unknown>).sub !== "string" ||
      typeof (decoded as Record<string, unknown>).sid !== "string"
    ) {
      throw new UnauthorizedError("Malformed token payload");
    }
    const claims = decoded as Record<string, unknown>;
    return { sub: claims.sub as string, sid: claims.sid as string };
  } catch (err) {
    if (err instanceof UnauthorizedError) throw err;
    throw new UnauthorizedError("Invalid or expired token", { cause: err });
  }
}
