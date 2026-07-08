// Dead code — session multiplexer is not wired into the server.
// Kept for future reference; all exports below are commented out.
package tunnel

import (
	"encoding/binary"
	"errors"
	"sync"
)

type StreamType byte

const (
	StreamTypeControl   StreamType = 0x00
	StreamTypeVideo     StreamType = 0x01
	StreamTypeAudio     StreamType = 0x02
	StreamTypeInput     StreamType = 0x03
	StreamTypeClipboard StreamType = 0x04
	StreamTypeFileMeta  StreamType = 0x05
	StreamTypeFileData  StreamType = 0x06
	StreamTypeSession   StreamType = 0x07
)

type fusedFrameHeader struct {
	Type     StreamType
	Seq      uint16
	Checksum byte
}

const fusedFrameHeaderSize = 4

type sessionMultiplexer struct {
	mu        sync.Mutex
	transport fusedTransport
	streams   map[StreamType]*fusedStream
}

type fusedTransport interface {
	ReadFrame() ([]byte, error)
	WriteFrame([]byte) error
	Close() error
}

type fusedStream struct {
	st     StreamType
	buf    []byte
	closed bool
}

func newSessionMultiplexer(transport fusedTransport) *sessionMultiplexer {
	return &sessionMultiplexer{
		transport: transport,
		streams:   make(map[StreamType]*fusedStream),
	}
}

func (m *sessionMultiplexer) OpenStream(st StreamType) *fusedStream {
	m.mu.Lock()
	defer m.mu.Unlock()

	s := &fusedStream{st: st}
	m.streams[st] = s
	return s
}

func (m *sessionMultiplexer) Write(st StreamType, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	frame := make([]byte, fusedFrameHeaderSize+len(data))
	frame[0] = byte(st)
	binary.BigEndian.PutUint16(frame[1:3], 0)
	var chk byte
	for _, b := range data {
		chk ^= b
	}
	frame[3] = chk
	copy(frame[fusedFrameHeaderSize:], data)

	return m.transport.WriteFrame(frame)
}

func (m *sessionMultiplexer) Read() (StreamType, []byte, error) {
	frame, err := m.transport.ReadFrame()
	if err != nil {
		return 0, nil, err
	}
	if len(frame) < fusedFrameHeaderSize {
		return 0, nil, errors.New("frame too short")
	}

	st := StreamType(frame[0])
	payload := frame[fusedFrameHeaderSize:]

	var chk byte
	for _, b := range payload {
		chk ^= b
	}
	if chk != frame[3] {
		return 0, nil, errors.New("frame checksum mismatch")
	}

	return st, payload, nil
}

func (m *sessionMultiplexer) Close() error {
	return m.transport.Close()
}

type wsFusedTransport struct {
	bridge *webSocketBridge
}

func (w *wsFusedTransport) ReadFrame() ([]byte, error) {
	return w.bridge.readDataFrame()
}

func (w *wsFusedTransport) WriteFrame(data []byte) error {
	return w.bridge.writeBinary(data)
}

func (w *wsFusedTransport) Close() error {
	return w.bridge.writeClose()
}

// func fuseWebSocketBridge(bridge *webSocketBridge) *sessionMultiplexer {
// 	return newSessionMultiplexer(&wsFusedTransport{bridge: bridge})
// }

// func copyConnFused(errCh chan<- error, dst *sessionMultiplexer, src io.Reader, st StreamType) {
// 	buf := getCopyBuf()
// 	defer putCopyBuf(buf)
// 	buffer := *buf
// 	for {
// 		n, err := src.Read(buffer)
// 		if n > 0 {
// 			if writeErr := dst.Write(st, buffer[:n]); writeErr != nil {
// 				errCh <- writeErr
// 				return
// 			}
// 		}
// 		if err != nil {
// 			if err == io.EOF {
// 				err = nil
// 			}
// 			errCh <- err
// 			return
// 		}
// 	}
// }
