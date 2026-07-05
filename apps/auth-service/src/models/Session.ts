/** Persisted session record representing an issued access token. */
export interface Session {
  id: string;
  userId: string;
  token: string;
  createdAt: Date;
  expiresAt: Date;
  revoked: boolean;
}
