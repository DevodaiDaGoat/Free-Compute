import { logger } from "../utils/logger";

export interface EmailService {
  sendVerification(email: string, token: string): Promise<void>;
}

/**
 * Stub email transport. Sending a verification email is a best-effort side
 * effect: a failure here must NOT abort registration (the account already
 * exists), but it must also not be silently swallowed. Callers log the failure
 * with full context so it is observable and retryable, rather than discarding
 * the error entirely.
 */
export class ConsoleEmailService implements EmailService {
  async sendVerification(email: string, token: string): Promise<void> {
    logger.info("Sending verification email", { email, token });
  }
}
