package websocket

import (
	"net/http"
	"time"

	"github.com/DevodaiDaGoat/Free-Compute/apps/gateway/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	ws "github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

var upgrader = ws.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Origin validation is handled by CORS middleware
		return true
	},
}

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

// Client represents a single WebSocket connection.
type Client struct {
	Hub    *Hub
	Conn   *ws.Conn
	Send   chan []byte
	VMID   string
	UserID string
}

// HandleConnection authenticates and upgrades a WebSocket connection.
// SECURITY: Requires a short-lived connection token in query params.
func HandleConnection(hub *Hub, cfg *config.Config, w http.ResponseWriter, r *http.Request) {
	vmID := chi.URLParam(r, "vm_id")
	if vmID == "" {
		http.Error(w, "missing vm_id", http.StatusBadRequest)
		return
	}

	// SECURITY: Validate connection token from query parameter
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	claims, err := validateConnectionToken(cfg, token, vmID)
	if err != nil {
		log.Debug().Err(err).Msg("invalid connection token")
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("websocket upgrade failed")
		return
	}

	client := &Client{
		Hub:    hub,
		Conn:   conn,
		Send:   make(chan []byte, 256),
		VMID:   vmID,
		UserID: claims.Subject,
	}

	hub.register <- client

	go client.writePump()
	go client.readPump()
}

// validateConnectionToken verifies a short-lived token scoped to a specific VM.
func validateConnectionToken(cfg *config.Config, tokenStr, vmID string) (*jwt.RegisteredClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &streamClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		pubKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(cfg.JWTPublicKey))
		if err != nil {
			return nil, err
		}
		return pubKey, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*streamClaims)
	if !ok || !token.Valid {
		return nil, jwt.ErrSignatureInvalid
	}

	// SECURITY: Verify the token is scoped to this specific VM
	if claims.VMID != vmID {
		return nil, jwt.ErrSignatureInvalid
	}

	return &claims.RegisteredClaims, nil
}

type streamClaims struct {
	jwt.RegisteredClaims
	VMID string `json:"vm_id"`
}

func (c *Client) readPump() {
	defer func() {
		c.Hub.remove <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}
		// Forward input messages to the VM host
		c.Hub.BroadcastToVM(c.VMID, message)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(ws.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(ws.BinaryMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(ws.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
