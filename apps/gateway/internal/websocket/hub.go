package websocket

import "sync"

// Hub maintains the set of active clients grouped by VM session and
// coordinates registration, unregistration and broadcasting.
type Hub struct {
	mu sync.RWMutex
	// clients maps a VM ID to the set of clients streaming that session.
	clients map[string]map[*Client]struct{}

	register   chan *Client
	unregister chan *Client
}

// NewHub constructs an empty Hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]map[*Client]struct{}),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run processes registration and unregistration events until stop is closed.
func (h *Hub) Run(stop <-chan struct{}) {
	for {
		select {
		case c := <-h.register:
			h.add(c)
		case c := <-h.unregister:
			h.remove(c)
		case <-stop:
			return
		}
	}
}

// Register enqueues a client for registration with the hub.
func (h *Hub) Register(c *Client) { h.register <- c }

// Unregister enqueues a client for removal from the hub.
func (h *Hub) Unregister(c *Client) { h.unregister <- c }

func (h *Hub) add(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[c.vmID] == nil {
		h.clients[c.vmID] = make(map[*Client]struct{})
	}
	h.clients[c.vmID][c] = struct{}{}
}

func (h *Hub) remove(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set, ok := h.clients[c.vmID]; ok {
		if _, ok := set[c]; ok {
			delete(set, c)
			close(c.send)
		}
		if len(set) == 0 {
			delete(h.clients, c.vmID)
		}
	}
}

// Broadcast sends a message to every client subscribed to the given VM session.
// Clients whose send buffers are full are dropped to avoid blocking the hub.
func (h *Hub) Broadcast(vmID string, msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients[vmID] {
		select {
		case c.send <- msg:
		default:
			// Slow consumer: skip rather than block the broadcast.
		}
	}
}
