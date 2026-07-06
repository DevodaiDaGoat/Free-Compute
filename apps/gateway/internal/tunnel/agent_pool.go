package tunnel

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

var errNoAgentTunnel = errors.New("no agent tunnel available")

type pooledAgentConn struct {
	conn net.Conn
	done chan struct{}
	once sync.Once
}

func (c *pooledAgentConn) finish() {
	c.once.Do(func() {
		close(c.done)
	})
}

type prefetchedConn struct {
	net.Conn
	prefix []byte
}

type agentPool struct {
	mu      sync.Mutex
	idle    map[string][]*pooledAgentConn
	waiters map[string][]chan *pooledAgentConn
}

func newAgentPool() *agentPool {
	return &agentPool{
		idle:    map[string][]*pooledAgentConn{},
		waiters: map[string][]chan *pooledAgentConn{},
	}
}

func (p *agentPool) add(ctx context.Context, routeID string, conn net.Conn) (<-chan struct{}, error) {
	agentConn := &pooledAgentConn{
		conn: conn,
		done: make(chan struct{}),
	}

	p.mu.Lock()
	if waiters := p.waiters[routeID]; len(waiters) > 0 {
		waiter := waiters[0]
		p.waiters[routeID] = waiters[1:]
		p.mu.Unlock()
		waiter <- agentConn
		close(waiter)
		return agentConn.done, nil
	}

	p.idle[routeID] = append(p.idle[routeID], agentConn)
	p.mu.Unlock()

	go func() {
		<-ctx.Done()
		p.removeIdle(routeID, agentConn)
		_ = conn.Close()
	}()

	return agentConn.done, nil
}

func (p *agentPool) take(ctx context.Context, routeID string) (net.Conn, func(), error) {
	for {
		p.mu.Lock()
		if idle := p.idle[routeID]; len(idle) > 0 {
			agentConn := idle[0]
			p.idle[routeID] = idle[1:]
			p.mu.Unlock()

			conn, err := liveAgentConn(agentConn.conn)
			if err == nil {
				return conn, agentConn.finish, nil
			}

			agentConn.finish()
			_ = agentConn.conn.Close()
			continue
		}

		waiter := make(chan *pooledAgentConn, 1)
		p.waiters[routeID] = append(p.waiters[routeID], waiter)
		p.mu.Unlock()

		select {
		case agentConn := <-waiter:
			if agentConn == nil {
				return nil, nil, errNoAgentTunnel
			}
			conn, err := liveAgentConn(agentConn.conn)
			if err == nil {
				return conn, agentConn.finish, nil
			}

			agentConn.finish()
			_ = agentConn.conn.Close()
		case <-ctx.Done():
			p.removeWaiter(routeID, waiter)
			return nil, nil, ctx.Err()
		}
	}
}

func liveAgentConn(conn net.Conn) (net.Conn, error) {
	if err := conn.SetReadDeadline(time.Now()); err != nil {
		return nil, err
	}

	var first [1]byte
	n, err := conn.Read(first[:])
	_ = conn.SetReadDeadline(time.Time{})
	if n > 0 {
		return &prefetchedConn{Conn: conn, prefix: first[:n]}, nil
	}
	if err == nil {
		return conn, nil
	}
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return conn, nil
	}

	return nil, err
}

func (c *prefetchedConn) Read(p []byte) (int, error) {
	if len(c.prefix) == 0 {
		return c.Conn.Read(p)
	}

	n := copy(p, c.prefix)
	c.prefix = c.prefix[n:]
	return n, nil
}

func (p *agentPool) removeIdle(routeID string, target *pooledAgentConn) {
	p.mu.Lock()
	defer p.mu.Unlock()

	idle := p.idle[routeID]
	for i, agentConn := range idle {
		if agentConn == target {
			p.idle[routeID] = append(idle[:i], idle[i+1:]...)
			return
		}
	}
}

func (p *agentPool) removeWaiter(routeID string, target chan *pooledAgentConn) {
	p.mu.Lock()
	defer p.mu.Unlock()

	waiters := p.waiters[routeID]
	for i, waiter := range waiters {
		if waiter == target {
			p.waiters[routeID] = append(waiters[:i], waiters[i+1:]...)
			close(waiter)
			return
		}
	}
}
