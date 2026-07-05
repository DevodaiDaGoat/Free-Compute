import { randomBytes, scrypt, timingSafeEqual } from "node:crypto";
import { promisify } from "node:util";

import { AppError } from "./errors";

const scryptAsync = promisify(scrypt);

const KEY_LEN = 64;
const SALT_LEN = 16;

/**
 * Hash a password using scrypt. Format: `scrypt$<saltHex>$<hashHex>`.
 * Uses Node's crypto so there is no native-module dependency.
 */
export async function hashPassword(password: string): Promise<string> {
  const salt = randomBytes(SALT_LEN);
  const derived = (await scryptAsync(password, salt, KEY_LEN)) as Buffer;
  return `scrypt$${salt.toString("hex")}$${derived.toString("hex")}`;
}

/**
 * Verify a password against a stored hash in constant time.
 *
 * A malformed stored hash is a data-integrity fault, not a wrong password, so
 * it is surfaced as an error rather than being swallowed into a `false` that
 * would masquerade as an ordinary failed login.
 */
export async function verifyPassword(password: string, stored: string): Promise<boolean> {
  const parts = stored.split("$");
  if (parts.length !== 3 || parts[0] !== "scrypt") {
    throw new AppError("Corrupt password hash on record", {
      code: "INTERNAL",
      isOperational: false,
    });
  }
  const salt = Buffer.from(parts[1]!, "hex");
  const expected = Buffer.from(parts[2]!, "hex");
  const derived = (await scryptAsync(password, salt, expected.length)) as Buffer;
  if (derived.length !== expected.length) return false;
  return timingSafeEqual(derived, expected);
}
