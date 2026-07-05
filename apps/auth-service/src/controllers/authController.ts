import type { Request, Response } from 'express';
import { z } from 'zod';

import { AuthError, type AuthService } from '../services/authService';
import type { EmailService } from '../services/emailService';
import { ErrorCode, sendError, sendSuccess } from '../utils/response';

export const registerSchema = z.object({
  email: z.string().email(),
  password: z.string().min(8),
});

export const loginSchema = z.object({
  email: z.string().email(),
  password: z.string().min(1),
});

export const verifySchema = z.object({
  token: z.string().min(1),
});

/** AuthController wires HTTP requests to the auth and email services. */
export class AuthController {
  constructor(
    private readonly auth: AuthService,
    private readonly email: EmailService,
  ) {}

  register = async (req: Request, res: Response): Promise<void> => {
    const { email, password } = req.body as z.infer<typeof registerSchema>;
    try {
      const user = await this.auth.register(email, password);
      await this.email.sendVerification(user.email, 'verification-token-placeholder');
      sendSuccess(res, 201, { user });
    } catch (err) {
      if (err instanceof AuthError && err.code === 'CONFLICT') {
        sendError(res, 409, ErrorCode.Conflict, err.message);
        return;
      }
      throw err;
    }
  };

  login = async (req: Request, res: Response): Promise<void> => {
    const { email, password } = req.body as z.infer<typeof loginSchema>;
    try {
      const { token, user } = await this.auth.login(email, password);
      sendSuccess(res, 200, { token, user });
    } catch (err) {
      if (err instanceof AuthError && err.code === 'UNAUTHORIZED') {
        sendError(res, 401, ErrorCode.Unauthorized, err.message);
        return;
      }
      throw err;
    }
  };

  verify = (req: Request, res: Response): void => {
    const { token } = req.body as z.infer<typeof verifySchema>;
    const result = this.auth.verify(token);
    sendSuccess(res, 200, result);
  };
}
