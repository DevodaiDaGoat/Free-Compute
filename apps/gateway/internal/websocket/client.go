package websocket

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

const (
	// writeWait is the time allowed to write a message to the peer.
	writeWait = 10 * time.Second
	// pongWait is the time allowed to read the next pong from the peer.
	pongWait = 60 * time.Second
	// pingPeriod is how often to send pings; must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
	// maxMessageSize is the maximum inbound message size in bytes.
	maxMessageSize = 1 << 20
)

// Client represents a single WebSocket connection streaming a VM session.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
	vmID string
}

// UpgradeHandler returns an http.HandlerFunc that upgrades the connection to a
// WebSocket, registers the resulting client with the hub, and starts its pumps.
// The allowedOrigins slice controls the CheckOrigin policy.
func UpgradeHandler(hub *Hub, allowedOrigins []string) http.HandlerFunc {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     originChecker(allowedOrigins),
	}

	return func(w http.ResponseWriter, r *http.Request) {
		vmID := chi.URLParam(r, "vmID")
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			// Upgrade already writes an error response on failure.
			return
		}

		client := &Client{
			hub:  hub,
			conn: conn,
			send: make(chan []byte, 256),
			vmID: vmID,
		}
		hub.Register(client)

		go client.writePump()
		go client.readPump()
	}
}

// readPump pumps messages from the WebSocket connection to the hub.
func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister(c)
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// Log-worthy in a real implementation; skeleton just returns.
			}
			return
		}
		// Placeholder: echo inbound messages to all clients on the same VM.
		c.hub.Broadcast(c.vmID, data)
	}
}

// writePump pumps messages from the send channel to the WebSocket connection
// and sends periodic pings to keep the connection alive.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// originChecker returns a CheckOrigin function permitting only the configured
// origins. An empty list permits all origins (useful for local development).
func originChecker(allowedOrigins []string) func(*http.Request) bool {
	if len(allowedOrigins) == 0 {
		return func(*http.Request) bool { return true }
	}
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		_, ok := allowed[origin]
		return ok
	}
}
