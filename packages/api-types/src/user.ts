export interface User {
  id: string;
  email: string;
  verified: boolean;
  credits: number;
  createdAt: string; // ISO 8601
  updatedAt: string;
}
