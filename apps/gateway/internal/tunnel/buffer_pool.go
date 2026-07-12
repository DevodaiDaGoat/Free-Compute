//go:build !windows

package tunnel

import (
	"net"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

var tcpStateNames = [...]string{
	1:  "ESTABLISHED",
	2:  "SYN_SENT",
	3:  "SYN_RECV",
	4:  "FIN_WAIT1",
	5:  "FIN_WAIT2",
	6:  "TIME_WAIT",
	7:  "CLOSE",
	8:  "CLOSE_WAIT",
	9:  "LAST_ACK",
	10: "LISTEN",
	11: "CLOSING",
}

const (
	tcpEstablished = 1
	tcpSynSent     = 2
	tcpSynRecv     = 3
	tcpFinWait1    = 4
	tcpFinWait2    = 5
	tcpTimeWait    = 6
	tcpClose       = 7
	tcpCloseWait   = 8
	tcpLastAck     = 9
	tcpListen      = 10
	tcpClosing     = 11
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

var tcpFastOpen = 5
var tcpDeferAccept = 5

func SetTCPFastOpen(queues int) {
	if queues > 0 {
		tcpFastOpen = queues
	}
}

var tcpUserTimeoutMs = 30_000

func SetTCPUserTimeout(ms int) {
	if ms > 0 {
		tcpUserTimeoutMs = ms
	}
}

func applyTCPSocketOptions(fd uintptr, qos *QoSConfig) {
	unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_NODELAY, 1)
	unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_QUICKACK, 1)
	_ = unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_FASTOPEN, tcpFastOpen)
	unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_KEEPIDLE, 5)
	unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_KEEPINTVL, 1)
	unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_KEEPCNT, 3)
	_ = unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_DEFER_ACCEPT, tcpDeferAccept)
	_ = unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_NOTSENT_LOWAT, 65_536)
	_ = unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_USER_TIMEOUT, tcpUserTimeoutMs)

	algo := tcpCCAlgo
	if algo == "" {
		current, _ := unix.GetsockoptString(int(fd), unix.IPPROTO_TCP, unix.TCP_CONGESTION)
		algo = current
	}
	if algo != "" {
		cc := algo
		if algo == "auto" {
			bbr, _ := unix.GetsockoptString(int(fd), unix.IPPROTO_TCP, unix.TCP_CONGESTION)
			if bbr != "bbr" {
				cc = "bbr"
			} else {
				cc = bbr
			}
		}
		_ = unix.SetsockoptString(int(fd), unix.IPPROTO_TCP, unix.TCP_CONGESTION, cc)
	}

	_ = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_SNDBUF, tcpBufferSize)
	_ = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_RCVBUF, tcpBufferSize)

	if qos != nil && qos.DSCP > 0 {
		priority := qos.DSCP << 2
		_ = unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_TOS, priority)
	}
}

func setTCPKeepaliveAggressive(conn *net.TCPConn) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	return raw.Control(func(fd uintptr) {
		applyTCPSocketOptions(fd, nil)
	})
}

func applyTCPListenerOptions(conn *net.TCPListener, qos *QoSConfig) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	return raw.Control(func(fd uintptr) {
		_ = unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_DEFER_ACCEPT, 5)
		_ = unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_QUICKACK, 1)
	})
}

func setUDSocketOptions(conn *net.UDPConn, qos *QoSConfig) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	sockBuf := int(udpSocketBufferSize.Load())
	return raw.Control(func(fd uintptr) {
		unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_RCVBUF, sockBuf)
		unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_SNDBUF, sockBuf)
		if qos != nil && qos.DSCP > 0 {
			priority := qos.DSCP << 2
			_ = unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_TOS, priority)
			_ = unix.SetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_TCLASS, priority)
		}
	})
}

func liveAgentConn(conn net.Conn) (net.Conn, error) {
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		raw, err := tcpConn.SyscallConn()
		if err == nil {
			var ti *unix.TCPInfo
			err = raw.Control(func(fd uintptr) {
				ti, err = unix.GetsockoptTCPInfo(int(fd), unix.IPPROTO_TCP, unix.TCP_INFO)
			})
			if err == nil && ti != nil {
				if ti.State == tcpTimeWait || ti.State == tcpCloseWait || ti.State == tcpClosing {
					return nil, errTCPConnClosing
				}
				if ti.Unacked > 10 && ti.Retransmits == 0 && ti.Lost > 3 {
					return nil, errTCPConnDead
				}
				return conn, nil
			}
		}
	}

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
