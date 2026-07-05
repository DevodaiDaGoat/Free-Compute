export interface Session {
  id: string;
  user_id: string;
  refresh_token_hash: string;
  ip_address: string;
  user_agent: string;
  created_at: Date;
  expires_at: Date;
  revoked: boolean;
}
