/**
 * EmailService sends transactional email (verification links, password resets).
 * This skeleton logs to stdout; a provider-backed implementation (SES, Resend,
 * etc.) can replace it behind the same interface.
 */
export interface EmailService {
  sendVerification(email: string, token: string): Promise<void>;
}

export class ConsoleEmailService implements EmailService {
  async sendVerification(email: string, token: string): Promise<void> {
    // Placeholder: a real implementation would dispatch via an email provider.
    console.log(`[email] verification for ${email}: token=${token}`);
  }
}
