package input

import (
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"
)

type InputManager struct {
	logger        *log.Logger
	sessions      map[string]*SessionInput
	sessionsMutex sync.RWMutex
}

type SessionInput struct {
	SessionID    string
	Active       bool
	InputDevices map[string]InputDevice
	Mutex        sync.RWMutex
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type InputDevice struct {
	ID          string
	Kind        string // 'keyboard', 'mouse', 'touch', 'xbox-controller', 'playstation-controller', 'generic-gamepad'
	Connected   bool
	LastActive  time.Time
	Capabilities DeviceCapabilities
}

type DeviceCapabilities struct {
	ButtonCount     int
	AxisCount       int
	SupportsRumble  bool
	SupportsMotion  bool
	SupportsTouch   bool
	SupportsPressure bool
}

type InputEvent struct {
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

type MouseMoveEvent struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	DeltaX float64 `json:"deltaX"`
	DeltaY float64 `json:"deltaY"`
}

type MouseButtonEvent struct {
	Button  string `json:"button"` // 'left', 'right', 'middle'
	Pressed bool   `json:"pressed"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
}

type KeyboardEvent struct {
	Key      string `json:"key"`
	Code     string `json:"code"`
	Pressed  bool   `json:"pressed"`
	CtrlKey  bool   `json:"ctrlKey"`
	ShiftKey bool   `json:"shiftKey"`
	AltKey   bool   `json:"altKey"`
	MetaKey  bool   `json:"metaKey"`
}

type TouchEvent struct {
	Touches []TouchPoint `json:"touches"`
}

type TouchPoint struct {
	ID       int     `json:"id"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Pressure float64 `json:"pressure,omitempty"`
}

type GamepadEvent struct {
	GamepadID string            `json:"gamepadId"`
	Vendor    string            `json:"vendor,omitempty"` // 'xbox', 'playstation', 'generic'
	Axes      []float64         `json:"axes"`
	Buttons   []GamepadButton    `json:"buttons"`
	Timestamp int64             `json:"timestamp"`
}

type GamepadButton struct {
	Index  int     `json:"index"`
	Pressed bool   `json:"pressed"`
	Value  float64 `json:"value"`
}

type ScrollEvent struct {
	DeltaX float64 `json:"deltaX"`
	DeltaY float64 `json:"deltaY"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
}

func NewInputManager(logger *log.Logger) *InputManager {
	if logger == nil {
		logger = log.Default()
	}

	return &InputManager{
		logger:   logger,
		sessions: make(map[string]*SessionInput),
	}
}

func (m *InputManager) RegisterSession(sessionID string) *SessionInput {
	m.sessionsMutex.Lock()
	defer m.sessionsMutex.Unlock()

	session := &SessionInput{
		SessionID:    sessionID,
		Active:       true,
		InputDevices: make(map[string]InputDevice),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	m.sessions[sessionID] = session
	m.logger.Printf("registered input session %s", sessionID)

	return session
}

func (m *InputManager) UnregisterSession(sessionID string) {
	m.sessionsMutex.Lock()
	defer m.sessionsMutex.Unlock()

	if _, exists := m.sessions[sessionID]; exists {
		delete(m.sessions, sessionID)
		m.logger.Printf("unregistered input session %s", sessionID)
	}
}

func (m *InputManager) GetSession(sessionID string) (*SessionInput, error) {
	m.sessionsMutex.RLock()
	defer m.sessionsMutex.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, errors.New("session not found")
	}

	return session, nil
}

func (m *InputManager) RegisterDevice(sessionID string, device InputDevice) error {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return err
	}

	session.Mutex.Lock()
	defer session.Mutex.Unlock()

	device.Connected = true
	device.LastActive = time.Now()
	session.InputDevices[device.ID] = device
	session.UpdatedAt = time.Now()

	m.logger.Printf("registered device %s of kind %s for session %s", device.ID, device.Kind, sessionID)

	return nil
}

func (m *InputManager) UnregisterDevice(sessionID string, deviceID string) error {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return err
	}

	session.Mutex.Lock()
	defer session.Mutex.Unlock()

	if device, exists := session.InputDevices[deviceID]; exists {
		device.Connected = false
		session.InputDevices[deviceID] = device
		session.UpdatedAt = time.Now()
		m.logger.Printf("unregistered device %s for session %s", deviceID, sessionID)
	}

	return nil
}

func (m *InputManager) HandleInputEvent(sessionID string, event *InputEvent) error {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return err
	}

	session.Mutex.Lock()
	defer session.Mutex.Unlock()

	session.UpdatedAt = time.Now()

	// Update device last active time
	var deviceID string
	switch event.Type {
	case "input.mouse.move", "input.mouse.down", "input.mouse.up", "input.mouse.click", "input.mouse.dblclick", "input.scroll":
		deviceID = "mouse"
	case "input.keyboard.press", "input.keyboard.release":
		deviceID = "keyboard"
	case "input.touch.start", "input.touch.move", "input.touch.end", "input.touch.cancel":
		deviceID = "touch"
	case "input.gamepad":
		// Extract gamepad ID from event data
		var gamepadEvent GamepadEvent
		if err := json.Unmarshal(event.Data, &gamepadEvent); err == nil {
			deviceID = gamepadEvent.GamepadID
		}
	}

	if deviceID != "" {
		if device, exists := session.InputDevices[deviceID]; exists {
			device.LastActive = time.Now()
			session.InputDevices[deviceID] = device
		}
	}

	// Process the event based on type
	switch event.Type {
	case "input.mouse.move":
		return m.handleMouseMove(session, event)
	case "input.mouse.down", "input.mouse.up", "input.mouse.click", "input.mouse.dblclick":
		return m.handleMouseButton(session, event)
	case "input.keyboard.press", "input.keyboard.release":
		return m.handleKeyboard(session, event)
	case "input.scroll":
		return m.handleScroll(session, event)
	case "input.touch.start", "input.touch.move", "input.touch.end", "input.touch.cancel":
		return m.handleTouch(session, event)
	case "input.gamepad":
		return m.handleGamepad(session, event)
	default:
		return errors.New("unknown input event type")
	}
}

func (m *InputManager) handleMouseMove(session *SessionInput, event *InputEvent) error {
	var mouseEvent MouseMoveEvent
	if err := json.Unmarshal(event.Data, &mouseEvent); err != nil {
		return err
	}

	// In a real implementation, this would send the mouse event to the VM/desktop
	// For now, we just log it
	m.logger.Printf("session %s mouse move: x=%.2f, y=%.2f", session.SessionID, mouseEvent.X, mouseEvent.Y)

	return nil
}

func (m *InputManager) handleMouseButton(session *SessionInput, event *InputEvent) error {
	var buttonEvent MouseButtonEvent
	if err := json.Unmarshal(event.Data, &buttonEvent); err != nil {
		return err
	}

	m.logger.Printf("session %s mouse button: %s %s at (%.2f, %.2f)", session.SessionID, buttonEvent.Button, map[bool]string{true: "pressed", false: "released"}[buttonEvent.Pressed], buttonEvent.X, buttonEvent.Y)

	return nil
}

func (m *InputManager) handleKeyboard(session *SessionInput, event *InputEvent) error {
	var keyEvent KeyboardEvent
	if err := json.Unmarshal(event.Data, &keyEvent); err != nil {
		return err
	}

	m.logger.Printf("session %s keyboard: %s %s (modifiers: ctrl=%v, shift=%v, alt=%v, meta=%v)", 
		session.SessionID, 
		keyEvent.Code, 
		map[bool]string{true: "pressed", false: "released"}[keyEvent.Pressed],
		keyEvent.CtrlKey, keyEvent.ShiftKey, keyEvent.AltKey, keyEvent.MetaKey)

	return nil
}

func (m *InputManager) handleScroll(session *SessionInput, event *InputEvent) error {
	var scrollEvent ScrollEvent
	if err := json.Unmarshal(event.Data, &scrollEvent); err != nil {
		return err
	}

	m.logger.Printf("session %s scroll: deltaX=%.2f, deltaY=%.2f at (%.2f, %.2f)", session.SessionID, scrollEvent.DeltaX, scrollEvent.DeltaY, scrollEvent.X, scrollEvent.Y)

	return nil
}

func (m *InputManager) handleTouch(session *SessionInput, event *InputEvent) error {
	var touchEvent TouchEvent
	if err := json.Unmarshal(event.Data, &touchEvent); err != nil {
		return err
	}

	m.logger.Printf("session %s touch: %s with %d touch points", session.SessionID, event.Type, len(touchEvent.Touches))

	return nil
}

func (m *InputManager) handleGamepad(session *SessionInput, event *InputEvent) error {
	var gamepadEvent GamepadEvent
	if err := json.Unmarshal(event.Data, &gamepadEvent); err != nil {
		return err
	}

	m.logger.Printf("session %s gamepad: %s vendor=%s axes=%d buttons=%d", 
		session.SessionID, 
		gamepadEvent.GamepadID, 
		gamepadEvent.Vendor, 
		len(gamepadEvent.Axes), 
		len(gamepadEvent.Buttons))

	return nil
}

func (m *InputManager) GetActiveDevices(sessionID string) ([]InputDevice, error) {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	session.Mutex.RLock()
	defer session.Mutex.RUnlock()

	devices := make([]InputDevice, 0, len(session.InputDevices))
	for _, device := range session.InputDevices {
		if device.Connected {
			devices = append(devices, device)
		}
	}

	return devices, nil
}