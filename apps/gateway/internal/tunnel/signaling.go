package tunnel

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxSignalMessagesPerRoom = 256
	maxSignalMessageBytes    = 64 * 1024
	signalLongPollTimeout    = 25 * time.Second
	signalRoomTTL            = 10 * time.Minute
	signalCleanupInterval    = time.Minute
)

type SignalMessage struct {
	Seq       int64           `json:"seq"`
	From      string          `json:"from,omitempty"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"createdAt"`
}

type signalRoom struct {
	seq       int64
	messages  []SignalMessage
	notify    chan struct{}
	updatedAt time.Time
}

type signalStore struct {
	mu    sync.Mutex
	rooms map[string]*signalRoom
}

func newSignalStore() signalStore {
	return signalStore{rooms: map[string]*signalRoom{}}
}

func (s *Server) handleSignal(w http.ResponseWriter, r *http.Request) {
	routeID, roomID := signalRouteAndRoom(r.URL.Path)
	if routeID == "" || roomID == "" {
		http.NotFound(w, r)
		return
	}

	route, ok := s.registry.Get(routeID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if !s.authorize(route, w, r) {
		return
	}
	if route.Protocol != ProtocolWebRTC && route.Protocol != ProtocolP2P {
		http.Error(w, "route does not support signaling", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.handleSignalPoll(w, r, routeID, roomID)
	case http.MethodPost:
		s.handleSignalPost(w, r, routeID, roomID)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSignalPoll(w http.ResponseWriter, r *http.Request, routeID string, roomID string) {
	w.Header().Set("Cache-Control", "no-store")
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	roomKey := routeID + "/" + roomID
	deadline := time.NewTimer(signalLongPollTimeout)
	defer deadline.Stop()

	for {
		messages, notify := s.signalStore.messagesAfter(roomKey, after)
		if len(messages) > 0 {
			writeJSON(w, http.StatusOK, map[string]any{"messages": messages})
			return
		}

		select {
		case <-r.Context().Done():
			return
		case <-deadline.C:
			writeJSON(w, http.StatusOK, map[string]any{"messages": []SignalMessage{}})
			return
		case <-notify:
		}
	}
}

func (s *Server) handleSignalPost(w http.ResponseWriter, r *http.Request, routeID string, roomID string) {
	w.Header().Set("Cache-Control", "no-store")
	body := http.MaxBytesReader(w, r.Body, maxSignalMessageBytes)
	defer body.Close()

	var incoming struct {
		From    string          `json:"from,omitempty"`
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.NewDecoder(body).Decode(&incoming); err != nil {
		http.Error(w, "invalid signaling message", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(incoming.Type) == "" {
		http.Error(w, "signal type is required", http.StatusBadRequest)
		return
	}

	message := SignalMessage{
		From:      incoming.From,
		Type:      incoming.Type,
		Payload:   incoming.Payload,
		CreatedAt: time.Now().UTC(),
	}
	message = s.signalStore.append(routeID+"/"+roomID, message)
	writeJSON(w, http.StatusAccepted, message)
}

func signalRouteAndRoom(path string) (string, string) {
	trimmed := strings.TrimPrefix(path, "/signal/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 3 || parts[1] != "rooms" {
		return "", ""
	}

	return parts[0], parts[2]
}

// pendingRoomCh is a sentinel channel returned to pollers when the target room
// does not exist yet (or was swept). It is intentionally never closed so the
// poll loop just waits for its deadline instead of spinning against a
// pre-closed channel and burning CPU inside a lock/unlock loop.
var pendingRoomCh = make(chan struct{})

func (s *signalStore) messagesAfter(roomKey string, after int64) ([]SignalMessage, <-chan struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Do NOT auto-create rooms on read. If sweep just deleted the room and
	// this GET recreated it, sweep+poll would resurrect swept rooms forever.
	// Only append() (write) creates rooms.
	room := s.rooms[roomKey]
	if room == nil {
		// Returning a closed channel here would make the caller's select fire
		// immediately and busy-loop back to messagesAfter — a CPU-burning
		// spin against the store mutex. Return the never-closing sentinel so
		// the poller waits for its long-poll deadline instead.
		return nil, pendingRoomCh
	}
	messages := make([]SignalMessage, 0)
	for _, message := range room.messages {
		if message.Seq > after {
			messages = append(messages, message)
		}
	}

	return messages, room.notify
}

func (s *signalStore) append(roomKey string, message SignalMessage) SignalMessage {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	room := s.getOrCreateLocked(roomKey, now)
	room.seq++
	message.Seq = room.seq
	room.messages = append(room.messages, message)
	if len(room.messages) > maxSignalMessagesPerRoom {
		room.messages = room.messages[len(room.messages)-maxSignalMessagesPerRoom:]
	}

	close(room.notify)
	room.notify = make(chan struct{})
	return message
}

func (s *signalStore) getOrCreateLocked(roomKey string, now time.Time) *signalRoom {
	room := s.rooms[roomKey]
	if room == nil {
		room = &signalRoom{notify: make(chan struct{}), updatedAt: now}
		s.rooms[roomKey] = room
	}
	room.updatedAt = now

	return room
}

// sweepLoop periodically removes expired signaling rooms. It runs for the
// lifetime of the server context and exits cleanly when ctx is cancelled.
func (s *signalStore) sweepLoop(ctx context.Context) {
	ticker := time.NewTicker(signalCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sweep()
		}
	}
}

func (s *signalStore) sweep() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for roomKey, room := range s.rooms {
		if now.Sub(room.updatedAt) > signalRoomTTL {
			close(room.notify)
			delete(s.rooms, roomKey)
		}
	}
}
