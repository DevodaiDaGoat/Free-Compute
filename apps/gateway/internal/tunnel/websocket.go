package tunnel

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	webSocketGUID            = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	webSocketOpcodeContinue  = 0x0
	webSocketOpcodeText      = 0x1
	webSocketOpcodeBinary    = 0x2
	webSocketOpcodeClose     = 0x8
	webSocketOpcodePing      = 0x9
	webSocketOpcodePong      = 0xA
	maxWebSocketFrameBytes   = 1024 * 1024
	maxWebSocketMessageBytes = 1024 * 1024
)

type webSocketBridge struct {
	conn        net.Conn
	rw          *bufio.ReadWriter
	idleTimeout time.Duration
	mu          sync.Mutex
}

func (s *Server) handleWebSocketTunnel(w http.ResponseWriter, r *http.Request) {
	routeID, _ := routeIDFromPath("/ws/", r.URL.Path)
	route, ok := s.registry.Get(routeID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if !s.authorize(route, w, r) {
		return
	}
	if route.Protocol != ProtocolTCP && route.Protocol != ProtocolSSH && route.Protocol != ProtocolUDP {
		http.Error(w, "route does not support websocket tunneling", http.StatusBadRequest)
		return
	}
	if !isWebSocketUpgrade(r) {
		http.Error(w, "websocket upgrade required", http.StatusUpgradeRequired)
		return
	}

	bridge, err := acceptWebSocket(w, r)
	if err != nil {
		s.logger.Printf("websocket accept route=%s error=%v", route.ID, err)
		return
	}
	bridge.idleTimeout = route.IdleTimeout()
	defer bridge.conn.Close()

	if route.Protocol == ProtocolUDP {
		upstream, err := dialUDP(route)
		if err != nil {
			_ = bridge.writeClose()
			s.logger.Printf("websocket udp dial route=%s error=%v", route.ID, err)
			return
		}
		defer upstream.Close()

		s.bridgeWebSocketToUDP(route, bridge, upstream)
		return
	}

	upstream, cleanup, err := s.openTCP(r.Context(), route)
	if err != nil {
		_ = bridge.writeClose()
		s.logger.Printf("websocket upstream dial route=%s error=%v", route.ID, err)
		return
	}
	defer upstream.Close()
	if cleanup != nil {
		defer cleanup()
	}

	s.bridgeWebSocketToTCP(route, bridge, upstream)
}

func (s *Server) bridgeWebSocketToTCP(route *Route, bridge *webSocketBridge, upstream net.Conn) {
	errCh := make(chan error, 2)

	go func() {
		buf := make([]byte, 32*1024)
		for {
			_ = upstream.SetReadDeadline(time.Now().Add(route.IdleTimeout()))
			n, err := upstream.Read(buf)
			if n > 0 {
				if writeErr := bridge.writeBinary(buf[:n]); writeErr != nil {
					errCh <- writeErr
					return
				}
			}
			if err != nil {
				errCh <- err
				return
			}
		}
	}()

	go func() {
		for {
			payload, err := bridge.readDataFrame()
			if len(payload) > 0 {
				_ = upstream.SetWriteDeadline(time.Now().Add(route.IdleTimeout()))
				if writeErr := writeAll(upstream, payload); writeErr != nil {
					errCh <- writeErr
					return
				}
			}
			if err != nil {
				errCh <- err
				return
			}
		}
	}()

	err := <-errCh
	if err != nil && !errors.Is(err, io.EOF) {
		s.logger.Printf("websocket tunnel route=%s closed=%v", route.ID, err)
	}
	_ = bridge.writeClose()
}

func (s *Server) bridgeWebSocketToUDP(route *Route, bridge *webSocketBridge, upstream *net.UDPConn) {
	errCh := make(chan error, 2)

	go func() {
		buf := make([]byte, maxUDPPacketSize)
		for {
			n, err := upstream.Read(buf)
			if n > 0 {
				if writeErr := bridge.writeBinary(buf[:n]); writeErr != nil {
					errCh <- writeErr
					return
				}
			}
			if err != nil {
				errCh <- err
				return
			}
		}
	}()

	go func() {
		for {
			payload, err := bridge.readDataFrame()
			if len(payload) > maxUDPPacketSize {
				errCh <- fmt.Errorf("udp websocket datagram too large: %d", len(payload))
				return
			}
			if len(payload) > 0 {
				if writeErr := writeDatagram(upstream, payload); writeErr != nil {
					errCh <- writeErr
					return
				}
			}
			if err != nil {
				errCh <- err
				return
			}
		}
	}()

	err := <-errCh
	if err != nil && !errors.Is(err, io.EOF) {
		s.logger.Printf("websocket udp tunnel route=%s closed=%v", route.ID, err)
	}
	_ = bridge.writeClose()
}

func writeAll(writer io.Writer, payload []byte) error {
	for len(payload) > 0 {
		n, err := writer.Write(payload)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		payload = payload[n:]
	}

	return nil
}

func writeDatagram(conn *net.UDPConn, payload []byte) error {
	n, err := conn.Write(payload)
	if err != nil {
		return err
	}
	if n != len(payload) {
		return io.ErrShortWrite
	}

	return nil
}

func acceptWebSocket(w http.ResponseWriter, r *http.Request) (*webSocketBridge, error) {
	key := strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key"))
	if key == "" {
		http.Error(w, "missing websocket key", http.StatusBadRequest)
		return nil, errors.New("missing websocket key")
	}
	if err := validateWebSocketKey(key); err != nil {
		http.Error(w, "invalid websocket key", http.StatusBadRequest)
		return nil, err
	}
	if version := strings.TrimSpace(r.Header.Get("Sec-WebSocket-Version")); version != "13" {
		http.Error(w, "unsupported websocket version", http.StatusBadRequest)
		return nil, fmt.Errorf("unsupported websocket version %q", version)
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking unsupported", http.StatusInternalServerError)
		return nil, errors.New("hijacking unsupported")
	}

	conn, rw, err := hijacker.Hijack()
	if err != nil {
		return nil, err
	}

	accept := computeWebSocketAccept(key)
	_, err = fmt.Fprintf(
		rw,
		"HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n",
		accept,
	)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := rw.Flush(); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return &webSocketBridge{conn: conn, rw: rw}, nil
}

func computeWebSocketAccept(key string) string {
	sum := sha1.Sum([]byte(key + webSocketGUID))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func validateWebSocketKey(key string) error {
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return err
	}
	if len(decoded) != 16 {
		return fmt.Errorf("websocket key must decode to 16 bytes")
	}

	return nil
}

func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

func (b *webSocketBridge) readDataFrame() ([]byte, error) {
	var message []byte
	fragmented := false

	for {
		fin, opcode, payload, err := b.readFrame()
		if err != nil {
			return nil, err
		}

		switch opcode {
		case webSocketOpcodeText, webSocketOpcodeBinary:
			if fragmented {
				return nil, errors.New("new websocket message before fragmented message completed")
			}
			if fin {
				return payload, nil
			}

			fragmented = true
			message = append(message[:0], payload...)
		case webSocketOpcodeContinue:
			if !fragmented {
				return nil, errors.New("unexpected websocket continuation frame")
			}
			if len(message)+len(payload) > maxWebSocketMessageBytes {
				return nil, fmt.Errorf("websocket message too large: %d", len(message)+len(payload))
			}
			message = append(message, payload...)
			if fin {
				return message, nil
			}
		case webSocketOpcodePing:
			if !fin {
				return nil, errors.New("fragmented websocket ping frame")
			}
			if err := b.writeControl(webSocketOpcodePong, payload); err != nil {
				return nil, err
			}
		case webSocketOpcodeClose:
			return nil, io.EOF
		case webSocketOpcodePong:
			continue
		default:
			return nil, fmt.Errorf("unsupported websocket opcode %d", opcode)
		}
	}
}

func (b *webSocketBridge) readFrame() (bool, byte, []byte, error) {
	b.setReadDeadline()
	first, err := b.rw.ReadByte()
	if err != nil {
		return false, 0, nil, err
	}
	second, err := b.rw.ReadByte()
	if err != nil {
		return false, 0, nil, err
	}

	fin := first&0x80 != 0
	opcode := first & 0x0f
	masked := second&0x80 != 0
	payloadLen := uint64(second & 0x7f)

	switch payloadLen {
	case 126:
		var raw [2]byte
		if _, err := io.ReadFull(b.rw, raw[:]); err != nil {
			return false, 0, nil, err
		}
		payloadLen = uint64(binary.BigEndian.Uint16(raw[:]))
	case 127:
		var raw [8]byte
		if _, err := io.ReadFull(b.rw, raw[:]); err != nil {
			return false, 0, nil, err
		}
		payloadLen = binary.BigEndian.Uint64(raw[:])
	}

	if payloadLen > maxWebSocketFrameBytes {
		return false, 0, nil, fmt.Errorf("websocket frame too large: %d", payloadLen)
	}
	if opcode >= webSocketOpcodeClose {
		if !fin {
			return false, 0, nil, errors.New("fragmented websocket control frame")
		}
		if payloadLen > 125 {
			return false, 0, nil, fmt.Errorf("websocket control frame too large: %d", payloadLen)
		}
	}
	if !masked {
		return false, 0, nil, errors.New("client websocket frames must be masked")
	}

	var mask [4]byte
	if _, err := io.ReadFull(b.rw, mask[:]); err != nil {
		return false, 0, nil, err
	}

	payloadSize := int(payloadLen)
	if uint64(payloadSize) != payloadLen {
		return false, 0, nil, fmt.Errorf("websocket frame too large for platform: %d", payloadLen)
	}

	payload := make([]byte, payloadSize)
	if _, err := io.ReadFull(b.rw, payload); err != nil {
		return false, 0, nil, err
	}

	for i := range payload {
		payload[i] ^= mask[i%4]
	}

	return fin, opcode, payload, nil
}

func (b *webSocketBridge) writeBinary(payload []byte) error {
	return b.writeFrame(webSocketOpcodeBinary, payload)
}

func (b *webSocketBridge) writeControl(opcode byte, payload []byte) error {
	if len(payload) > 125 {
		payload = payload[:125]
	}

	return b.writeFrame(opcode, payload)
}

func (b *webSocketBridge) writeClose() error {
	return b.writeControl(webSocketOpcodeClose, nil)
}

func (b *webSocketBridge) writeFrame(opcode byte, payload []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.setWriteDeadline()

	header := []byte{0x80 | opcode}
	switch {
	case len(payload) < 126:
		header = append(header, byte(len(payload)))
	case len(payload) <= 0xffff:
		header = append(header, 126, byte(len(payload)>>8), byte(len(payload)))
	default:
		header = append(header, 127)
		var raw [8]byte
		binary.BigEndian.PutUint64(raw[:], uint64(len(payload)))
		header = append(header, raw[:]...)
	}

	if _, err := b.rw.Write(header); err != nil {
		return err
	}
	if len(payload) > 0 {
		if _, err := b.rw.Write(payload); err != nil {
			return err
		}
	}

	return b.rw.Flush()
}

func (b *webSocketBridge) setReadDeadline() {
	if b.idleTimeout > 0 {
		_ = b.conn.SetReadDeadline(time.Now().Add(b.idleTimeout))
	}
}

func (b *webSocketBridge) setWriteDeadline() {
	if b.idleTimeout > 0 {
		_ = b.conn.SetWriteDeadline(time.Now().Add(b.idleTimeout))
	}
}
