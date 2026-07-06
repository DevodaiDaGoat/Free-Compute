package tunnel

import (
	"context"
	"io"
	"net"
	"testing"
	"time"
)

func TestAgentPoolSkipsClosedIdleConnection(t *testing.T) {
	pool := newAgentPool()
	ctx := context.Background()

	staleGateway, staleAgent := net.Pipe()
	if _, err := pool.add(ctx, "vm-web", staleGateway); err != nil {
		t.Fatalf("add stale agent: %v", err)
	}
	_ = staleAgent.Close()

	liveGateway, liveAgent := net.Pipe()
	defer liveAgent.Close()
	if _, err := pool.add(ctx, "vm-web", liveGateway); err != nil {
		t.Fatalf("add live agent: %v", err)
	}

	takeCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	conn, cleanup, err := pool.take(takeCtx, "vm-web")
	if err != nil {
		t.Fatalf("take live agent: %v", err)
	}
	defer cleanup()
	defer conn.Close()

	go func() {
		_, _ = liveAgent.Write([]byte("ok"))
	}()

	buffer := make([]byte, 2)
	if _, err := io.ReadFull(conn, buffer); err != nil {
		t.Fatalf("read live agent payload: %v", err)
	}
	if string(buffer) != "ok" {
		t.Fatalf("payload = %q, want ok", string(buffer))
	}
}

func TestAgentPoolPreservesPrefetchedBytes(t *testing.T) {
	pool := newAgentPool()
	ctx := context.Background()

	gatewayConn, agentConn := net.Pipe()
	defer agentConn.Close()
	if _, err := pool.add(ctx, "vm-ssh", gatewayConn); err != nil {
		t.Fatalf("add agent: %v", err)
	}

	go func() {
		_, _ = agentConn.Write([]byte("SSH-2.0-test\r\n"))
	}()

	takeCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	conn, cleanup, err := pool.take(takeCtx, "vm-ssh")
	if err != nil {
		t.Fatalf("take agent: %v", err)
	}
	defer cleanup()
	defer conn.Close()

	buffer := make([]byte, len("SSH-2.0-test\r\n"))
	if _, err := io.ReadFull(conn, buffer); err != nil {
		t.Fatalf("read prefetched banner: %v", err)
	}
	if string(buffer) != "SSH-2.0-test\r\n" {
		t.Fatalf("banner = %q", string(buffer))
	}
}
