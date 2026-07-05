import { hash, verify } from '@node-rs/argon2';

// SECURITY: Argon2id is the recommended password hashing algorithm.
// It is memory-hard (resists GPU attacks) and provides both side-channel
// resistance (from Argon2i) and GPU resistance (from Argon2d).
const ARGON2_OPTIONS = {
  memoryCost: 65536,   // 64 MB — high memory cost resists GPU/ASIC attacks
  timeCost: 3,         // 3 iterations
  parallelism: 4,      // 4 parallel threads
};

export async function hashPassword(password: string): Promise<string> {
  return hash(password, ARGON2_OPTIONS);
}

export async function verifyPassword(hashedPassword: string, plainPassword: string): Promise<boolean> {
  return verify(hashedPassword, plainPassword);
}

// Password strength requirements
export function validatePasswordStrength(password: string): { valid: boolean; reason?: string } {
  if (password.length < 8) {
    return { valid: false, reason: 'Password must be at least 8 characters' };
  }
  if (password.length > 128) {
    return { valid: false, reason: 'Password must not exceed 128 characters' };
  }

  // Check for common weak passwords
  const commonPasswords = new Set([
    'password', '12345678', 'qwerty123', 'admin123', 'letmein1',
  ]);
  if (commonPasswords.has(password.toLowerCase())) {
    return { valid: false, reason: 'Password is too common' };
  }

  return { valid: true };
}
