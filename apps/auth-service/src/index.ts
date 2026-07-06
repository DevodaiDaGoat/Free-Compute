import cors from 'cors';
import express from 'express';

import { loadConfig } from './config';
import { AuthController } from './controllers/authController';
import { SessionController } from './controllers/sessionController';
import { errorHandler, notFoundHandler } from './middleware/errorHandler';
import { authRoutes } from './routes/auth';
import { sessionRoutes } from './routes/session';
import { AuthService } from './services/authService';
import { ConsoleEmailService } from './services/emailService';
import { SessionService } from './services/sessionService';
import { sendSuccess } from './utils/response';

export function createApp(): express.Express {
  const config = loadConfig();

  // Compose services and controllers (constructor dependency injection).
  const authService = new AuthService(config);
  const emailService = new ConsoleEmailService();
  const sessionService = new SessionService();
  const authController = new AuthController(authService, emailService);
  const sessionController = new SessionController(sessionService);

  const app = express();
  app.use(express.json());
  app.use(cors({ origin: config.allowedOrigins, credentials: true }));

  app.get('/health', (_req, res) => {
    sendSuccess(res, 200, { status: 'ok' });
  });

  // All API routes are versioned under /v1.
  app.use('/v1/auth', authRoutes(authController));
  app.use('/v1/session', sessionRoutes(sessionController));

  app.use(notFoundHandler);
  app.use(errorHandler);

  return app;
}

function main(): void {
  const config = loadConfig();
  const app = createApp();
  app.listen(config.port, () => {
    console.log(`auth-service listening on :${config.port}`);
  });
}

// Only start the server when run directly, so the app can be imported in tests.
if (require.main === module) {
  main();
}
