import { randomUUID } from 'node:crypto';

import type { Session } from '../models/Session';

/**
 * SessionService tracks issued sessions and supports revocation (logout).
 * This skeleton keeps sessions in memory; swap the map for Redis/Postgres later.
 */
export class SessionService {
  private readonly sessions = new Map<string, Session>();

  create(userId: string, token: string, ttlSeconds: number): Session {
    const now = new Date();
    const session: Session = {
      id: randomUUID(),
      userId,
      token,
      createdAt: now,
      expiresAt: new Date(now.getTime() + ttlSeconds * 1000),
      revoked: false,
    };
    this.sessions.set(session.id, session);
    return session;
  }

  revoke(token: string): boolean {
    for (const session of this.sessions.values()) {
      if (session.token === token && !session.revoked) {
        session.revoked = true;
        return true;
      }
    }
    return false;
  }
}
