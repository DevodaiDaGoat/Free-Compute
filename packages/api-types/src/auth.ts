// Registration
export interface RegisterRequest {
  email: string;
  password: string;
}
export interface RegisterResponse {
  userId: string;
  message: string;
}

// Login
export interface LoginRequest {
  email: string;
  password: string;
}
export interface LoginResponse {
  token: string;
  user: UserSummary;
}

// Verify
export interface VerifyRequest {
  code: string;
  userId: string;
}
export interface VerifyResponse {
  verified: boolean;
}

export interface UserSummary {
  id: string;
  email: string;
  verified: boolean;
}
