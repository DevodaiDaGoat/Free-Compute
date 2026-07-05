import { Router } from "express";

import type { AuthController } from "../controllers/authController";
import type { SessionService } from "../services/sessionService";
import { requireAuth } from "../middleware/auth";
import { asyncHandler } from "../utils/asyncHandler";

export function authRoutes(controller: AuthController, sessions: SessionService): Router {
  const router = Router();
  router.post("/register", asyncHandler(controller.register));
  router.post("/login", asyncHandler(controller.login));
  router.post("/verify", asyncHandler(controller.verify));
  router.post("/logout", requireAuth(sessions), asyncHandler(controller.logout));
  return router;
}
