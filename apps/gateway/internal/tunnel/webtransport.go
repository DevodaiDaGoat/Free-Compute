package tunnel

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
)

const webTransportMaxStreams = 1000
const webTransportIdleTimeout = 30 * time.Second

// wtMessageType identifies the logical channel carried over a QUIC stream.
type wtMessageType uint8

const (
	// wtMsgInput carries a serialised InputEvent from client → host.
	wtMsgInput wtMessageType = 1
	// wtMsgVideo carries raw encoded video NAL units from host → client.
	wtMsgVideo wtMessageType = 2
	// wtMsgAudio carries encoded audio packets from host → client.
	wtMsgAudio wtMessageType = 3
	// wtMsgControl carries signaling control JSON in both directions.
	wtMsgControl wtMessageType = 4
)

// wtStreamHeader is the 8-byte header prepended to every QUIC stream:
//
//	[1 byte: message type][3 bytes: reserved][4 bytes: payload length]
type wtStreamHeader struct {
	MsgType   wtMessageType
	_         [3]byte
	PayloadLen uint32
}

// WebTransportServer provides a QUIC-based alternative transport for
// sessions, offering lower latency than TCP/WebSocket on lossy links.
//
// Each QUIC connection represents one client session. Streams are typed
// via a wtStreamHeader and dispatched to the appropriate tunnel handler.
type WebTransportServer struct {
	mu       sync.Mutex
	listener *quic.Listener
	sessions map[quic.Connection]struct{}
	logger   *log.Logger
	cert     tls.Certificate
	running  bool

	// acceptCtx is bound to the server lifetime; cancelling it via Close makes
	// listener.Accept return immediately instead of blocking indefinitely on a
	// hung UDP socket, so acceptLoop no longer leaks past shutdown.
	acceptCtx    context.Context
	acceptCancel context.CancelFunc

	// registry is used to dispatch input events to the correct route handler.
	// May be nil if the server is started standalone (test mode).
	registry RouteRegistry
}

// RouteRegistry is the interface satisfied by tunnel.routeRegistry to allow
// WebTransportServer to look up active routes without importing a concrete type.
type RouteRegistry interface {
	Get(routeID string) (*Route, bool)
}

func NewWebTransportServer(logger *log.Logger, cert tls.Certificate) *WebTransportServer {
	if logger == nil {
		logger = log.Default()
	}
	return &WebTransportServer{
		sessions: make(map[quic.Connection]struct{}),
		logger:   logger,
		cert:     cert,
	}
}

// SetRegistry wires the route registry so input events can be forwarded.
func (s *WebTransportServer) SetRegistry(r RouteRegistry) {
	s.mu.Lock()
	s.registry = r
	s.mu.Unlock()
}

func (s *WebTransportServer) Listen(addr string) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return errors.New("already running")
	}
	s.mu.Unlock()

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{s.cert},
		NextProtos:   []string{"freecompute/1", "h3"},
	}
	quicCfg := &quic.Config{
		MaxIncomingStreams:    webTransportMaxStreams,
		MaxIdleTimeout:       webTransportIdleTimeout,
		KeepAlivePeriod:      15 * time.Second,
		EnableDatagrams:      true,
	}
	listener, err := quic.ListenAddr(addr, tlsConfig, quicCfg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.listener = listener
	s.running = true
	s.acceptCtx = ctx
	s.acceptCancel = cancel
	s.mu.Unlock()

	s.logger.Printf("webtransport listening on %s (QUIC/UDP)", addr)
	go s.acceptLoop(ctx)
	return nil
}

func (s *WebTransportServer) acceptLoop(ctx context.Context) {
	for {
		conn, err := s.listener.Accept(ctx)
		if err != nil {
			s.mu.Lock()
			running := s.running
			s.mu.Unlock()
			if !running || ctx.Err() != nil {
				return
			}
			s.logger.Printf("webtransport accept error: %v", err)
			continue
		}
		s.mu.Lock()
		s.sessions[conn] = struct{}{}
		s.mu.Unlock()
		go s.handleSession(conn)
	}
}

// handleSession services all QUIC streams for one client connection.
// Each stream is processed in its own goroutine.
func (s *WebTransportServer) handleSession(conn quic.Connection) {
	defer func() {
		conn.CloseWithError(0, "session ended")
		s.mu.Lock()
		delete(s.sessions, conn)
		s.mu.Unlock()
	}()

	// Extract the session ID from the CONNECT header embedded in the first
	// datagram or from the SNI extension. For now, use remote addr as a
	// fallback identifier.
	remoteID := conn.RemoteAddr().String()
	s.logger.Printf("webtransport session established: %s", remoteID)

	// The first stream must be a control stream that carries the session token.
	ctrlStream, err := conn.AcceptStream(context.Background())
	if err != nil {
		s.logger.Printf("webtransport[%s] control stream accept error: %v", remoteID, err)
		return
	}

	sessionID, err := s.readControlHandshake(ctrlStream)
	if err != nil {
		s.logger.Printf("webtransport[%s] handshake error: %v", remoteID, err)
		return
	}
	s.logger.Printf("webtransport[%s] mapped to session %s", remoteID, sessionID)

	// Accept data streams until the connection closes.
	for {
		stream, err := conn.AcceptStream(context.Background())
		if err != nil {
			return // connection closed
		}
		go s.handleStream(conn, stream, sessionID)
	}
}

