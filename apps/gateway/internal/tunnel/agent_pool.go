package tunnel

import (
	"container/list"
	"context"
	"errors"
	"net"
	"sync"
)

var (
	errNoAgentTunnel = errors.New("no agent tunnel available")
	errTCPConnClosing = errors.New("tcp connection closing")
	errTCPConnDead    = errors.New("tcp connection appears dead (unacked + lost)")
)

type pooledAgentConn struct {
	conn  net.Conn
	done  chan struct{}
	once  sync.Once
	route string
}

func (c *pooledAgentConn) finish() {
	c.once.Do(func() {
		close(c.done)
	})
}

type agentPool struct {
	mu      sync.Mutex
	idle    map[string]*list.List
	waiters map[string]map[chan *pooledAgentConn]struct{}
}

func newAgentPool() *agentPool {
	return &agentPool{
		idle:    map[string]*list.List{},
		waiters: map[string]map[chan *pooledAgentConn]struct{}{},
	}
}

func (p *agentPool) add(ctx context.Context, routeID string, conn net.Conn) (<-chan struct{}, error) {
	agentConn := &pooledAgentConn{
		conn:  conn,
		done:  make(chan struct{}),
		route: routeID,
	}

	p.mu.Lock()
	if waiters := p.waiters[routeID]; len(waiters) > 0 {
		for waiter := range waiters {
			delete(waiters, waiter)
			p.mu.Unlock()
			waiter <- agentConn
			close(waiter)
			return agentConn.done, nil
		}
	}

	l, ok := p.idle[routeID]
	if !ok {
		l = list.New()
		p.idle[routeID] = l
	}
	l.PushBack(agentConn)
	p.mu.Unlock()

	go func() {
		<-ctx.Done()
		if p.removeIdle(agentConn) {
			_ = conn.Close()
		}
	}()

	return agentConn.done, nil
}

func (p *agentPool) take(ctx context.Context, routeID string) (net.Conn, func(), error) {
	for {
		p.mu.Lock()
		if l, ok := p.idle[routeID]; ok && l.Len() > 0 {
			e := l.Front()
			l.Remove(e)
			agentConn := e.Value.(*pooledAgentConn)
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
		waiters := p.waiters[routeID]
		if waiters == nil {
			waiters = make(map[chan *pooledAgentConn]struct{})
			p.waiters[routeID] = waiters
		}
		waiters[waiter] = struct{}{}
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

func (p *agentPool) removeIdle(target *pooledAgentConn) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	l, ok := p.idle[target.route]
	if !ok {
		return false
	}
	for e := l.Front(); e != nil; e = e.Next() {
		if e.Value.(*pooledAgentConn) == target {
			l.Remove(e)
			if l.Len() == 0 {
				delete(p.idle, target.route)
			}
			return true
		}
	}
	return false
}

func (p *agentPool) removeWaiter(routeID string, target chan *pooledAgentConn) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if waiters, ok := p.waiters[routeID]; ok {
		if _, exists := waiters[target]; exists {
			delete(waiters, target)
			if len(waiters) == 0 {
				delete(p.waiters, routeID)
			}
		}
	}
}
