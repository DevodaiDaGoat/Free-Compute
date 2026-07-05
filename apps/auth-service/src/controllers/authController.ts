import type { Request, Response } from "express";

import type { AuthService } from "../services/authService";
import { BadRequestError, UnauthorizedError } from "../utils/errors";
import { parseCredentials } from "../middleware/validation";

export class AuthController {
  constructor(private readonly auth: AuthService) {}

  register = async (req: Request, res: Response): Promise<void> => {
    const { email, password } = parseCredentials(req.body);
    const result = await this.auth.register(email, password);
    res.status(201).json(result);
  };

  login = async (req: Request, res: Response): Promise<void> => {
    const { email, password } = parseCredentials(req.body);
    const result = await this.auth.login(email, password);
    res.status(200).json(result);
  };

  verify = async (req: Request, res: Response): Promise<void> => {
    const body = (typeof req.body === "object" && req.body !== null ? req.body : {}) as Record<
      string,
      unknown
    >;
    const userId = body.userId;
    if (typeof userId !== "string" || userId.length === 0) {
      throw new BadRequestError("userId is required");
    }
    const user = await this.auth.verify(userId);
    res.status(200).json({ user });
  };

  logout = async (req: Request, res: Response): Promise<void> => {
    // requireAuth guarantees a session, but guard defensively so a misordered
    // middleware chain fails loudly rather than dereferencing undefined.
    if (!req.session) {
      throw new UnauthorizedError("Not authenticated");
    }
    await this.auth.logout(req.session.id);
    res.status(204).send();
  };
}
