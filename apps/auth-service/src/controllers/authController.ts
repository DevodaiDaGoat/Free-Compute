import { Request, Response, NextFunction } from 'express';
import { AuthService } from '../services/authService';

const authService = new AuthService();

export async function register(req: Request, res: Response, next: NextFunction): Promise<void> {
  try {
    const { email, password } = req.body;
    const result = await authService.register(email, password);
    res.status(201).json(result);
  } catch (err) {
    next(err);
  }
}

export async function login(req: Request, res: Response, next: NextFunction): Promise<void> {
  try {
    const { email, password } = req.body;
    const result = await authService.login(email, password);
    res.json(result);
  } catch (err) {
    // SECURITY: Return consistent error to prevent account enumeration
    res.status(401).json({ error: 'Invalid credentials' });
  }
}

export async function verify(req: Request, res: Response, next: NextFunction): Promise<void> {
  try {
    const { token } = req.body;
    const result = await authService.verifyEmail(token);
    res.json(result);
  } catch (err) {
    next(err);
  }
}

export async function logout(req: Request, res: Response, next: NextFunction): Promise<void> {
  try {
    const authHeader = req.headers.authorization;
    if (authHeader) {
      const token = authHeader.split(' ')[1];
      if (token) {
        await authService.revokeToken(token);
      }
    }
    res.json({ message: 'Logged out' });
  } catch (err) {
    next(err);
  }
}
