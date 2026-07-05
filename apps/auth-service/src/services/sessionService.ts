import type { Config } from "../config";
import type { Session, SessionRepository } from "../models/Session";
import { NotFoundError, UnauthorizedError } from "../utils/errors";
import { signAccessToken, verifyAccessToken } from "../utils/jwt";

export interface IssuedTokens {
  accessToken: string;
  session: Session;
}

export class SessionService {
  constructor(
    private readonly sessions: SessionRepository,
    private readonly config: Config,
  ) {}

  async issue(userId: string): Promise<IssuedTokens> {
    const session = await this.sessions.create(userId, this.config.sessionTtlSeconds);
    const accessToken = signAccessToken(
      { sub: userId, sid: session.id },
      this.config.jwtSecret,
      this.config.accessTokenTtlSeconds,
    );
    return { accessToken, session };
  }

  /**
   * Resolve the active session behind an access token. Distinguishes a bad
   * token (401) from a token whose session has been revoked/expired (401),
   * and never returns a partially-valid state.
   */
  async authenticate(accessToken: string): Promise<Session> {
    const claims = verifyAccessToken(accessToken, this.config.jwtSecret);
    const session = await this.sessions.findById(claims.sid);
    if (!session) {
      throw new UnauthorizedError("Session expired or revoked");
    }
    return session;
  }

  async revoke(sessionId: string): Promise<void> {
    const deleted = await this.sessions.delete(sessionId);
    if (!deleted) {
      throw new NotFoundError("Session not found");
    }
  }
}
