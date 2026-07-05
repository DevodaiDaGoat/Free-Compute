import { Router } from 'express';
import { getSession, refreshToken, revokeSession } from '../controllers/sessionController';

export const sessionRouter = Router();

sessionRouter.get('/', getSession);
sessionRouter.post('/refresh', refreshToken);
sessionRouter.post('/revoke', revokeSession);
