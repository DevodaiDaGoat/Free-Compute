import { JWTUtil } from '../utils/jwt';

export class SessionService {
  private jwtUtil = new JWTUtil();

  async validate(token: string): Promise<{ user_id: string; role: string; expires_at: number }> {
    const decoded = this.jwtUtil.verifyAccessToken(token);
    return {
      user_id: decoded.sub,
      role: decoded.role,
      expires_at: decoded.exp,
    };
  }

  async refresh(refreshToken: string): Promise<{ access_token: string; refresh_token: string }> {
    // SECURITY: Implement refresh token rotation
    // 1. Verify the refresh token
    const decoded = this.jwtUtil.verifyRefreshToken(refreshToken);

    // 2. Check if refresh token is in the revocation list
    // TODO: Check Redis for revoked refresh tokens

    // 3. Issue new token pair
    const newAccessToken = this.jwtUtil.signAccessToken(decoded.sub, decoded.role);
    const newRefreshToken = this.jwtUtil.signRefreshToken(decoded.sub);

    // 4. Revoke the old refresh token (rotation)
    // TODO: Add old refresh token to Redis revocation list

    return {
      access_token: newAccessToken,
      refresh_token: newRefreshToken,
    };
  }

  async revoke(sessionId: string): Promise<void> {
    // TODO: Add all tokens for this session to the Redis revocation list
    void sessionId;
  }
}
