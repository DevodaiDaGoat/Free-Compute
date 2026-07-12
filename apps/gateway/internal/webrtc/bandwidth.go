package webrtc

import (
	"encoding/json"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// bweSampleInterval is how often the estimator samples session stats.
const bweSampleInterval = 500 * time.Millisecond

// bweWindowSamples is the number of samples used for trend detection.
const bweWindowSamples = 6 // 3 seconds

// bitrateMin / bitrateMax are the adaptive range in bits per second.
const bitrateMin uint32 = 100_000    // 100 Kbps
const bitrateMax uint32 = 50_000_000 // 50 Mbps

// BWE state constants (mirrors Google GCC state machine).
type bweState int

const (
	bweNormal    bweState = iota // No congestion detected.
	bweUnderuse                  // Bandwidth available above current send rate.
	bweOveruse                   // Congestion detected, reduce bitrate.
)

// CodecQuality ranks codecs from worst (0) to best (3) quality for a given
// network condition. The estimator downgrades in bad conditions and upgrades
// in good conditions.
//
// Names match the short codec identifiers used by Session.VideoCodec (see
// selectVideoCodec in webrtc.go). Do not use the MIME-style "video/H264"
// form here — codecIndex compares against Session.VideoCodec directly.
var codecQualityLadder = []string{
	"vp8",
	"vp9",
	"h264",
	"h265",
}

// BandwidthEstimator runs the GCC-style adaptive bitrate control loop for
// a single WebRTC session. It reads SessionStats produced by startStatsCollector
// and adjusts the encoder bitrate + signals codec switches via the DataChannel.
type BandwidthEstimator struct {
	sessionID string
	session   *Session
	logger    *log.Logger

	mu            sync.Mutex
	currentBps    uint32 // current target bitrate in bps
	state         bweState
	lossHistory   []float64 // recent packet-loss-percent samples
	jitterHistory []uint32  // recent jitter (ms) samples

	// Codec proposal dedup + cooldown. Avoids spamming the DataChannel and
	// gateway log with the same "propose h264 -> h265" every tick.
	lastProposedCodec string
	lastProposedAt    time.Time

	stopCh   chan struct{}
	stopOnce sync.Once
}

// codecProposalCooldown is the minimum interval between two codec-switch
// proposals for the same target. Prevents log flooding when steady-state
// conditions repeatedly satisfy the upgrade/downgrade criteria.
const codecProposalCooldown = 30 * time.Second

// NewBandwidthEstimator creates an estimator for the given session.
func NewBandwidthEstimator(session *Session, logger *log.Logger) *BandwidthEstimator {
	if logger == nil {
		logger = log.Default()
	}
	return &BandwidthEstimator{
		sessionID:     session.ID,
		session:       session,
		logger:        logger,
		currentBps:    5_000_000, // start at 5 Mbps
		state:         bweNormal,
		lossHistory:   make([]float64, 0, bweWindowSamples),
		jitterHistory: make([]uint32, 0, bweWindowSamples),
		stopCh:        make(chan struct{}),
	}
}

// Start launches the BWE control loop in its own goroutine.
func (b *BandwidthEstimator) Start() {
	go b.run()
}

// Stop signals the control loop to exit. Safe to call multiple times.
func (b *BandwidthEstimator) Stop() {
	b.stopOnce.Do(func() {
		close(b.stopCh)
	})
}

// SetTargetBitrate updates the current target bitrate (called externally, e.g.
// from RTCP REMB handler or admin override).
func (b *BandwidthEstimator) SetTargetBitrate(bps uint32) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.currentBps = clampBitrate(bps)
}

// TargetBitrate returns the current target bitrate in bits per second.
func (b *BandwidthEstimator) TargetBitrate() uint32 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.currentBps
}

func (b *BandwidthEstimator) run() {
	ticker := time.NewTicker(bweSampleInterval)
	defer ticker.Stop()
	for {
		select {
		case <-b.stopCh:
			return
		case <-ticker.C:
			b.tick()
		}
	}
}

