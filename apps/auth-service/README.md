# auth-service

Authentication and session service for Free-Compute (see
[`docs/BACKEND_STRUCTURE.md`](../../docs/BACKEND_STRUCTURE.md)).

A small Express + TypeScript service whose primary goal is **robust,
consistent error handling**: no error is silently swallowed, and every failure
is propagated to a single place that turns it into a stable HTTP response.

## Error handling model

- **Typed errors** (`src/utils/errors.ts`): `AppError` plus subclasses carry a
  machine-readable `code`, an HTTP `statusCode`, optional safe `details`, an
  `isOperational` flag, and the original `cause`. `AppError.from()` normalizes
  any thrown value without discarding context.
- **Async propagation** (`src/utils/asyncHandler.ts`): wraps async route
  handlers so rejected promises reach Express via `next(err)` instead of
  becoming unhandled rejections (the classic Express footgun).
- **Central handler** (`src/middleware/errorHandler.ts`): the only place errors
  become responses. Logs operational errors at `warn` and unexpected faults at
  `error` with the full cause chain, and never leaks internal messages/stacks
  on 5xx responses.
- **Process-level safety net** (`src/index.ts`): `unhandledRejection` and
  `uncaughtException` are logged (and the latter triggers a graceful shutdown)
  rather than dying silently. Misconfiguration fails fast at boot.

## Endpoints

| Method | Path               | Notes                          |
| ------ | ------------------ | ------------------------------ |
| GET    | `/health`          | Liveness probe                 |
| POST   | `/auth/register`   | Create account, returns tokens |
| POST   | `/auth/login`      | Returns tokens                 |
| POST   | `/auth/verify`     | Mark a user verified           |
| POST   | `/auth/logout`     | Requires `Authorization` bearer |
| GET    | `/session/current` | Requires `Authorization` bearer |

## Development

```bash
npm install
npm run typecheck   # tsc --noEmit
npm run build       # emit dist/
JWT_SECRET=dev npm start
```

`JWT_SECRET` is required in production (boot fails without it); a dev fallback
is used otherwise. Data is stored in-memory — the repository classes mirror an
async DB surface so PostgreSQL/Redis can be dropped in later without touching
call sites.
