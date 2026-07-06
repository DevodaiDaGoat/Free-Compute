import jwt, { type SignOptions } from 'jsonwebtoken';

export interface AccessTokenClaims {
  sub: string; // user ID
  email: string;
}

/** Signs a JWT access token for the given claims. */
export function signAccessToken(
  claims: AccessTokenClaims,
  secret: string,
  expiresIn: string,
): string {
  const options: SignOptions = { expiresIn: expiresIn as SignOptions['expiresIn'] };
  return jwt.sign(claims, secret, options);
}

/** Verifies a JWT and returns its claims, or throws if invalid/expired. */
export function verifyAccessToken(token: string, secret: string): AccessTokenClaims {
  const decoded = jwt.verify(token, secret);
  if (typeof decoded === 'string' || !decoded.sub) {
    throw new Error('invalid token payload');
  }
  return { sub: String(decoded.sub), email: String(decoded.email ?? '') };
}
