import { randomUUID } from "node:crypto";

import type { User, UserRepository } from "../models/User";
import { ConflictError, InvalidCredentialsError, NotFoundError } from "../utils/errors";
import { logger } from "../utils/logger";
import { hashPassword, verifyPassword } from "../utils/password";
import type { EmailService } from "./emailService";
import type { IssuedTokens, SessionService } from "./sessionService";

export interface PublicUser {
  id: string;
  email: string;
  verified: boolean;
}

function toPublic(user: User): PublicUser {
  return { id: user.id, email: user.email, verified: user.verified };
}

export interface AuthResult {
  user: PublicUser;
  tokens: IssuedTokens;
}

export class AuthService {
  constructor(
    private readonly users: UserRepository,
    private readonly sessions: SessionService,
    private readonly email: EmailService,
  ) {}

  async register(email: string, password: string): Promise<AuthResult> {
    const existing = await this.users.findByEmail(email);
    if (existing) {
      // Propagate a precise 409 rather than letting a duplicate slip through.
      throw new ConflictError("An account with this email already exists");
    }

    const passwordHash = await hashPassword(password);
    const user = await this.users.create(email, passwordHash);

    // Verification email is best-effort: log failures but don't fail the
    // registration, and never swallow the error without a trace.
    const verificationToken = randomUUID();
    try {
      await this.email.sendVerification(user.email, verificationToken);
    } catch (err) {
      logger.error("Failed to send verification email", { err, userId: user.id });
    }

    const tokens = await this.sessions.issue(user.id);
    return { user: toPublic(user), tokens };
  }

  async login(email: string, password: string): Promise<AuthResult> {
    const user = await this.users.findByEmail(email);
    // Use the same error for "no such user" and "wrong password" to avoid
    // leaking which emails are registered (user enumeration).
    if (!user) {
      throw new InvalidCredentialsError();
    }
    const ok = await verifyPassword(password, user.passwordHash);
    if (!ok) {
      throw new InvalidCredentialsError();
    }
    const tokens = await this.sessions.issue(user.id);
    return { user: toPublic(user), tokens };
  }

  async verify(userId: string): Promise<PublicUser> {
    const user = await this.users.markVerified(userId);
    if (!user) {
      throw new NotFoundError("User not found");
    }
    return toPublic(user);
  }

  async logout(sessionId: string): Promise<void> {
    await this.sessions.revoke(sessionId);
  }
}
