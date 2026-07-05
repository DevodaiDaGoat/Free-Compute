import { ValidationError } from "../utils/errors";

export interface Credentials {
  email: string;
  password: string;
}

interface FieldIssue {
  field: string;
  message: string;
}

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

/**
 * Validate a login/registration body. Collects every problem and throws a
 * single 422 with structured `details` so the client gets all field errors at
 * once instead of discovering them one request at a time.
 */
export function parseCredentials(body: unknown): Credentials {
  const issues: FieldIssue[] = [];
  const record = (typeof body === "object" && body !== null ? body : {}) as Record<
    string,
    unknown
  >;

  const email = record.email;
  if (typeof email !== "string" || !EMAIL_RE.test(email)) {
    issues.push({ field: "email", message: "A valid email is required" });
  }

  const password = record.password;
  if (typeof password !== "string" || password.length < 8) {
    issues.push({ field: "password", message: "Password must be at least 8 characters" });
  }

  if (issues.length > 0) {
    throw new ValidationError("Invalid request body", { details: { issues } });
  }

  return { email: (email as string).trim().toLowerCase(), password: password as string };
}
