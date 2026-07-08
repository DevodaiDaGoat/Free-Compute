package webrtc

import (
	"math"
	"sync"
	"time"
)

type QualitySample struct {
	RTT           time.Duration
	Jitter        time.Duration
	PacketLoss    float64
	ThroughputBps uint64
	SampledAt     time.Time
}

type Trend struct {
	LossTrend    float64
	RTTTrend     float64
	ThroughTrend float64
}

type BandwidthEstimator struct {
	mu         sync.Mutex
	history    []QualitySample
	maxSamples int
	minBitrate int
	maxBitrate int
	currentBps int
}

type BitrateConfig struct {
	TargetBps    int
	MinBps       int
	MaxBps       int
	ResolutionW  int
	ResolutionH  int
	MaxFPS       int
	JitterBuffer time.Duration
}

func NewBandwidthEstimator(minBitrate, maxBitrate, startBitrate int) *BandwidthEstimator {
	return &BandwidthEstimator{
		history:    make([]QualitySample, 0, 60),
		maxSamples: 60,
		minBitrate: minBitrate,
		maxBitrate: maxBitrate,
		currentBps: startBitrate,
	}
}

func (e *BandwidthEstimator) OnSample(s QualitySample) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.history = append(e.history, s)
	if len(e.history) > e.maxSamples {
		e.history = e.history[1:]
	}

	trend := e.predictTrend()
	e.currentBps = e.computeTarget(trend, s)
}

func (e *BandwidthEstimator) CurrentBitrate() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.currentBps
}

func (e *BandwidthEstimator) predictTrend() Trend {
	n := len(e.history)
	if n < 2 {
		return Trend{}
	}

	lookback := 10
	if n < lookback {
		lookback = n
	}
	recent := e.history[n-lookback:]

	var sumX, sumYloss, sumYrtt, sumYthru float64
	for i, s := range recent {
		x := float64(i)
		sumX += x
		sumYloss += s.PacketLoss
		sumYrtt += float64(s.RTT.Milliseconds())
		sumYthru += float64(s.ThroughputBps)
	}

	meanX := sumX / float64(lookback)
	meanYloss := sumYloss / float64(lookback)
	meanYrtt := sumYrtt / float64(lookback)
	meanYthru := sumYthru / float64(lookback)

	var numLoss, numRTT, numThru, denom float64
	for i, s := range recent {
		x := float64(i) - meanX
		denom += x * x
		numLoss += x * (s.PacketLoss - meanYloss)
		numRTT += x * (float64(s.RTT.Milliseconds()) - meanYrtt)
		numThru += x * (float64(s.ThroughputBps) - meanYthru)
	}

	if denom == 0 {
		return Trend{}
	}

	return Trend{
		LossTrend:    numLoss / denom,
		RTTTrend:     numRTT / denom,
		ThroughTrend: numThru / denom,
	}
}

func (e *BandwidthEstimator) computeTarget(trend Trend, latest QualitySample) int {
	target := e.currentBps

	lossRatio := latest.PacketLoss
	if lossRatio > 0.05 {
		target = int(float64(target) * 0.7)
	} else if lossRatio > 0.03 {
		target = int(float64(target) * 0.85)
	} else if lossRatio > 0.01 {
	} else if latest.RTT < 30*time.Millisecond {
		target = int(float64(target) * 1.1)
	}

	if latest.RTT > 150*time.Millisecond {
		target = int(float64(target) * 0.8)
	}

	if target < e.minBitrate {
		target = e.minBitrate
	}
	if target > e.maxBitrate {
		target = e.maxBitrate
	}

	return target
}

type AdaptiveBitrate struct {
	estimator *BandwidthEstimator
	config    BitrateConfig
	mu        sync.Mutex
}

func NewAdaptiveBitrate(minBps, maxBps, startBps int) *AdaptiveBitrate {
	return &AdaptiveBitrate{
		estimator: NewBandwidthEstimator(minBps, maxBps, startBps),
		config: BitrateConfig{
			TargetBps:   startBps,
			MinBps:      minBps,
			MaxBps:      maxBps,
			ResolutionW: 1920,
			ResolutionH: 1080,
			MaxFPS:      60,
		},
	}
}

func (a *AdaptiveBitrate) OnQualitySample(rtt time.Duration, jitter time.Duration, loss float64, throughput uint64) {
	a.estimator.OnSample(QualitySample{
		RTT:           rtt,
		Jitter:        jitter,
		PacketLoss:    loss,
		ThroughputBps: throughput,
		SampledAt:     time.Now(),
	})

	a.mu.Lock()
	defer a.mu.Unlock()

	targetBps := a.estimator.CurrentBitrate()
	a.config.TargetBps = targetBps

	if targetBps < 2_000_000 {
		a.config.ResolutionW = 854
		a.config.ResolutionH = 480
		a.config.MaxFPS = 30
	} else if targetBps < 5_000_000 {
		a.config.ResolutionW = 1280
		a.config.ResolutionH = 720
		a.config.MaxFPS = 30
	} else if targetBps < 15_000_000 {
		a.config.ResolutionW = 1920
		a.config.ResolutionH = 1080
		a.config.MaxFPS = 60
	} else {
		a.config.ResolutionW = 2560
		a.config.ResolutionH = 1440
		a.config.MaxFPS = 60
	}

	if jitter > 40*time.Millisecond {
		a.config.JitterBuffer = 60 * time.Millisecond
	} else if jitter > 20*time.Millisecond {
		a.config.JitterBuffer = 40 * time.Millisecond
	} else {
		a.config.JitterBuffer = 20 * time.Millisecond
	}
}

func (a *AdaptiveBitrate) GetConfig() BitrateConfig {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.config
}

func computeQualityScore(rtt time.Duration, jitter time.Duration, loss float64, targetBps int, actualBps uint64) float64 {
	rttScore := 1.0 - math.Min(float64(rtt.Milliseconds())/300.0, 1.0)*0.3
	jitterScore := 1.0 - math.Min(float64(jitter.Milliseconds())/50.0, 1.0)*0.2
	lossScore := 1.0 - math.Min(loss/0.1, 1.0)*0.25
	throughputRatio := float64(actualBps) / float64(max(targetBps, 1))
	throughputScore := math.Min(throughputRatio, 1.0) * 0.25

	return rttScore + jitterScore + lossScore + throughputScore
}
