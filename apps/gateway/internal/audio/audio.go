package audio

import (
	"encoding/binary"
	"errors"
	"io"
	"log"
	"math"
	"sync"
	"time"
)

const (
	defaultSampleRate   = 48000
	defaultChannels     = 2
	defaultFrameSize    = 960 // 20ms at 48kHz
	maxBufferSize      = 1024 * 1024 // 1MB max buffer
	silenceThreshold   = 100
	opusDefaultBitrate = 64000 // 64kbps
)

type AudioStreamer struct {
	logger        *log.Logger
	sessions      map[string]*AudioSession
	sessionsMutex sync.RWMutex
}

type AudioSession struct {
	SessionID      string
	Active         bool
	Config         AudioConfig
	Buffer         *AudioBuffer
	Mutex          sync.RWMutex
	CreatedAt      time.Time
	UpdatedAt      time.Time
	BytesSent      uint64
	PacketsSent    uint64
	LastPacketSent time.Time
}

type AudioConfig struct {
	SampleRate   int
	Channels     int
	FrameSize    int
	Codec        string // 'opus', 'aac'
	Bitrate      int
	Latency      time.Duration
	EnableAEC    bool // Acoustic Echo Cancellation
	EnableNS     bool // Noise Suppression
	EnableAGC    bool // Automatic Gain Control
}

type AudioBuffer struct {
	data         []byte
	writePos     int
	readPos      int
	size         int
	mutex        sync.Mutex
	ready        bool
}

type AudioFrame struct {
	Data       []byte
	Timestamp  time.Time
	Sequence   uint32
	SampleRate int
	Channels   int
}

func NewAudioStreamer(logger *log.Logger) *AudioStreamer {
	if logger == nil {
		logger = log.Default()
	}

	return &AudioStreamer{
		logger:   logger,
		sessions: make(map[string]*AudioSession),
	}
}

func (s *AudioStreamer) CreateSession(sessionID string, config AudioConfig) (*AudioSession, error) {
	s.sessionsMutex.Lock()
	defer s.sessionsMutex.Unlock()

	// Set defaults
	if config.SampleRate == 0 {
		config.SampleRate = defaultSampleRate
	}
	if config.Channels == 0 {
		config.Channels = defaultChannels
	}
	if config.FrameSize == 0 {
		config.FrameSize = defaultFrameSize
	}
	if config.Codec == "" {
		config.Codec = "opus"
	}
	if config.Bitrate == 0 {
		config.Bitrate = opusDefaultBitrate
	}
	if config.Latency == 0 {
		config.Latency = 20 * time.Millisecond
	}

	session := &AudioSession{
		SessionID: sessionID,
		Active:    true,
		Config:    config,
		Buffer:    NewAudioBuffer(maxBufferSize),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	s.sessions[sessionID] = session
	s.logger.Printf("created audio session %s (codec=%s, sampleRate=%d, channels=%d)", 
		sessionID, config.Codec, config.SampleRate, config.Channels)

	return session, nil
}

func (s *AudioStreamer) GetSession(sessionID string) (*AudioSession, error) {
	s.sessionsMutex.RLock()
	defer s.sessionsMutex.RUnlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return nil, errors.New("audio session not found")
	}

	return session, nil
}

func (s *AudioStreamer) EndSession(sessionID string) error {
	s.sessionsMutex.Lock()
	defer s.sessionsMutex.Unlock()

	if _, exists := s.sessions[sessionID]; exists {
		delete(s.sessions, sessionID)
		s.logger.Printf("ended audio session %s", sessionID)
		return nil
	}

	return errors.New("audio session not found")
}

func (s *AudioStreamer) WriteFrame(sessionID string, frame *AudioFrame) error {
	session, err := s.GetSession(sessionID)
	if err != nil {
		return err
	}

	session.Mutex.Lock()
	defer session.Mutex.Unlock()

	if !session.Active {
		return errors.New("session is not active")
	}

	// Write to buffer
	if err := session.Buffer.Write(frame.Data); err != nil {
		return err
	}

	session.BytesSent += uint64(len(frame.Data))
	session.PacketsSent++
	session.LastPacketSent = time.Now()
	session.UpdatedAt = time.Now()

	return nil
}

func (s *AudioStreamer) ReadFrame(sessionID string) (*AudioFrame, error) {
	session, err := s.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	session.Mutex.Lock()
	defer session.Mutex.Unlock()

	if !session.Active {
		return nil, errors.New("session is not active")
	}

	data, err := session.Buffer.Read(session.Config.FrameSize * session.Config.Channels * 2) // 16-bit samples
	if err != nil {
		return nil, err
	}

	frame := &AudioFrame{
		Data:       data,
		Timestamp:  time.Now(),
		SampleRate: session.Config.SampleRate,
		Channels:   session.Config.Channels,
	}

	return frame, nil
}

