/** Persisted user record. Mirrors the `users` table in the schema overview. */
export interface User {
  id: string;
  email: string;
  passwordHash: string;
  verified: boolean;
  credits: number;
  createdAt: Date;
  updatedAt: Date;
}

/** Public user shape returned to clients (never exposes the password hash). */
export interface PublicUser {
  id: string;
  email: string;
  verified: boolean;
  credits: number;
}

export function toPublicUser(user: User): PublicUser {
  return {
    id: user.id,
    email: user.email,
    verified: user.verified,
    credits: user.credits,
  };
}
