FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /host-agent ./cmd/main.go

FROM alpine:3.19
RUN apk --no-cache add ca-certificates qemu-system-x86_64
RUN adduser -D -g '' appuser
RUN mkdir -p /var/lib/freecompute/vms /etc/freecompute && chown -R appuser:appuser /var/lib/freecompute
USER appuser
COPY --from=builder /host-agent /host-agent
ENTRYPOINT ["/host-agent"]
