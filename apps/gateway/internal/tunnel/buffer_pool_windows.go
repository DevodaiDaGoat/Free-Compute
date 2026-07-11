//go:build windows

package tunnel

import (
	"net"
	"sync"
	"time"
)

const proxyBufferSize = 512 * 1024

type byteBufferPool struct {
	size int
	pool sync.Pool
}

func newByteBufferPool(size int) *byteBufferPool {
	pool := &byteBufferPool{size: size}
	pool.pool.New = func() any {
		return make([]byte, size)
	}
	return pool
}

func (p *byteBufferPool) Get() []byte {
	return p.pool.Get().([]byte)
}

func (p *byteBufferPool) Put(buffer []byte) {
	if cap(buffer) < p.size {
		return
	}
	p.pool.Put(buffer[:p.size])
}

var copyBufferPool = sync.Pool{
	New: func() any {
		b := make([]byte, 512*1024)
		return &b
	},
}

func getCopyBuf() *[]byte {
	return copyBufferPool.Get().(*[]byte)
}

func putCopyBuf(buf *[]byte) {
	copyBufferPool.Put(buf)
}

var tcpCCAlgo string

func SetTCPCCAlgo(algo string) {
	tcpCCAlgo = algo
}

var tcpBufferSize = 8_388_608

func SetTCPBufferSize(size int) {
	if size > 0 {
		if size > 16*1024*1024 {
			size = 16 * 1024 * 1024
		}
		tcpBufferSize = size
	}
}

func applyTCPSocketOptions(fd uintptr, qos *QoSConfig) {}

func setTCPKeepaliveAggressive(conn *net.TCPConn) error {
	return nil
}

func applyTCPListenerOptions(conn *net.TCPListener, qos *QoSConfig) error {
	return nil
}

func setUDSocketOptions(conn *net.UDPConn, qos *QoSConfig) error {
	return nil
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

type prefetchedConn struct {
	net.Conn
	prefix []byte
}

func (c *prefetchedConn) Read(p []byte) (int, error) {
	if len(c.prefix) == 0 {
		return c.Conn.Read(p)
	}
	n := copy(p, c.prefix)
	c.prefix = c.prefix[n:]
	return n, nil
}

var rtpBufferPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 1500)
		return &buf
	},
}

func getRTPBuf() *[]byte {
	return rtpBufferPool.Get().(*[]byte)
}

func putRTPBuf(buf *[]byte) {
	rtpBufferPool.Put(buf)
}

var udpBufferPool = sync.Pool{
	New: func() any {
		buf := make([]byte, maxUDPPacketSize)
		return &buf
	},
}

func getUDPBuf() *[]byte {
	return udpBufferPool.Get().(*[]byte)
}

func putUDPBuf(buf *[]byte) {
	udpBufferPool.Put(buf)
}