package tunnel

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	maxUDPPacketSize    = 64 * 1024
	udpClientIdleTimeout = 60 * time.Second
	udpClientSweepInterval = 30 * time.Second
)

var udpSocketBufferSize atomic.Int64

func init() {
	udpSocketBufferSize.Store(32 * 1024 * 1024)
}

func SetUDPBufferSize(size int) {
	if size > 0 {
		if size > 64*1024*1024 {
			size = 64 * 1024 * 1024
		}
		udpSocketBufferSize.Store(int64(size))
	}
}

type udpClient struct {
	conn      *net.UDPConn
	updatedAt time.Time
}

func (s *Server) serveUDP(ctx context.Context, route *Route) error {
	listenAddr, err := net.ResolveUDPAddr("udp", route.Listen)
	if err != nil {
		return fmt.Errorf("resolve udp listen route=%s addr=%s: %w", route.ID, route.Listen, err)
	}

	listener, err := net.ListenUDP("udp", listenAddr)
	if err != nil {
		return fmt.Errorf("listen udp route=%s addr=%s: %w", route.ID, route.Listen, err)
	}
	defer listener.Close()
	sockBuf := int(udpSocketBufferSize.Load())
	_ = listener.SetReadBuffer(sockBuf)
	_ = listener.SetWriteBuffer(sockBuf)
	_ = setUDSocketOptions(listener, route.QoS)

	s.logger.Printf("udp tunnel route=%s listening=%s target=%s", route.ID, route.Listen, route.Target)

	clients := &udpClientMap{
		clients: make(map[string]*udpClient),
	}

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	go clients.sweepLoop(ctx)

	bufPtr := getUDPBuf()
	defer putUDPBuf(bufPtr)
	buf := *bufPtr

	for {
		n, clientAddr, err := listener.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}

		key := clientAddr.String()
		client := clients.get(key)
		if client == nil {
			upstream, err := s.dialUDP(route)
			if err != nil {
				s.logger.Printf("udp dial route=%s client=%s error=%v", route.ID, key, err)
				continue
			}

			client = &udpClient{conn: upstream, updatedAt: time.Now()}
			clients.set(key, client)
			go s.copyUDPToClient(ctx, route, listener, upstream, clientAddr, func() {
				clients.delete(key)
			})
		} else {
			clients.touch(key)
		}

		_ = client.conn.SetWriteDeadline(time.Now().Add(route.IdleTimeout()))
		if _, err := client.conn.Write(buf[:n]); err != nil {
			s.logger.Printf("udp write upstream route=%s client=%s error=%v", route.ID, key, err)
		}
	}
}

type udpClientMap struct {
	mu      sync.Mutex
	clients map[string]*udpClient
}

func (m *udpClientMap) get(key string) *udpClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.clients[key]
}

func (m *udpClientMap) set(key string, client *udpClient) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients[key] = client
}

func (m *udpClientMap) touch(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if client, ok := m.clients[key]; ok {
		client.updatedAt = time.Now()
	}
}

func (m *udpClientMap) delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.clients, key)
}

func (m *udpClientMap) sweepLoop(ctx context.Context) {
	ticker := time.NewTicker(udpClientSweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.sweep()
		}
	}
}

func (m *udpClientMap) sweep() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for key, client := range m.clients {
		if now.Sub(client.updatedAt) > udpClientIdleTimeout {
			client.conn.Close()
			delete(m.clients, key)
		}
	}
}

// chooseUDPBufSize returns cfgSize if it is positive, otherwise falls back to
// the provided default. This avoids setting the socket buffer size twice.
func chooseUDPBufSize(cfgSize, fallback int) int {
	if cfgSize > 0 {
		return cfgSize
	}
	return fallback
}

func (s *Server) dialUDP(route *Route) (*net.UDPConn, error) {
	targetAddr, err := net.ResolveUDPAddr("udp", route.Target)
	if err != nil {
		return nil, fmt.Errorf("resolve udp target route=%s target=%s: %w", route.ID, route.Target, err)
	}

	upstream, err := net.DialUDP("udp", nil, targetAddr)
	if err != nil {
		return nil, err
	}
	bufSize := chooseUDPBufSize(s.cfg.UDPBufferSize, int(udpSocketBufferSize.Load()))
	_ = upstream.SetReadBuffer(bufSize)
	_ = upstream.SetWriteBuffer(bufSize)

	return upstream, nil
}

func (s *Server) copyUDPToClient(ctx context.Context, route *Route, listener *net.UDPConn, upstream *net.UDPConn, clientAddr *net.UDPAddr, cleanup func()) {
	defer cleanup()
	defer upstream.Close()

	bufPtr := getUDPBuf()
	defer putUDPBuf(bufPtr)
	buf := *bufPtr
	for {
		_ = upstream.SetReadDeadline(time.Now().Add(route.IdleTimeout()))
		n, err := upstream.Read(buf)
		if err != nil {
			if ctx.Err() == nil {
				s.logger.Printf("udp upstream closed route=%s client=%s error=%v", route.ID, clientAddr.String(), err)
			}
			return
		}

		_ = listener.SetWriteDeadline(time.Now().Add(route.IdleTimeout()))
		if _, err := listener.WriteToUDP(buf[:n], clientAddr); err != nil {
			s.logger.Printf("udp write client route=%s client=%s error=%v", route.ID, clientAddr.String(), err)
			return
		}
	}
}
