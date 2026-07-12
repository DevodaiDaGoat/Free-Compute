package webrtc

import (
	"encoding/binary"
	"fmt"
	"sync"
)

type DataChannelID uint16

const (
	DCtrlSession  DataChannelID = 0
	DCtrlInput    DataChannelID = 1
	DCtrlGamepad  DataChannelID = 2
	DCtrlAudioMeta DataChannelID = 3
	DCtrlClipboard DataChannelID = 4
	DCtrlFileXfer DataChannelID = 5
	DCtrlTelemetry DataChannelID = 6
)

type ChannelConfig struct {
	ID        DataChannelID
	Label     string
	Reliable  bool
	Ordered   bool
	Priority  int
}

var defaultChannels = []ChannelConfig{
	{ID: DCtrlSession, Label: "session", Reliable: true, Ordered: true, Priority: 0},
	{ID: DCtrlInput, Label: "input", Reliable: false, Ordered: false, Priority: 1},
	{ID: DCtrlGamepad, Label: "gamepad", Reliable: false, Ordered: false, Priority: 1},
	{ID: DCtrlAudioMeta, Label: "audio-meta", Reliable: true, Ordered: true, Priority: 2},
	{ID: DCtrlClipboard, Label: "clipboard", Reliable: true, Ordered: true, Priority: 2},
	{ID: DCtrlFileXfer, Label: "file-xfer", Reliable: true, Ordered: true, Priority: 3},
	{ID: DCtrlTelemetry, Label: "telemetry", Reliable: false, Ordered: false, Priority: 3},
}

type MultiChannelManager struct {
	mu       sync.Mutex
	channels map[DataChannelID]*ManagedChannel
}

type ManagedChannel struct {
	Config ChannelConfig
	Send   func([]byte) error
	OnMsg  func([]byte)
}

func NewMultiChannelManager() *MultiChannelManager {
	return &MultiChannelManager{
		channels: make(map[DataChannelID]*ManagedChannel),
	}
}

func (m *MultiChannelManager) RegisterChannel(id DataChannelID, sendFn func([]byte) error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, cfg := range defaultChannels {
		if cfg.ID == id {
			m.channels[id] = &ManagedChannel{
				Config: cfg,
				Send:   sendFn,
			}
			return
		}
	}
}

func (m *MultiChannelManager) Send(id DataChannelID, data []byte) error {
	m.mu.Lock()
	ch, ok := m.channels[id]
	m.mu.Unlock()
	if !ok {
		// Was returning nil silently — callers logged "sent" while the frame
		// was black-holed. Now surfaces missing-channel as a real error so a
		// misconfigured session doesn't look successful.
		return fmt.Errorf("data channel %d not registered", id)
	}

	frame := make([]byte, 2+len(data))
	binary.BigEndian.PutUint16(frame[:2], uint16(id))
	copy(frame[2:], data)

	return ch.Send(frame)
}

func InputEventFrame(seq uint32, eventType byte, data []byte) []byte {
	frame := make([]byte, 5+len(data))
	binary.LittleEndian.PutUint32(frame[:4], seq)
	frame[4] = eventType
	copy(frame[5:], data)
	return frame
}

func ParseInputEventFrame(frame []byte) (seq uint32, eventType byte, data []byte, ok bool) {
	if len(frame) < 5 {
		return 0, 0, nil, false
	}
	seq = binary.LittleEndian.Uint32(frame[:4])
	eventType = frame[4]
	return seq, eventType, frame[5:], true
}
