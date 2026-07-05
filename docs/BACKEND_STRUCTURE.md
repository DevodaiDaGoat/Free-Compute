# Backend Services Structure

## Gateway (`apps/gateway/`)

```
gateway/
├── cmd/
│   └── main.go                    # Entry point
│
├── internal/
│   ├── api/
│   │   ├── auth.go
│   │   ├── vm.go
│   │   ├── queue.go
│   │   ├── files.go
│   │   └── credits.go
│   │
│   ├── middleware/
│   │   ├── auth.go
│   │   ├── ratelimit.go
│   │   └── cors.go
│   │
│   ├── websocket/
│   │   ├── hub.go                 # Connection hub
│   │   ├── client.go              # Connection handler
│   │   └── message.go
│   │
│   ├── config/
│   │   └── config.go
│   │
│   └── utils/
│       └── errors.go
│
├── go.mod
├── go.sum
└── Dockerfile
```

## Auth Service (`apps/auth-service/`)

```
auth-service/
├── src/
│   ├── index.ts                   # Express server
│   │
│   ├── routes/
│   │   ├── auth.ts
│   │   └── session.ts
│   │
│   ├── controllers/
│   │   ├── authController.ts
│   │   └── sessionController.ts
│   │
│   ├── services/
│   │   ├── authService.ts
│   │   ├── emailService.ts
│   │   └── sessionService.ts
│   │
│   ├── models/
│   │   ├── User.ts
│   │   └── Session.ts
│   │
│   ├── middleware/
│   │   ├── errorHandler.ts
│   │   └── validation.ts
│   │
│   └── utils/
│       ├── jwt.ts
│       └── password.ts
│
├── package.json
└── Dockerfile
```

## Scheduler (`apps/scheduler/`)

```
scheduler/
├── cmd/
│   └── main.go                    # Entry point
│
├── internal/
│   ├── scheduler/
│   │   ├── galaxy.go              # Main algorithm
│   │   ├── ranker.go              # Host scoring
│   │   ├── allocator.go           # Resource allocation
│   │   └── queue.go               # Queue management
│   │
│   ├── host/
│   │   ├── manager.go             # Host lifecycle
│   │   ├── metrics.go             # Host metrics collection
│   │   └── health.go              # Health checks
│   │
│   ├── database/
│   │   ├── db.go
│   │   ├── models.go
│   │   └── queries.go
│   │
│   └── config/
│       └── config.go
│
├── go.mod
├── go.sum
└── Dockerfile
```

## Billing Service (`apps/billing-service/`)

```
billing-service/
├── src/
│   ├── index.ts
│   │
│   ├── routes/
│   │   ├── credits.ts
│   │   └── transactions.ts
│   │
│   ├── controllers/
│   │   ├── creditController.ts
│   │   └── transactionController.ts
│   │
│   ├── services/
│   │   ├── creditService.ts
│   │   ├── paymentService.ts
│   │   └── rewardService.ts
│   │
│   ├── models/
│   │   ├── Credit.ts
│   │   └── Transaction.ts
│   │
│   └── database/
│       ├── db.ts
│       └── migrations/
│
├── package.json
└── Dockerfile
```

## File Service (`apps/file-service/`)

```
file-service/
├── cmd/
│   └── main.go
│
├── internal/
│   ├── handler/
│   │   ├── upload.go
│   │   ├── download.go
│   │   └── delete.go
│   │
│   ├── storage/
│   │   ├── s3.go                  # S3-compatible
│   │   └── local.go               # Local storage
│   │
│   ├── models/
│   │   └── file.go
│   │
│   └── config/
│       └── config.go
│
├── go.mod
├── go.sum
└── Dockerfile
```

## Host Agent (`host-agent/`)

```
host-agent/
├── cmd/
│   └── main.go                    # Entry point
│
├── internal/
│   ├── agent/
│   │   ├── agent.go               # Main loop
│   │   ├── heartbeat.go           # Send metrics
│   │   └── register.go            # Register with gateway
│   │
│   ├── vm/
│   │   ├── launcher.go            # Launch QEMU
│   │   ├── manager.go             # Manage VMs
│   │   ├── monitor.go             # Monitor resources
│   │   └── cleanup.go             # Cleanup
│   │
│   ├── security/
│   │   ├── sign.go                # Sign requests
│   │   ├── verify.go              # Verify gateway
│   │   └── certs.go               # Certificate management
│   │
│   ├── config/
│   │   └── config.go
│   │
│   └── utils/
│       └── ssh.go
│
├── go.mod
├── go.sum
└── Dockerfile
```

## Package Structures

### `packages/api-types/`

```
api-types/
├── src/
│   ├── auth.ts
│   ├── vm.ts
│   ├── queue.ts
│   ├── credits.ts
│   ├── user.ts
│   ├── host.ts
│   └── api.ts
│
├── package.json
└── tsconfig.json
```

### `packages/ui/`

```
ui/
├── src/
│   ├── components/
│   │   ├── Button.tsx
│   │   ├── Card.tsx
│   │   ├── Input.tsx
│   │   └── ...
│   │
│   ├── hooks/
│   │   └── index.ts
│   │
│   └── index.ts
│
├── package.json
└── tsconfig.json
```

### `packages/utils/`

```
utils/
├── src/
│   ├── api.ts
│   ├── date.ts
│   ├── format.ts
│   ├── validate.ts
│   └── index.ts
│
├── package.json
└── tsconfig.json
```

## Database Schema Overview

```
users
├── id (UUID)
├── email (unique)
├── password_hash
├── verified
├── credits
├── created_at
└── updated_at

vms
├── id (UUID)
├── user_id (FK)
├── host_id (FK)
├── name
├── state (running, paused, stopped)
├── cpu_cores
├── ram_gb
├── storage_gb
├── created_at
└── updated_at

hosts
├── id (UUID)
├── name
├── region
├── cpu_cores
├── ram_gb
├── gpu_vram_gb
├── online
├── last_heartbeat
└── created_at

queue
├── id (UUID)
├── user_id (FK)
├── position
├── joined_at
├── estimated_wait_seconds
└── updated_at
```

## API Endpoints Summary

```
POST   /auth/register
POST   /auth/login
POST   /auth/verify
POST   /auth/logout

GET    /vm
POST   /vm/launch
POST   /vm/:id/pause
POST   /vm/:id/resume
POST   /vm/:id/stop
DELETE /vm/:id

GET    /queue/status
POST   /queue/join
POST   /queue/leave

GET    /credits
POST   /credits/purchase

GET    /admin/hosts
POST   /admin/hosts/:id/restart

WS     /stream/:vm_id
```