func (s *AudioStreamer) ProcessAudioData(sessionID string, inputData []byte) ([]byte, error) {
	session, err := s.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	session.Mutex.Lock()
	defer session.Mutex.Unlock()

	// Apply audio processing if enabled
	outputData := inputData

	if session.Config.EnableAEC {
		// Acoustic Echo Cancellation would be applied here
		// For now, we just pass through
	}

	if session.Config.EnableNS {
		// Noise Suppression would be applied here
		// For now, we just pass through
	}

	if session.Config.EnableAGC {
		// Automatic Gain Control would be applied here
		// For now, we just pass through
	}

	return outputData, nil
}

func (s *AudioStreamer) GetStats(sessionID string) (map[string]interface{}, error) {
	session, err := s.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	session.Mutex.RLock()
	defer session.Mutex.RUnlock()

	return map[string]interface{}{
		"bytesSent":       session.BytesSent,
		"packetsSent":     session.PacketsSent,
		"lastPacketSent":  session.LastPacketSent,
		"bufferSize":      session.Buffer.size,
		"bufferAvailable": session.Buffer.Available(),
		"active":          session.Active,
	}, nil
}

func NewAudioBuffer(size int) *AudioBuffer {
	return &AudioBuffer{
		data:     make([]byte, size),
		size:     size,
		writePos: 0,
		readPos:  0,
	}
}

func (b *AudioBuffer) Write(data []byte) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	n := len(data)
	if n >= b.size {
		// Fill the buffer with the tail. readPos moves to writePos+1 so the
		// buffer is exactly full and Read returns size-1 bytes (classic ring
		// discipline). The previous code left readPos at the old start after
		// a full-buffer write, so subsequent reads reported 0 bytes.
		data = data[n-b.size:]
		copy(b.data, data)
		b.writePos = 0
		if b.size > 0 {
			b.readPos = 1 % b.size
		}
		b.ready = true
		return nil
	}
	if n == 0 {
		return nil
	}
	// Snapshot the pre-write available count so we can detect overtake by a
	// simple bytes-available comparison instead of trying to reason about wrap
	// arithmetic (the previous implementation had ambiguous precedence and
	// admitted false positives/negatives on wrap boundaries).
	writable := b.size - b.availableLocked()
	overtook := b.ready && n >= writable

	firstLen := copy(b.data[b.writePos:], data)
	if firstLen < n {
		copy(b.data, data[firstLen:])
	}
	b.writePos = (b.writePos + n) % b.size

	if overtook {
		// Push readPos forward so it never sits on freshly-overwritten bytes.
		b.readPos = (b.writePos + 1) % b.size
	}
	b.ready = true
	return nil
}

func (b *AudioBuffer) Read(size int) ([]byte, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if !b.ready {
		return nil, io.EOF
	}

	available := b.availableLocked()
	if available == 0 {
		return nil, io.EOF
	}

	readSize := size
	if available < readSize {
		readSize = available
	}

	result := make([]byte, readSize)
	firstLen := copy(result, b.data[b.readPos:])
	copy(result[firstLen:], b.data)
	b.readPos = (b.readPos + readSize) % b.size
	return result, nil
}

// availableLocked computes the number of bytes available to read.
// Must be called with b.mutex held.
func (b *AudioBuffer) availableLocked() int {
	if b.writePos >= b.readPos {
		return b.writePos - b.readPos
	}
	return b.size - b.readPos + b.writePos
}

func (b *AudioBuffer) Available() int {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.availableLocked()
}

func (b *AudioBuffer) Clear() {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.writePos = 0
	b.readPos = 0
	b.ready = false
}

// Audio utilities
func EncodePCM16Float(data []byte) []float32 {
	samples := make([]float32, len(data)/2)
	for i := 0; i < len(samples); i++ {
		sample := int16(binary.LittleEndian.Uint16(data[i*2 : i*2+2]))
		samples[i] = float32(sample) / 32768.0
	}
	return samples
}

func DecodePCM16Float(samples []float32) []byte {
	data := make([]byte, len(samples)*2)
	for i, sample := range samples {
		if sample > 1.0 {
			sample = 1.0
		} else if sample < -1.0 {
			sample = -1.0
		}
		intSample := int16(sample * 32767.0)
		binary.LittleEndian.PutUint16(data[i*2:i*2+2], uint16(intSample))
	}
	return data
}

func DetectSilence(data []byte, threshold int) bool {
	samples := EncodePCM16Float(data)
	for _, sample := range samples {
		if int(sample*32768) > threshold || int(sample*32768) < -threshold {
			return false
		}
	}
	return true
}

func CalculateRMS(samples []float32) float32 {
	if len(samples) == 0 {
		return 0.0
	}
	sum := float32(0)
	for _, sample := range samples {
		sum += sample * sample
	}
	return float32(math.Sqrt(float64(sum / float32(len(samples)))))
}