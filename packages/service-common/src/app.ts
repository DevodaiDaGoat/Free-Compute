import express, { type Express } from "express";
import rateLimit from "express-rate-limit";

export interface ServiceConfig {
  name: string;
  port: number;
  corsOrigins?: string[];
  rateLimitWindowMs?: number;
  rateLimitMax?: number;
}

// Creates an Express app with standard middleware (JSON parsing, CORS, rate
// limiting, health check). After adding routes, call `app.use(errorHandler)`
// from `./error-handler` to install the shared error handler.
export function createApp(config: ServiceConfig): Express {
  const app = express();

  app.use(express.json());

  app.use((req, res, next) => {
    const origins = config.corsOrigins ?? ["http://localhost:3000"];
    const origin = req.headers.origin;
    if (origin && origins.includes(origin)) {
      res.setHeader("Access-Control-Allow-Origin", origin);
      res.setHeader(
        "Access-Control-Allow-Methods",
        "GET, POST, PUT, DELETE, PATCH, OPTIONS",
      );
      res.setHeader(
        "Access-Control-Allow-Headers",
        "Authorization, Content-Type",
      );
      res.setHeader("Access-Control-Allow-Credentials", "true");
    }
    if (req.method === "OPTIONS") {
      res.sendStatus(204);
      return;
    }
    next();
  });

  app.use(
    rateLimit({
      windowMs: config.rateLimitWindowMs ?? 60_000,
      max: config.rateLimitMax ?? 100,
      standardHeaders: true,
      legacyHeaders: false,
    }),
  );

  app.get("/health", (_req, res) => {
    res.json({ status: "ok", service: config.name });
  });

  return app;
}
