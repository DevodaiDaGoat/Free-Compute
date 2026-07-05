import { randomUUID } from "node:crypto";

export interface User {
  id: string;
  email: string;
  passwordHash: string;
  verified: boolean;
  createdAt: string;
  updatedAt: string;
}

/**
 * In-memory user store standing in for the `users` table described in
 * docs/BACKEND_STRUCTURE.md. The async surface mirrors a real DB driver so
 * swapping in PostgreSQL later requires no call-site changes.
 */
export class UserRepository {
  private readonly byId = new Map<string, User>();
  private readonly idByEmail = new Map<string, string>();

  async create(email: string, passwordHash: string): Promise<User> {
    const now = new Date().toISOString();
    const user: User = {
      id: randomUUID(),
      email,
      passwordHash,
      verified: false,
      createdAt: now,
      updatedAt: now,
    };
    this.byId.set(user.id, user);
    this.idByEmail.set(email.toLowerCase(), user.id);
    return user;
  }

  async findByEmail(email: string): Promise<User | null> {
    const id = this.idByEmail.get(email.toLowerCase());
    if (id === undefined) return null;
    return this.byId.get(id) ?? null;
  }

  async findById(id: string): Promise<User | null> {
    return this.byId.get(id) ?? null;
  }

  async markVerified(id: string): Promise<User | null> {
    const user = this.byId.get(id);
    if (!user) return null;
    user.verified = true;
    user.updatedAt = new Date().toISOString();
    return user;
  }
}
