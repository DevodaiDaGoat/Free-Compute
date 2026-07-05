import { randomUUID } from 'node:crypto';

import type { Config } from '../config';
import { type PublicUser, type User, toPublicUser } from '../models/User';
import { hashPassword, verifyPassword } from '../utils/password';
import { signAccessToken, verifyAccessToken } from '../utils/jwt';

export class AuthError extends Error {
  constructor(
    public readonly code: 'CONFLICT' | 'UNAUTHORIZED' | 'NOT_FOUND',
    message: string,
  ) {
    super(message);
    this.name = 'AuthError';
  }
}

/**
 * AuthService owns user registration, credential verification and token
 * issuance. This skeleton uses an in-memory store; a database-backed
 * implementation can replace the internal map without changing the interface.
 */
export class AuthService {
  private readonly usersByEmail = new Map<string, User>();

  constructor(private readonly config: Config) {}

  async register(email: string, password: string): Promise<PublicUser> {
    const key = email.toLowerCase();
    if (this.usersByEmail.has(key)) {
      throw new AuthError('CONFLICT', 'email already registered');
    }
    const now = new Date();
    const user: User = {
      id: randomUUID(),
      email: key,
      passwordHash: await hashPassword(password, this.config.bcryptRounds),
      verified: false,
      credits: 0,
      createdAt: now,
      updatedAt: now,
    };
    this.usersByEmail.set(key, user);
    return toPublicUser(user);
  }

  async login(email: string, password: string): Promise<{ token: string; user: PublicUser }> {
    const user = this.usersByEmail.get(email.toLowerCase());
    if (!user || !(await verifyPassword(password, user.passwordHash))) {
      throw new AuthError('UNAUTHORIZED', 'invalid credentials');
    }
    const token = signAccessToken(
      { sub: user.id, email: user.email },
      this.config.jwtSecret,
      this.config.jwtExpiresIn,
    );
    return { token, user: toPublicUser(user) };
  }

  verify(token: string): { valid: boolean; userId?: string } {
    try {
      const claims = verifyAccessToken(token, this.config.jwtSecret);
      return { valid: true, userId: claims.sub };
    } catch {
      return { valid: false };
    }
  }
}
