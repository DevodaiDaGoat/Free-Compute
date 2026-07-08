package tunnel

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/freecompute/free-compute/apps/gateway/internal/auth"
)

func (s *Server) serveTCP(ctx context.Context, route *Route) error {
	listenConfig := net.ListenConfig{KeepAlive: 5 * time.Second}
	listener, err := listenConfig.Listen(ctx, "tcp", route.Listen)
	if err != nil {
		return fmt.Errorf("listen tcp route=%s addr=%s: %w", route.ID, route.Listen, err)
	}
	defer listener.Close()

	if tcpListener, ok := listener.(*net.TCPListener); ok {
		_ = applyTCPListenerOptions(tcpListener, route.QoS)
	}

	s.logger.Printf("tcp tunnel route=%s listening=%s target=%s", route.ID, route.Listen, route.Target)

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		client, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}

		go s.relayTCP(ctx, route, client)
	}
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	routeID, _ := routeIDFromPath("/connect/", r.URL.Path)
	route, ok := s.registry.Get(routeID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if !s.authorize(route, w, r) {
		return
	}
	if user := auth.UserFromContext(r); user != nil {
		if !s.incrementUserConns(user.ID) {
			s.writeConnLimitReached(w, route.ID)
			return
		}
		defer s.decrementUserConns(user.ID)
	}
	if route.Protocol != ProtocolTCP && route.Protocol != ProtocolSSH {
		http.Error(w, "route does not support CONNECT tunneling", http.StatusBadRequest)
		return
	}
	if r.Method != http.MethodConnect {
		http.Error(w, "CONNECT required", http.StatusMethodNotAllowed)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking unsupported", http.StatusInternalServerError)
		return
	}

	client, rw, err := hijacker.Hijack()
	if err != nil {
		s.logger.Printf("connect hijack route=%s error=%v", route.ID, err)
		return
	}

	if tcpConn, ok := client.(*net.TCPConn); ok {
		raw, err := tcpConn.SyscallConn()
		if err == nil {
			_ = raw.Control(func(fd uintptr) {
				applyTCPSocketOptions(fd, route.QoS)
			})
		}
	}

	upstream, cleanup, err := s.openTCP(r.Context(), route)
	if err != nil {
		_, _ = client.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		_ = client.Close()
		s.logger.Printf("connect dial route=%s error=%v", route.ID, err)
		return
	}

	_, _ = client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	s.bridgeTCP(r.Context(), route, client, upstream, rw.Reader, cleanup)
}

func (s *Server) relayTCP(ctx context.Context, route *Route, client net.Conn) {
	upstream, cleanup, err := s.openTCP(ctx, route)
	if err != nil {
		s.logger.Printf("tcp dial route=%s error=%v", route.ID, err)
		_ = client.Close()
		return
	}

	s.bridgeTCP(ctx, route, client, upstream, nil, cleanup)
}

func (s *Server) dialTCP(ctx context.Context, route *Route) (net.Conn, error) {
	dialer := net.Dialer{
		Timeout:   s.cfg.DialTimeout,
		KeepAlive: 5 * time.Second,
	}

	conn, err := dialer.DialContext(ctx, "tcp", route.Target)
	if err != nil {
		return nil, err
	}

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
		_ = setTCPKeepaliveAggressive(tcpConn)
	}

	return conn, nil
}

func (s *Server) openTCP(ctx context.Context, route *Route) (net.Conn, func(), error) {
	if route.UsesAgentTunnel() {
		tsConn, tsErr := s.dialViaTailscale(ctx, route)
		if tsErr == nil {
			return tsConn, func() {}, nil
		}

		waitCtx, cancel := context.WithTimeout(ctx, s.agentWaitTimeout())
		defer cancel()

		conn, cleanup, err := s.agents.take(waitCtx, route.ID)
		if err != nil {
			return nil, nil, err
		}
		return conn, cleanup, nil
	}

	conn, err := s.dialTCP(ctx, route)
	if err != nil {
		return nil, nil, err
	}
	return conn, func() {}, nil
}

func (s *Server) bridgeTCP(ctx context.Context, route *Route, client net.Conn, upstream net.Conn, buffered *bufio.Reader, cleanup func()) {
	defer client.Close()
	defer upstream.Close()
	if cleanup != nil {
		defer cleanup()
	}

	if tcpConn, ok := client.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
		_ = setTCPKeepaliveAggressive(tcpConn)
	}

	errCh := make(chan error, 2)
	if buffered != nil && buffered.Buffered() > 0 {
		_ = upstream.SetWriteDeadline(time.Now().Add(route.IdleTimeout()))
		if _, err := io.CopyN(upstream, buffered, int64(buffered.Buffered())); err != nil {
			s.logger.Printf("tcp buffered copy route=%s error=%v", route.ID, err)
			return
		}
	}

	go copyConnWithIdle(errCh, upstream, client, route.IdleTimeout())
	go copyConnWithIdle(errCh, client, upstream, route.IdleTimeout())

	select {
	case <-ctx.Done():
	case <-errCh:
	}
	// Drain second error to prevent goroutine leak
	select {
	case <-errCh:
	default:
	}
}

func (s *Server) agentWaitTimeout() time.Duration {
	if s.cfg.AgentWaitTimeout > 0 {
		return s.cfg.AgentWaitTimeout
	}
	if s.cfg.DialTimeout > 0 {
		return s.cfg.DialTimeout
	}

	return time.Duration(defaultAgentWaitSeconds) * time.Second
}

func copyConnWithIdle(errCh chan<- error, dst net.Conn, src net.Conn, idleTimeout time.Duration) {
	buf := getCopyBuf()
	defer putCopyBuf(buf)
	buffer := *buf
	var err error

	for {
		if idleTimeout > 0 {
			_ = src.SetReadDeadline(time.Now().Add(idleTimeout))
		}

		n, readErr := src.Read(buffer)
		if n > 0 {
			if idleTimeout > 0 {
				_ = dst.SetWriteDeadline(time.Now().Add(idleTimeout))
			}
			if writeErr := writeAll(dst, buffer[:n]); writeErr != nil {
				err = writeErr
				break
			}
		}
		if readErr != nil {
			err = readErr
			break
		}
	}

	if closeWriter, ok := dst.(interface{ CloseWrite() error }); ok {
		_ = closeWriter.CloseWrite()
	}
	errCh <- err
}
