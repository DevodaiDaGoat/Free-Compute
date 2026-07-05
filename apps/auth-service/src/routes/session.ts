import { Router } from 'express';

import type { SessionController } from '../controllers/sessionController';

/** Builds the /session router group backed by the given controller. */
export function sessionRoutes(controller: SessionController): Router {
  const router = Router();
  router.post('/logout', controller.logout);
  return router;
}
