import type { Request, Response } from 'express';

import type { SessionService } from '../services/sessionService';
import { ErrorCode, sendError, sendSuccess } from '../utils/response';

/** SessionController handles logout / session revocation. */
export class SessionController {
  constructor(private readonly sessions: SessionService) {}

  logout = (req: Request, res: Response): void => {
    const header = req.header('authorization') ?? '';
    const [scheme, token] = header.split(' ');
    if (scheme?.toLowerCase() !== 'bearer' || !token) {
      sendError(res, 401, ErrorCode.Unauthorized, 'missing bearer token');
      return;
    }
    const revoked = this.sessions.revoke(token);
    sendSuccess(res, 200, { revoked });
  };
}
