import { Request, Response, NextFunction } from 'express';
import { SessionService } from '../services/sessionService';

const sessionService = new SessionService();

export async function getSession(req: Request, res: Response, next: NextFunction): Promise<void> {
  try {
    const authHeader = req.headers.authorization;
    if (!authHeader) {
      res.status(401).json({ error: 'Unauthorized' });
      return;
    }

    const token = authHeader.split(' ')[1];
    if (!token) {
      res.status(401).json({ error: 'Unauthorized' });
      return;
    }

    const session = await sessionService.validate(token);
    res.json(session);
  } catch (err) {
    next(err);
  }
}

export async function refreshToken(req: Request, res: Response, next: NextFunction): Promise<void> {
  try {
    const { refresh_token } = req.body;
    if (!refresh_token) {
      res.status(400).json({ error: 'refresh_token required' });
      return;
    }

    const result = await sessionService.refresh(refresh_token);
    res.json(result);
  } catch (err) {
    next(err);
  }
}

export async function revokeSession(req: Request, res: Response, next: NextFunction): Promise<void> {
  try {
    const { session_id } = req.body;
    if (!session_id) {
      res.status(400).json({ error: 'session_id required' });
      return;
    }

    await sessionService.revoke(session_id);
    res.json({ message: 'Session revoked' });
  } catch (err) {
    next(err);
  }
}
