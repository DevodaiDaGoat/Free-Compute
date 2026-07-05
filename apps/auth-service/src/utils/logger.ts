/**
 * Minimal structured logger. Kept dependency-free so it can run anywhere;
 * swap for pino/winston later without changing call sites.
 */

type Level = "debug" | "info" | "warn" | "error";

function serializeError(err: unknown): unknown {
  if (err instanceof Error) {
    const base: Record<string, unknown> = {
      name: err.name,
      message: err.message,
      stack: err.stack,
    };
    // Walk the cause chain so wrapped errors don't lose their root context.
    const cause = (err as { cause?: unknown }).cause;
    if (cause !== undefined) base.cause = serializeError(cause);
    return base;
  }
  return err;
}

function emit(level: Level, message: string, meta?: Record<string, unknown>): void {
  const entry: Record<string, unknown> = {
    ts: new Date().toISOString(),
    level,
    message,
  };
  if (meta) {
    for (const [key, value] of Object.entries(meta)) {
      entry[key] = key === "err" || key === "cause" ? serializeError(value) : value;
    }
  }
  const line = JSON.stringify(entry);
  if (level === "error") console.error(line);
  else if (level === "warn") console.warn(line);
  else console.log(line);
}

export const logger = {
  debug: (msg: string, meta?: Record<string, unknown>) => emit("debug", msg, meta),
  info: (msg: string, meta?: Record<string, unknown>) => emit("info", msg, meta),
  warn: (msg: string, meta?: Record<string, unknown>) => emit("warn", msg, meta),
  error: (msg: string, meta?: Record<string, unknown>) => emit("error", msg, meta),
};
