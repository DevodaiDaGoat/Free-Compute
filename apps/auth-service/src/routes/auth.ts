import { Router } from 'express';
import { register, login, verify, logout } from '../controllers/authController';
import { validate } from '../middleware/validation';
import { registerSchema, loginSchema, verifySchema } from '../middleware/validation';
import rateLimit from 'express-rate-limit';

export const authRouter = Router();

// SECURITY: Aggressive rate limiting on auth endpoints
const authLimiter = rateLimit({
  windowMs: 60 * 1000, // 1 minute
  max: 5,              // 5 attempts per minute per IP
  standardHeaders: true,
  legacyHeaders: false,
  message: { error: 'Too many attempts, please try again later' },
});

authRouter.post('/register', authLimiter, validate(registerSchema), register);
authRouter.post('/login', authLimiter, validate(loginSchema), login);
authRouter.post('/verify', validate(verifySchema), verify);
authRouter.post('/logout', logout);
