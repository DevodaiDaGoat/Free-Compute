# Backend Services Structure

## Gateway (`apps/gateway/`)

```
gateway/
└── gateway
    └── main.go


├── admin
│   └── admin.go
├── audio
│   └── audio.go
├── auth
│   ├── auth.go
│   └── context.go
├── database
│   └── database.go
├── firewall
│   └── firewall.go
├── gaming
│   └── gaming.go
├── images
│   └── images.go
├── input
│   └── input.go
├── keys
│   └── keys.go
├── monitoring
│   ├── collector.go
│   ├── health.go
│   └── metrics.go
├── security
│   └── detector.go
├── session
│   ├── allocator.go
│   ├── audit.go
│   ├── scheduler.go
│   └── session.go
├── storage
│   └── storage.go
├── transfer
│   └── transfer.go
├── tunnel
│   ├── agent_handler.go
│   ├── agent_pool.go
│   ├── agent_pool_test.go
│   ├── auth.go
│   ├── buffer_pool.go
│   ├── config.go
│   ├── http_proxy.go
│   ├── routes.go
│   ├── server.go
│   ├── server_test.go
│   ├── signaling.go
│   ├── tcp.go
│   ├── udp.go
│   └── websocket.go
├── usage
│   └── usage.go
└── webrtc
    ├── encoder.go
    └── webrtc.go

```

## Scheduler (`apps/scheduler/`)

```
scheduler/
├── cmd
│   └── scheduler
│       └── main.go
├── go.mod
└── internal
    ├── config
    │   └── config.go
    ├── host
    │   ├── manager.go
    │   └── metrics.go
    └── scheduler
        ├── allocator.go
        ├── queue.go
        ├── ranker.go
        └── scheduler.go

```

## File Service (`apps/file-service/`)

```
file-service/
├── cmd
│   └── file-service
│       └── main.go
├── go.mod
└── internal
    ├── config
    │   └── config.go
    ├── handler
    │   └── handler.go
    ├── models
    │   └── models.go
    └── storage
        └── storage.go

```

## Host Agent (`host-agent/`)

```
host-agent/
├── README.md
├── cmd
│   ├── host-agent
│   │   ├── main.go
│   │   ├── tailscale.go
│   │   └── vm_manager.go
│   └── vm-setup
│       └── main.go
├── go.mod
├── host-agent
└── vm-setup.go

```

## Package Structures

### `packages/api-types/`

```
api-types/
├── api.ts
├── auth.ts
├── credits.ts
├── host.ts
├── index.ts
├── proxy.ts
├── queue.ts
├── remote.ts
├── user.ts
├── vm.ts
└── websocket.ts

```

