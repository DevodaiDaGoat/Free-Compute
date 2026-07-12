package tunnel

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type dnsCacheEntry struct {
	addrs   []string
	expires time.Time
}

type DNSCache struct {
	mu         sync.RWMutex
	entries    map[string]*dnsCacheEntry
	ttl        time.Duration
	maxEntries int
	resolver   *net.Resolver
	hits       int64
	misses     int64
}

func NewDNSCache(ttl time.Duration, maxEntries int) *DNSCache {
	return &DNSCache{
		entries:    make(map[string]*dnsCacheEntry),
		ttl:        ttl,
		maxEntries: maxEntries,
		resolver:   net.DefaultResolver,
	}
}

func (c *DNSCache) LookupHost(ctx context.Context, host string) ([]string, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []string{host}, nil
	}

	c.mu.RLock()
	entry, ok := c.entries[host]
	c.mu.RUnlock()

	if ok && time.Now().Before(entry.expires) {
		atomic.AddInt64(&c.hits, 1)
		return entry.addrs, nil
	}

	atomic.AddInt64(&c.misses, 1)
	addrs, err := c.resolver.LookupHost(ctx, host)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	if len(c.entries) >= c.maxEntries {
		// Evict expired entries first (cheap)
		for k, v := range c.entries {
			if time.Now().After(v.expires) {
				delete(c.entries, k)
			}
		}
		// If still full, kill a random entry (good enough for DNS cache)
		if len(c.entries) >= c.maxEntries {
			for k := range c.entries {
				delete(c.entries, k)
				break
			}
		}
	}
	c.entries[host] = &dnsCacheEntry{
		addrs:   addrs,
		expires: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()

	return addrs, nil
}

func (c *DNSCache) LookupAddr(ctx context.Context, addr string) (string, error) {
	c.mu.RLock()
	entry, ok := c.entries[addr]
	c.mu.RUnlock()

	if ok && time.Now().Before(entry.expires) && len(entry.addrs) > 0 {
		atomic.AddInt64(&c.hits, 1)
		return entry.addrs[0], nil
	}

	atomic.AddInt64(&c.misses, 1)
	name, err := c.resolver.LookupAddr(ctx, addr)
	if err != nil {
		return "", err
	}
	if len(name) == 0 {
		return "", nil
	}

	c.mu.Lock()
	c.entries[addr] = &dnsCacheEntry{
		addrs:   name,
		expires: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()

	return name[0], nil
}

func (c *DNSCache) Refresh(host string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, host)
}

func (c *DNSCache) Stats() (hits, misses int64) {
	// hits/misses are mutated via atomic.AddInt64 by LookupHost/LookupAddr on
	// the fast path. Reading them under RLock is a mixed-access data race —
	// the RLock does not synchronize with atomic stores. Use atomic loads to
	// stay consistent with the write path.
	return atomic.LoadInt64(&c.hits), atomic.LoadInt64(&c.misses)
}

func (c *DNSCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

type dnsCacheDialer struct {
	cache  *DNSCache
	dialer *net.Dialer
}

func newDNSCacheDialer(d *net.Dialer, cache *DNSCache) *dnsCacheDialer {
	return &dnsCacheDialer{cache: cache, dialer: d}
}

func (d *dnsCacheDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return d.dialer.DialContext(ctx, network, addr)
	}

	addrs, err := d.cache.LookupHost(ctx, host)
	if err != nil {
		return d.dialer.DialContext(ctx, network, addr)
	}

	var firstErr error
	for _, ip := range addrs {
		conn, err := d.dialer.DialContext(ctx, network, net.JoinHostPort(ip, port))
		if err == nil {
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				raw, err := tcpConn.SyscallConn()
				if err == nil {
					_ = raw.Control(func(fd uintptr) {
						applyTCPSocketOptions(fd, nil)
					})
				}
			}
			return conn, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}

	return nil, firstErr
}
