import bcrypt from 'bcryptjs';

/** Hashes a plaintext password using bcrypt with the given cost. */
export async function hashPassword(plaintext: string, rounds: number): Promise<string> {
  const salt = await bcrypt.genSalt(rounds);
  return bcrypt.hash(plaintext, salt);
}

/** Compares a plaintext password against a stored bcrypt hash. */
export async function verifyPassword(plaintext: string, hash: string): Promise<boolean> {
  return bcrypt.compare(plaintext, hash);
}
