<div align="center">

# FreeCompute

### Community-Powered Cloud Computing

Launch secure cloud desktops, development environments, and gaming sessions directly from your browser.

Powered by donated computers and cloud infrastructure.

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Status](https://img.shields.io/badge/status-Pre--Alpha-orange)]()
[![Platform](https://img.shields.io/badge/platform-Web-blue)]()
[![Made With](https://img.shields.io/badge/Made%20With-Go%20%7C%20Next.js%20%7C%20React-success)]()

</div>

---

# 🚀 What is FreeCompute?

FreeCompute is an open-source distributed cloud computing platform that allows users to launch virtual desktops directly from their browser.

Instead of relying entirely on expensive cloud providers, FreeCompute combines donated computers, community-hosted machines, and cloud infrastructure into one distributed network.

Whether you're developing software, browsing the web, testing applications, or gaming remotely, FreeCompute provides secure virtual machines that are accessible from anywhere.

---

# ✨ Features

## 🌐 Browser-Based Desktop

- Custom WebOS
- File manager
- Terminal
- Browser
- Window manager
- Multiple applications
- Virtual desktops
- Responsive interface

---

## 💻 Community-Powered Infrastructure

Contribute idle hardware to the network.

Donate:

- CPU
- RAM
- GPU
- Storage
- Bandwidth

Receive community rewards while helping others.

---

## ⚡ Smart Galaxy Scheduler

Automatically selects the best available machine based on:

- Region
- Network latency
- CPU utilization
- RAM availability
- GPU availability
- Host health
- Queue length
- Resource requirements

---

## 🎮 Gaming Support

Gaming is a first-class feature.

Supported:

- Browser streaming
- Controller support
- Fullscreen mode
- Low-latency streaming
- GPU scheduling
- Adaptive bitrate
- Adaptive resolution

Future plans:

- Steam integration
- Game launcher
- Cloud save support
- Dedicated gaming hosts

---

## 🖥 Remote Desktop

Secure remote desktop sessions.

Features include:

- Browser access
- Clipboard sync
- File transfers
- Audio forwarding
- Multiple display support (planned)
- Temporary session links
- Secure authentication

---

## 💳 Credit System

FreeCompute uses Credits instead of subscriptions.

Credits may be earned by:

- Hosting computers
- Community testing
- Bug reports
- Events
- Contributions
- Optional purchases (future)

Credits can unlock:

- More CPU
- More RAM
- GPU access
- Longer sessions
- Priority queue

---

## 🔒 Security

Security is built into every layer.

Features include:

- Email verification
- Optional multi-factor authentication
- Signed host agents
- JWT authentication
- TLS encryption
- Session isolation
- Sandboxed virtual machines
- Malware scanning for uploads
- Audit logging
- Role-based permissions

---

# 🏗 Architecture

```
                Internet
                    │
          Cloudflare Tunnel
                    │
             API Gateway
                    │
          Authentication Service
                    │
             Orchestrator
                    │
           Galaxy Scheduler
                    │
      ┌─────────────┴─────────────┐
      │                           │
 Community Hosts           Cloud Hosts
      │                           │
      └─────────────┬─────────────┘
                    │
              Virtual Machine
                    │
            WebRTC Streaming
                    │
              Browser Client
```

---

# 📦 Monorepo Structure

```
apps/
    frontend/
    gateway/
    auth-service/
    scheduler/
    billing-service/
    file-service/
    host-control/
    admin-api/
    notifications/
    orchestrator/

desktop/

host-agent/

packages/
    ui/
    api-types/
    utils/
    config/
    logger/
    database/
    auth/
    websocket/
    theme/
    sdk/

docs/

scripts/

infrastructure/

tests/
```

---

# 🖥 WebOS

FreeCompute includes a custom browser-based operating system.

Applications include:

- Terminal
- Browser
- Files
- Settings
- Task Manager
- System Monitor
- Store

Future applications:

- VS Code
- Firefox
- Chrome
- Discord
- Steam
- Minecraft Launcher

---

# 🛰 Host Agent

Users can contribute computers securely.

The Host Agent provides:

- Automatic registration
- Heartbeats
- Resource monitoring
- Secure communication
- Automatic updates
- Job execution
- Crash recovery
- Usage reporting

---

# 🧠 Galaxy Scheduler

The scheduler continuously evaluates:

- Host health
- Region
- Latency
- CPU usage
- RAM usage
- GPU availability
- Queue length
- Storage
- Network quality

Its goal is to provide the best experience while protecting host machines from overload.

---

# 🧩 Tech Stack

## Frontend

- Next.js 15
- React 19
- TypeScript
- Tailwind CSS v4
- shadcn/ui
- Framer Motion
- Zustand
- TanStack Query
- React Hook Form
- Zod

## Backend

- Go
- Node.js
- PostgreSQL
- Redis
- WebSockets

## Desktop

- React
- TypeScript
- WebRTC

## Infrastructure

- Docker
- Docker Compose
- Kubernetes
- GitHub Actions
- Cloudflare Tunnel

---

# 📈 Roadmap

## Phase 1

- Monorepo
- Shared UI
- Landing page
- Authentication
- Dashboard

## Phase 2

- Queue
- Credit system
- Host Agent
- Scheduler
- Basic Linux VM

## Phase 3

- WebOS
- Window manager
- Terminal
- File manager
- Browser

## Phase 4

- GPU scheduling
- Gaming mode
- Remote desktop improvements
- Persistent storage
- Snapshots

## Phase 5

- Global infrastructure
- High availability
- Marketplace
- Public API
- SDK

---

# 🤝 Contributing

Contributions are welcome.

Please read:

- CONTRIBUTING.md
- CODE_OF_CONDUCT.md
- SECURITY.md

before opening a Pull Request.

---

# 📖 Documentation

Project documentation lives inside:

```
docs/
```

Including:

- Architecture
- API
- Development Guide
- Desktop Design
- Backend Services
- Scheduler
- Deployment

---

# 🌎 Vision

Our long-term goal is to build a free, open-source cloud computing platform powered by the community.

Instead of requiring expensive infrastructure, FreeCompute allows anyone to contribute unused computing resources to help power desktops, development environments, and remote workloads for others.

By combining donated hardware with cloud infrastructure, we aim to make secure, browser-based computing more accessible while remaining transparent, extensible, and community-driven.

---

# 📜 License

Released under the MIT License.

See LICENSE for more information.

---

<div align="center">

### ⭐ If you like this project, consider starring the repository!

Built with ❤️ by the FreeCompute community.

</div>
