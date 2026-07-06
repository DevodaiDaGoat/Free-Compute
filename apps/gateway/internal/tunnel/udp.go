package tunnel

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

const (
	maxUDPPacketSize    = 64 * 1024
	udpSocketBufferSize = 4 * 1024 * 1024
)

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
	_ = listener.SetReadBuffer(udpSocketBufferSize)
	_ = listener.SetWriteBuffer(udpSocketBufferSize)

	s.logger.Printf("udp tunnel route=%s listening=%s target=%s", route.ID, route.Listen, route.Target)

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	type udpClient struct {
		conn *net.UDPConn
	}

	clients := map[string]*udpClient{}
	var clientsMu sync.Mutex
	buf := make([]byte, maxUDPPacketSize)

	for {
		n, clientAddr, err := listener.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}

		key := clientAddr.String()
		clientsMu.Lock()
		client := clients[key]
		if client == nil {
			upstream, err := dialUDP(route)
			if err != nil {
				clientsMu.Unlock()
				s.logger.Printf("udp dial route=%s client=%s error=%v", route.ID, key, err)
				continue
			}

			client = &udpClient{conn: upstream}
			clients[key] = client
			go s.copyUDPToClient(ctx, route, listener, upstream, clientAddr, func() {
				clientsMu.Lock()
				delete(clients, key)
				clientsMu.Unlock()
			})
		}
		clientsMu.Unlock()

		_ = client.conn.SetWriteDeadline(time.Now().Add(route.IdleTimeout()))
		if _, err := client.conn.Write(buf[:n]); err != nil {
			s.logger.Printf("udp write upstream route=%s client=%s error=%v", route.ID, key, err)
		}
	}
}

func dialUDP(route *Route) (*net.UDPConn, error) {
	targetAddr, err := net.ResolveUDPAddr("udp", route.Target)
	if err != nil {
		return nil, fmt.Errorf("resolve udp target route=%s target=%s: %w", route.ID, route.Target, err)
	}

	upstream, err := net.DialUDP("udp", nil, targetAddr)
	if err != nil {
		return nil, err
	}
	_ = upstream.SetReadBuffer(udpSocketBufferSize)
	_ = upstream.SetWriteBuffer(udpSocketBufferSize)

	return upstream, nil
}

func (s *Server) copyUDPToClient(ctx context.Context, route *Route, listener *net.UDPConn, upstream *net.UDPConn, clientAddr *net.UDPAddr, cleanup func()) {
	defer cleanup()
	defer upstream.Close()

	buf := make([]byte, maxUDPPacketSize)
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