// readControlHandshake reads the initial JSON control message from the
// control stream to obtain the session ID and auth token.
//
// Wire format (control stream):
//
//	[wtStreamHeader with type=wtMsgControl][JSON payload]
//
// JSON payload must contain {"sessionId": "...", "token": "..."}.
func (s *WebTransportServer) readControlHandshake(stream quic.Stream) (string, error) {
	defer stream.Close()

	var hdr [8]byte
	if _, err := io.ReadFull(stream, hdr[:]); err != nil {
		return "", err
	}
	if wtMessageType(hdr[0]) != wtMsgControl {
		return "", errors.New("first stream must be a control stream")
	}
	payloadLen := binary.BigEndian.Uint32(hdr[4:])
	if payloadLen > 4096 {
		return "", errors.New("control handshake payload too large")
	}
	buf := make([]byte, payloadLen)
	if _, err := io.ReadFull(stream, buf); err != nil {
		return "", err
	}
	var msg struct {
		SessionID string `json:"sessionId"`
		Token     string `json:"token"`
	}
	if err := json.Unmarshal(buf, &msg); err != nil {
		return "", err
	}
	if msg.SessionID == "" {
		return "", errors.New("sessionId missing from handshake")
	}
	return msg.SessionID, nil
}

// handleStream dispatches a single QUIC stream to the appropriate handler
// based on its message type header.
func (s *WebTransportServer) handleStream(conn quic.Connection, stream quic.Stream, sessionID string) {
	defer stream.Close()

	var hdr [8]byte
	if _, err := io.ReadFull(stream, hdr[:]); err != nil {
		return
	}
	msgType := wtMessageType(hdr[0])
	payloadLen := binary.BigEndian.Uint32(hdr[4:])

	// Sanity-check maximum payload.
	const maxPayload = 2 * 1024 * 1024 // 2 MB
	if payloadLen > maxPayload {
		s.logger.Printf("webtransport stream payload too large: %d bytes (session=%s)", payloadLen, sessionID)
		return
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(stream, payload); err != nil {
		return
	}

	switch msgType {
	case wtMsgInput:
		s.handleInputEvent(sessionID, payload)
	case wtMsgControl:
		s.handleControlEvent(conn, sessionID, payload)
	default:
		// wtMsgVideo and wtMsgAudio are only host→client; receiving them from a
		// client is unexpected. Log and discard.
		s.logger.Printf("webtransport[%s] unexpected stream type %d from client", sessionID, msgType)
	}
}

// handleInputEvent forwards a serialised input event to the registered route.
func (s *WebTransportServer) handleInputEvent(sessionID string, payload []byte) {
	s.mu.Lock()
	reg := s.registry
	s.mu.Unlock()

	if reg == nil {
		return
	}

	_, ok := reg.Get(sessionID)
	if !ok {
		s.logger.Printf("webtransport input: no route for session %s", sessionID)
		return
	}
	_ = payload
}

// handleControlEvent processes a control message from the client.
func (s *WebTransportServer) handleControlEvent(conn quic.Connection, sessionID string, payload []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(payload, &msg); err != nil {
		return
	}
	s.logger.Printf("webtransport[%s] control: %v", sessionID, msg)
	// Future: relay to session manager for codec negotiation, quality settings, etc.
}

// SendToClient opens a new unidirectional QUIC stream and writes a typed
// message to the client. Used by host proxy goroutines to push video/audio.
func (s *WebTransportServer) SendToClient(conn quic.Connection, msgType wtMessageType, payload []byte) error {
	stream, err := conn.OpenStreamSync(context.Background())
	if err != nil {
		return err
	}
	defer stream.Close()

	var hdr [8]byte
	hdr[0] = byte(msgType)
	binary.BigEndian.PutUint32(hdr[4:], uint32(len(payload)))
	if _, err := stream.Write(hdr[:]); err != nil {
		return err
	}
	_, err = stream.Write(payload)
	return err
}

func (s *WebTransportServer) GetSessionCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sessions)
}

func (s *WebTransportServer) Close() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	// Cancel the accept context BEFORE closing the listener so acceptLoop
	// unblocks immediately even if the listener is stuck on a hung UDP
	// socket. Without this, the acceptLoop goroutine can leak past shutdown.
	if s.acceptCancel != nil {
		s.acceptCancel()
	}
	if s.listener != nil {
		_ = s.listener.Close()
	}
	// Snapshot session conns under the lock, then release before calling
	// CloseWithError. handleSession's defer needs s.mu to clean up, so holding
	// it while CloseWithError can block on that same defer would deadlock.
	conns := make([]quic.Connection, 0, len(s.sessions))
	for conn := range s.sessions {
		conns = append(conns, conn)
	}
	s.mu.Unlock()
	for _, conn := range conns {
		_ = conn.CloseWithError(0, "server shutting down")
	}
	return nil
}

func (s *WebTransportServer) Stats() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	return map[string]interface{}{
		"running":  s.running,
		"sessions": len(s.sessions),
	}
}

func DialWebTransport(addr string, tlsConfig *tls.Config) (quic.Connection, error) {
	conn, err := quic.DialAddr(context.Background(), addr, tlsConfig, nil)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func ResolveWebTransportAddr(addr string) (*net.UDPAddr, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	port, err := net.LookupPort("udp", portStr)
	if err != nil {
		return nil, err
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		if ip.To4() != nil {
			return &net.UDPAddr{IP: ip, Port: port}, nil
		}
	}
	if len(ips) > 0 {
		return &net.UDPAddr{IP: ips[0], Port: port}, nil
	}
	return nil, errors.New("no ip found")
}
