import express, { type Express } from "express";

import type { Config } from "./config";
import { AuthController } from "./controllers/authController";
import { SessionController } from "./controllers/sessionController";
import { errorHandler, notFoundHandler } from "./middleware/errorHandler";
import { SessionRepository } from "./models/Session";
import { UserRepository } from "./models/User";
import { authRoutes } from "./routes/auth";
import { sessionRoutes } from "./routes/session";
import { AuthService } from "./services/authService";
import { ConsoleEmailService } from "./services/emailService";
import { SessionService } from "./services/sessionService";
import { asyncHandler } from "./utils/asyncHandler";

/**
 * Compose the Express app and its dependency graph. Kept separate from server
 * bootstrap so it can be imported by tests without opening a port.
 */
export function buildApp(config: Config): Express {
  const app = express();
  app.use(express.json());

  const users = new UserRepository();
  const sessionRepo = new SessionRepository();
  const email = new ConsoleEmailService();

  const sessionService = new SessionService(sessionRepo, config);
  const authService = new AuthService(users, sessionService, email);

  const authController = new AuthController(authService);
  const sessionController = new SessionController();

  app.get(
    "/health",
    asyncHandler(async (_req, res) => {
      res.status(200).json({ status: "ok" });
    }),
  );

  app.use("/auth", authRoutes(authController, sessionService));
  app.use("/session", sessionRoutes(sessionController, sessionService));

  // Order matters: unmatched-route handler first, then the central error
  // handler last so every error funnels through one place.
  app.use(notFoundHandler);
  app.use(errorHandler);

  return app;
}
