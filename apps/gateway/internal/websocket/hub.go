package websocket

import "sync"

// Hub maintains the set of active WebSocket clients grouped by VM ID.
type Hub struct {
	mu       sync.RWMutex
	clients  map[string]map[*Client]bool // vmID -> set of clients
	register chan *Client
	remove   chan *Client
}

func NewHub() *Hub {
	return &Hub{
		clients:  make(map[string]map[*Client]bool),
		register: make(chan *Client),
		remove:   make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.clients[client.VMID] == nil {
				h.clients[client.VMID] = make(map[*Client]bool)
			}
			h.clients[client.VMID][client] = true
			h.mu.Unlock()

		case client := <-h.remove:
			h.mu.Lock()
			if clients, ok := h.clients[client.VMID]; ok {
				delete(clients, client)
				if len(clients) == 0 {
					delete(h.clients, client.VMID)
				}
			}
			h.mu.Unlock()
			close(client.Send)
		}
	}
}

// BroadcastToVM sends a message to all clients connected to a specific VM.
func (h *Hub) BroadcastToVM(vmID string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, ok := h.clients[vmID]; ok {
		for client := range clients {
			select {
			case client.Send <- message:
			default:
				// Client buffer full, schedule removal
				go func(c *Client) { h.remove <- c }(client)
			}
		}
	}
}
