package tunnel

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"net"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
)

const webTransportMaxStreams = 1000
const webTransportIdleTimeout = 30 * time.Second

type WebTransportServer struct {
	mu         sync.Mutex
	listener   *quic.Listener
	sessions   map[quic.Connection]struct{}
	logger     *log.Logger
	cert       tls.Certificate
	running    bool
}

func NewWebTransportServer(logger *log.Logger, cert tls.Certificate) *WebTransportServer {
	if logger == nil {
		logger = log.Default()
	}
	return &WebTransportServer{
		sessions: make(map[quic.Connection]struct{}),
		logger:   logger,
		cert:     cert,
	}
}

func (s *WebTransportServer) Listen(addr string) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return errors.New("already running")
	}
	s.mu.Unlock()

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{s.cert},
		NextProtos:   []string{"freecompute/1", "h3"},
	}
	listener, err := quic.ListenAddr(addr, tlsConfig, nil)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.listener = listener
	s.running = true
	s.mu.Unlock()

	s.logger.Printf("webtransport listening on %s", addr)
	go s.acceptLoop()
	return nil
}

func (s *WebTransportServer) acceptLoop() {
	for {
		conn, err := s.listener.Accept(context.Background())
		if err != nil {
			s.mu.Lock()
			if !s.running {
				s.mu.Unlock()
				return
			}
			s.mu.Unlock()
			s.logger.Printf("webtransport accept error: %v", err)
			continue
		}
		s.mu.Lock()
		s.sessions[conn] = struct{}{}
		s.mu.Unlock()
		go s.handleSession(conn)
	}
}

func (s *WebTransportServer) handleSession(conn quic.Connection) {
	defer func() {
		conn.CloseWithError(0, "")
		s.mu.Lock()
		delete(s.sessions, conn)
		s.mu.Unlock()
	}()

	stream, err := conn.AcceptStream(context.Background())
	if err != nil {
		s.logger.Printf("webtransport accept stream error: %v", err)
		return
	}
	buf := make([]byte, 4096)
	_, _ = stream.Read(buf)
	_ = stream.Close()
}

func (s *WebTransportServer) GetSessionCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sessions)
}

func (s *WebTransportServer) Close() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	if s.listener != nil {
		_ = s.listener.Close()
	}
	for conn := range s.sessions {
		_ = conn.CloseWithError(0, "")
	}
	s.mu.Unlock()
	return nil
}

func (s *WebTransportServer) Stats() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	return map[string]interface{}{
		"running":  s.running,
		"sessions": len(s.sessions),
	}
}

func DialWebTransport(addr string, tlsConfig *tls.Config) (quic.Connection, error) {
	conn, err := quic.DialAddr(context.Background(), addr, tlsConfig, nil)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func ResolveWebTransportAddr(addr string) (*net.UDPAddr, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	port, err := net.LookupPort("udp", portStr)
	if err != nil {
		return nil, err
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		if ip.To4() != nil {
			return &net.UDPAddr{IP: ip, Port: port}, nil
		}
	}
	if len(ips) > 0 {
		return &net.UDPAddr{IP: ips[0], Port: port}, nil
	}
	return nil, errors.New("no ip found")
}
