FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /file-service ./cmd/main.go

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
RUN adduser -D -g '' appuser
RUN mkdir -p /data/files && chown appuser:appuser /data/files
USER appuser
COPY --from=builder /file-service /file-service
EXPOSE 8082
ENTRYPOINT ["/file-service"]
