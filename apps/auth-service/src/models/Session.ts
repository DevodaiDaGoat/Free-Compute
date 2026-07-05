import { randomUUID } from "node:crypto";

export interface Session {
  id: string;
  userId: string;
  createdAt: string;
  expiresAt: string;
}

/**
 * In-memory session store standing in for a Redis-backed session table.
 */
export class SessionRepository {
  private readonly byId = new Map<string, Session>();

  async create(userId: string, ttlSeconds: number): Promise<Session> {
    const now = Date.now();
    const session: Session = {
      id: randomUUID(),
      userId,
      createdAt: new Date(now).toISOString(),
      expiresAt: new Date(now + ttlSeconds * 1000).toISOString(),
    };
    this.byId.set(session.id, session);
    return session;
  }

  async findById(id: string): Promise<Session | null> {
    const session = this.byId.get(id);
    if (!session) return null;
    if (Date.parse(session.expiresAt) <= Date.now()) {
      this.byId.delete(id);
      return null;
    }
    return session;
  }

  async delete(id: string): Promise<boolean> {
    return this.byId.delete(id);
  }
}
