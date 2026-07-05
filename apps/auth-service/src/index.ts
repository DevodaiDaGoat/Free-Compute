import { buildApp } from "./app";
import { loadConfig } from "./config";
import { AppError } from "./utils/errors";
import { logger } from "./utils/logger";

function main(): void {
  let config;
  try {
    config = loadConfig();
  } catch (err) {
    // Misconfiguration is fatal — fail fast and loudly rather than starting in
    // a degraded/insecure state.
    logger.error("Failed to load configuration", { err: AppError.from(err) });
    process.exit(1);
  }

  const app = buildApp(config);
  const server = app.listen(config.port, () => {
    logger.info("auth-service listening", { port: config.port, env: config.nodeEnv });
  });

  // Surface faults that escape the request lifecycle instead of letting the
  // process die silently or linger in an unknown state.
  process.on("unhandledRejection", (reason) => {
    logger.error("Unhandled promise rejection", { err: AppError.from(reason) });
  });
  process.on("uncaughtException", (err) => {
    logger.error("Uncaught exception; shutting down", { err: AppError.from(err) });
    server.close(() => process.exit(1));
  });

  const shutdown = (signal: string): void => {
    logger.info("Received shutdown signal", { signal });
    server.close((err) => {
      if (err) {
        logger.error("Error during shutdown", { err: AppError.from(err) });
        process.exit(1);
      }
      process.exit(0);
    });
  };
  process.on("SIGTERM", () => shutdown("SIGTERM"));
  process.on("SIGINT", () => shutdown("SIGINT"));
}

main();