// tick is called every bweSampleInterval to sample stats and adjust bitrate.
func (b *BandwidthEstimator) tick() {
	// PacketsReceived / BytesReceived / BytesSent / PacketsSent are mutated
	// via atomic ops from WriteVideoRTP/HandleMediaIngest/startStatsCollector,
	// so we must read them via atomic.LoadUint64 rather than a struct copy.
	// The remaining fields (PacketsLost, Jitter) are lock-protected.
	packetsReceived := atomic.LoadUint64(&b.session.Stats.PacketsReceived)
	b.session.Mutex.RLock()
	packetsLost := b.session.Stats.PacketsLost
	jitterMs := b.session.Stats.Jitter // already in ms from startStatsCollector
	codec := b.session.VideoCodec
	b.session.Mutex.RUnlock()

	// Compute loss percent from cumulative counters.
	// (PacketsLost is absolute; PacketsReceived is absolute — so the loss
	// fraction is Lost / (Received + Lost).)
	var lossPct float64
	total := packetsReceived + uint64(packetsLost)
	if total > 0 {
		lossPct = float64(packetsLost) / float64(total) * 100.0
	}

	b.mu.Lock()
	// Slide the history windows.
	if len(b.lossHistory) >= bweWindowSamples {
		b.lossHistory = b.lossHistory[1:]
	}
	b.lossHistory = append(b.lossHistory, lossPct)

	if len(b.jitterHistory) >= bweWindowSamples {
		b.jitterHistory = b.jitterHistory[1:]
	}
	b.jitterHistory = append(b.jitterHistory, jitterMs)

	avgLoss := average64(b.lossHistory)
	avgJitter := averageU32(b.jitterHistory)
	b.mu.Unlock()

	// --- Loss-based controller (GCC Phase 1) ---
	var newBps uint32
	b.mu.Lock()
	cur := b.currentBps
	b.mu.Unlock()

	switch {
	case avgLoss > 10.0:
		// Severe loss — halve the bitrate.
		newBps = clampBitrate(cur / 2)
		b.setState(bweOveruse)
	case avgLoss > 2.0:
		// Moderate loss — reduce by 8%.
		newBps = clampBitrate(uint32(float64(cur) * 0.92))
		b.setState(bweOveruse)
	case avgLoss < 0.5 && b.currentState() != bweOveruse:
		// Headroom — increase by 5% (multiplicative increase).
		newBps = clampBitrate(uint32(float64(cur) * 1.05))
		b.setState(bweUnderuse)
	default:
		newBps = cur
		b.setState(bweNormal)
	}

	b.mu.Lock()
	b.currentBps = newBps
	b.mu.Unlock()

	// Send a bitrate update event to the client over the data channel so the
	// browser-side renderer can display live stats.
	b.sendDataChannelEvent("bitrate-update", map[string]interface{}{
		"targetBps":   newBps,
		"lossPercent": avgLoss,
		"jitterMs":    avgJitter,
		"state":       b.currentState().String(),
	})

	// --- Adaptive codec switching based on jitter ---
	// Only adapt when we actually have media flowing. With no packets the
	// jitter/loss samples are trivially zero and the estimator would propose
	// upgrades indefinitely on every idle session.
	if packetsReceived > 0 {
		b.maybeAdaptCodec(avgJitter, avgLoss, codec)
	}
}

// maybeAdaptCodec proposes a codec downgrade or upgrade over the DataChannel.
//
// Quality ladder: VP8 (worst) → VP9 → H264 → H265 (best)
//  - Downgrade when jitter > 30ms sustained or loss > 10%.
//  - Upgrade  when jitter < 5ms and loss < 0.5% for the full window.
func (b *BandwidthEstimator) maybeAdaptCodec(avgJitter uint32, avgLoss float64, currentCodec string) {
	idx := codecIndex(currentCodec)
	if idx < 0 {
		return // unknown codec, skip
	}

	var targetIdx int
	switch {
	case avgJitter > 30 || avgLoss > 10.0:
		// Degrade to VP8 for severe conditions.
		targetIdx = 0
	case avgJitter > 15 || avgLoss > 2.0:
		// One step down.
		targetIdx = max(idx-1, 0)
	case avgJitter < 5 && avgLoss < 0.5:
		// One step up (only if full window is clean).
		if len(b.jitterHistory) == bweWindowSamples {
			targetIdx = min(idx+1, len(codecQualityLadder)-1)
		} else {
			targetIdx = idx
		}
	default:
		targetIdx = idx
	}

	if targetIdx != idx {
		proposed := codecQualityLadder[targetIdx]
		// Dedup: same proposal within the cooldown window is dropped. Steady-
		// state good/bad conditions would otherwise fire a proposal every tick
		// (500ms) even though the receiver already knows.
		b.mu.Lock()
		if b.lastProposedCodec == proposed && time.Since(b.lastProposedAt) < codecProposalCooldown {
			b.mu.Unlock()
			return
		}
		b.lastProposedCodec = proposed
		b.lastProposedAt = time.Now()
		b.mu.Unlock()

		b.logger.Printf("BWE session=%s codec-switch proposal: %s -> %s (jitter=%dms loss=%.1f%%)",
			b.sessionID, currentCodec, proposed, avgJitter, avgLoss)
		b.sendDataChannelEvent("codec-switch-proposal", map[string]interface{}{
			"from":        currentCodec,
			"to":          proposed,
			"jitterMs":    avgJitter,
			"lossPercent": avgLoss,
		})
	}
}

// sendDataChannelEvent marshals payload to JSON and sends it over the session's
// DataChannel. Silently drops if the channel is nil or not open.
func (b *BandwidthEstimator) sendDataChannelEvent(eventType string, payload interface{}) {
	b.session.Mutex.RLock()
	dc := b.session.DataChannel
	b.session.Mutex.RUnlock()

	if dc == nil {
		return
	}
	data, err := json.Marshal(map[string]interface{}{
		"type":    eventType,
		"payload": payload,
	})
	if err != nil {
		return
	}
	// SendText is safe to call from any goroutine; errors are non-fatal.
	_ = dc.SendText(string(data))
}

func (b *BandwidthEstimator) setState(s bweState) {
	b.mu.Lock()
	b.state = s
	b.mu.Unlock()
}

func (b *BandwidthEstimator) currentState() bweState {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

func (s bweState) String() string {
	switch s {
	case bweNormal:
		return "normal"
	case bweUnderuse:
		return "underuse"
	case bweOveruse:
		return "overuse"
	default:
		return "unknown"
	}
}

// ---- helpers ----

func clampBitrate(bps uint32) uint32 {
	if bps < bitrateMin {
		return bitrateMin
	}
	if bps > bitrateMax {
		return bitrateMax
	}
	return bps
}

func average64(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func averageU32(vals []uint32) uint32 {
	if len(vals) == 0 {
		return 0
	}
	var sum uint64
	for _, v := range vals {
		sum += uint64(v)
	}
	return uint32(sum / uint64(len(vals)))
}

func codecIndex(codec string) int {
	for i, c := range codecQualityLadder {
		if c == codec {
			return i
		}
	}
	return -1
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
