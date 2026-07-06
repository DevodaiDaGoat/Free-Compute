package tunnel

import "sync"

const proxyBufferSize = 32 * 1024

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
