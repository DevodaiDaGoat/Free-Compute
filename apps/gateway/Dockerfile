# syntax=docker/dockerfile:1

# --- Builder stage ---
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Cache dependencies first.
COPY go.mod go.sum ./
RUN go mod download

# Build the static binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /gateway ./cmd

# --- Runtime stage ---
FROM alpine:3.19

RUN apk add --no-cache ca-certificates && adduser -D -u 10001 appuser

WORKDIR /app
COPY --from=builder /gateway /app/gateway

USER appuser
EXPOSE 8080

ENTRYPOINT ["/app/gateway"]
