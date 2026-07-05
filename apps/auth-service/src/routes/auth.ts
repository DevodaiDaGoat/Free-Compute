import { Router } from 'express';

import {
  AuthController,
  loginSchema,
  registerSchema,
  verifySchema,
} from '../controllers/authController';
import { validateBody } from '../middleware/validation';

/** Builds the /auth router group backed by the given controller. */
export function authRoutes(controller: AuthController): Router {
  const router = Router();
  router.post('/register', validateBody(registerSchema), controller.register);
  router.post('/login', validateBody(loginSchema), controller.login);
  router.post('/verify', validateBody(verifySchema), controller.verify);
  return router;
}
