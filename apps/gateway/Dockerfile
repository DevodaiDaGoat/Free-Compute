FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /gateway ./cmd/main.go

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
RUN adduser -D -g '' appuser
USER appuser
COPY --from=builder /gateway /gateway
EXPOSE 8080
ENTRYPOINT ["/gateway"]
