import type { Request, Response } from "express";

import { UnauthorizedError } from "../utils/errors";

export class SessionController {
  /** Return the current authenticated session (populated by requireAuth). */
  current = async (req: Request, res: Response): Promise<void> => {
    if (!req.session) {
      throw new UnauthorizedError("Not authenticated");
    }
    res.status(200).json({ session: req.session });
  };
}
