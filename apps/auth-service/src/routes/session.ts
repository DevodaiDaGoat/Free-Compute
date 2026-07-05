import { Router } from "express";

import type { SessionController } from "../controllers/sessionController";
import type { SessionService } from "../services/sessionService";
import { requireAuth } from "../middleware/auth";
import { asyncHandler } from "../utils/asyncHandler";

export function sessionRoutes(controller: SessionController, sessions: SessionService): Router {
  const router = Router();
  router.get("/current", requireAuth(sessions), asyncHandler(controller.current));
  return router;
}
