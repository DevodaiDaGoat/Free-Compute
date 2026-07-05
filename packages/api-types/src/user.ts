export interface User {
  id: string;
  email: string;
  verified: boolean;
  credits: number;
  created_at: string;
  updated_at: string;
}

export type PublicUser = Pick<User, "id" | "email" | "created_at">;
