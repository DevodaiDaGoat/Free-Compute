export interface User {
  id: string;
  email: string;
  password_hash: string;
  verified: boolean;
  role: 'user' | 'admin';
  credits: number;
  created_at: Date;
  updated_at: Date;
}

export interface CreateUserInput {
  email: string;
  password_hash: string;
  verification_token: string;
}
