package websocket

import "encoding/json"

// MessageType enumerates the kinds of messages exchanged over a stream.
type MessageType string

const (
	// TypeInput carries a serialized browser input event (keyboard, mouse, etc.).
	TypeInput MessageType = "input"
	// TypeSystem carries a system/control message (state changes, notices).
	TypeSystem MessageType = "system"
	// TypeError carries an error notice to the client.
	TypeError MessageType = "error"
	// TypePing / TypePong are used for application-level keepalive.
	TypePing MessageType = "ping"
	TypePong MessageType = "pong"
)

// Message is the envelope for all stream messages. Payload is left as raw JSON
// so consumers can decode it into the concrete type implied by Type.
type Message struct {
	Type    MessageType     `json:"type"`
	VMID    string          `json:"vm_id,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// InputEvent represents a serialized browser input event forwarded to a VM.
type InputEvent struct {
	Kind string  `json:"kind"` // e.g. "keydown", "mousemove", "click"
	Code string  `json:"code,omitempty"`
	X    float64 `json:"x,omitempty"`
	Y    float64 `json:"y,omitempty"`
}

// SystemEvent represents a control/status message.
type SystemEvent struct {
	Event  string `json:"event"`
	Detail string `json:"detail,omitempty"`
}

// Encode serializes a Message to its JSON wire representation.
func Encode(m Message) ([]byte, error) {
	return json.Marshal(m)
}

// Decode parses a JSON wire representation into a Message.
func Decode(data []byte) (Message, error) {
	var m Message
	err := json.Unmarshal(data, &m)
	return m, err
}
