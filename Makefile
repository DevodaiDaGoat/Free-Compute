.PHONY: all build run dev test clean lint typecheck docker-build docker-up

GO_GATEWAY_DIR  := apps/gateway
GO_HOST_AGENT   := host-agent
FRONTEND_DIR    := apps/frontend

all: build

# ── Go builds ──
build:
	@echo "=== Building Go services ==="
	(cd $(GO_GATEWAY_DIR) && go build -o /tmp/freecompute-gateway ./cmd/gateway)
	(cd $(GO_HOST_AGENT) && go build -o /tmp/freecompute-host-agent ./cmd/host-agent)
	@echo "=== Building frontend ==="
	(cd $(FRONTEND_DIR) && npm run build)
	@echo "=== Done ==="

build-go:
	@echo "=== Building Go services ==="
	(cd $(GO_GATEWAY_DIR) && go build -o /tmp/freecompute-gateway ./cmd/gateway)
	(cd $(GO_HOST_AGENT) && go build -o /tmp/freecompute-host-agent ./cmd/host-agent)
	@echo "=== Done ==="

build-frontend:
	@echo "=== Building frontend ==="
	(cd $(FRONTEND_DIR) && npm run build)
	@echo "=== Done ==="

# ── Run ──
run:
	./run-backend.sh

run-gateway:
	(cd $(GO_GATEWAY_DIR) && go run ./cmd/gateway)

run-host-agent:
	(cd $(GO_HOST_AGENT) && go run ./cmd/host-agent)

run-frontend:
	(cd $(FRONTEND_DIR) && npm run dev)

# ── Dev (convenience) ──
dev:
	@echo "Starting all services in dev mode..."
	@echo "  Frontend: http://localhost:3000"
	@echo "  Gateway:  http://localhost:8080"
	@echo "Run 'make run' to start full backend."
	(cd $(FRONTEND_DIR) && npm run dev)

# ── Test ──
test:
	@echo "=== Running Go tests ==="
	go test ./...
	@echo "=== Done ==="

# ── Lint / Typecheck ──
lint:
	npm run lint

typecheck:
	npm run typecheck

# ── Docker ──
docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

# ── Utility ──
env-check:
	@echo "Checking required environment variables..."
	@for var in FREECOMPUTE_GATEWAY_ADDR FREECOMPUTE_TUNNEL_TOKEN; do \
		if [ -z "$${!var}" ]; then \
			echo "  WARNING: $$var is not set"; \
		else \
			echo "  OK: $$var=$${!var}"; \
		fi; \
	done

clean:
	@echo "=== Cleaning ==="
	rm -f /tmp/freecompute-gateway /tmp/freecompute-host-agent /tmp/freecompute-vm-setup
	rm -rf $(FRONTEND_DIR)/.next
	@echo "=== Done ==="

help:
	@echo "FreeCompute Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  build          Build all services (Go + frontend)"
	@echo "  build-go       Build only Go services"
	@echo "  build-frontend Build only frontend"
	@echo "  run            Start full backend (run-backend.sh)"
	@echo "  run-gateway    Start gateway only"
	@echo "  run-host-agent Start host-agent only"
	@echo "  run-frontend   Start frontend dev server"
	@echo "  dev            Start frontend dev server"
	@echo "  test           Run Go tests"
	@echo "  lint           Run linter (turbo)"
	@echo "  typecheck      Run TypeScript typecheck (turbo)"
	@echo "  docker-build   Build Docker images"
	@echo "  docker-up      Start Docker Compose services"
	@echo "  docker-down    Stop Docker Compose services"
	@echo "  docker-logs    Follow Docker Compose logs"
	@echo "  env-check      Verify required env vars"
	@echo "  clean          Clean build artifacts"
	@echo "  help           Show this help"
