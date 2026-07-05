export interface RegisterRequest {
  email: string;
  password: string;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface AuthTokens {
  access_token: string;
  refresh_token?: string;
  expires_in: number;
}

export interface VerifyEmailRequest {
  token: string;
}

export interface JWTPayload {
  user_id: string;
  email: string;
  iat: number;
  exp: number;
}
