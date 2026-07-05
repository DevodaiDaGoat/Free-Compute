export class EmailService {
  async sendVerification(email: string, token: string): Promise<void> {
    // TODO: Send verification email via SMTP or transactional email provider
    // SECURITY: Never include the token in logs
    console.log(`Verification email queued for: ${email}`);
    void token;
  }

  async sendPasswordReset(email: string, token: string): Promise<void> {
    // TODO: Send password reset email
    // SECURITY: Token must be single-use and expire within 1 hour
    console.log(`Password reset email queued for: ${email}`);
    void token;
  }
}
