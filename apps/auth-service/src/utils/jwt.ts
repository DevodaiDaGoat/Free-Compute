import jwt from 'jsonwebtoken';

interface AccessTokenPayload {
  sub: string;
  role: string;
  exp: number;
}

interface RefreshTokenPayload {
  sub: string;
  role: string;
  exp: number;
  type: 'refresh';
}

export class JWTUtil {
  // SECURITY: Keys loaded from environment/secrets manager — never hardcoded
  private readonly privateKey: string;
  private readonly publicKey: string;
  private readonly issuer = 'freecompute.io';
  private readonly audience = 'freecompute-api';

  constructor() {
    const privateKey = process.env.JWT_PRIVATE_KEY;
    const publicKey = process.env.JWT_PUBLIC_KEY;

    if (!privateKey || !publicKey) {
      throw new Error('JWT_PRIVATE_KEY and JWT_PUBLIC_KEY environment variables are required');
    }

    this.privateKey = privateKey;
    this.publicKey = publicKey;
  }

  // SECURITY: Use RS256 (asymmetric) — separates signing from verification
  signAccessToken(userId: string, role: string): string {
    return jwt.sign(
      { sub: userId, role },
      this.privateKey,
      {
        algorithm: 'RS256',
        expiresIn: '15m', // Short-lived access token
        issuer: this.issuer,
        audience: this.audience,
      }
    );
  }

  signRefreshToken(userId: string): string {
    return jwt.sign(
      { sub: userId, type: 'refresh' },
      this.privateKey,
      {
        algorithm: 'RS256',
        expiresIn: '7d',
        issuer: this.issuer,
        audience: this.audience,
      }
    );
  }

  signConnectionToken(userId: string, vmId: string): string {
    return jwt.sign(
      { sub: userId, vm_id: vmId },
      this.privateKey,
      {
        algorithm: 'RS256',
        expiresIn: '30s', // Very short-lived — single use for WS upgrade
        issuer: this.issuer,
        audience: 'freecompute-stream',
      }
    );
  }

  verifyAccessToken(token: string): AccessTokenPayload {
    // SECURITY: Explicitly specify allowed algorithms to prevent algorithm confusion
    const decoded = jwt.verify(token, this.publicKey, {
      algorithms: ['RS256'],
      issuer: this.issuer,
      audience: this.audience,
    }) as AccessTokenPayload;

    return decoded;
  }

  verifyRefreshToken(token: string): RefreshTokenPayload {
    const decoded = jwt.verify(token, this.publicKey, {
      algorithms: ['RS256'],
      issuer: this.issuer,
      audience: this.audience,
    }) as RefreshTokenPayload;

    if (decoded.type !== 'refresh') {
      throw new Error('Invalid token type');
    }

    return decoded;
  }
}
