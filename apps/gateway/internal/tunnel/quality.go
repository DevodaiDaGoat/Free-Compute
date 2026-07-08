package tunnel

import (
	"math"
	"sync"
	"time"
)

const (
	ewmaAlphaDefault     = 0.3
	scoreExcellent       = 100.0
	scoreGood            = 80.0
	scoreFair            = 50.0
	scorePoor            = 20.0
	maxRTTMs             = 300.0
	maxJitterMs          = 100.0
	maxPacketLoss        = 0.2
	scoreRTTWeight       = 0.30
	scoreLossWeight      = 0.30
	scoreJitterWeight    = 0.20
	scoreBandwidthWeight = 0.20
)

type EWMA struct {
	alpha float64
	value float64
	init  bool
}

func NewEWMA(alpha float64) *EWMA {
	if alpha <= 0 || alpha > 1 {
		alpha = ewmaAlphaDefault
	}
	return &EWMA{alpha: alpha}
}

func (e *EWMA) Update(sample float64) float64 {
	if !e.init {
		e.value = sample
		e.init = true
	} else {
		e.value = e.alpha*sample + (1-e.alpha)*e.value
	}
	return e.value
}

func (e *EWMA) Value() float64 {
	return e.value
}

type ConnQuality struct {
	mu                sync.RWMutex
	RouteID           string
	SmoothedRTTMs     float64
	SmoothedLossRatio float64
	SmoothedJitterMs  float64
	BandwidthBps      uint64
	Score             float64
	LastUpdated       time.Time

	rttEWMA    *EWMA
	lossEWMA   *EWMA
	jitterEWMA *EWMA
	lastRTT    float64
}

type QualityThresholds struct {
	GoodRTTMs        float64
	FairRTTMs        float64
	GoodLossRatio    float64
	FairLossRatio    float64
	GoodJitterMs     float64
	FairJitterMs     float64
	MinBandwidthBps  uint64
	ExcellentScore   float64
	GoodScore        float64
	FairScore        float64
}

func DefaultQualityThresholds() QualityThresholds {
	return QualityThresholds{
		GoodRTTMs:       30.0,
		FairRTTMs:       100.0,
		GoodLossRatio:   0.01,
		FairLossRatio:   0.05,
		GoodJitterMs:    15.0,
		FairJitterMs:    40.0,
		MinBandwidthBps: 100_000,
		ExcellentScore:  90.0,
		GoodScore:       70.0,
		FairScore:       40.0,
	}
}

func NewConnQuality(routeID string) *ConnQuality {
	return &ConnQuality{
		RouteID:    routeID,
		rttEWMA:    NewEWMA(ewmaAlphaDefault),
		lossEWMA:   NewEWMA(ewmaAlphaDefault),
		jitterEWMA: NewEWMA(ewmaAlphaDefault),
	}
}

func (q *ConnQuality) Update(rtt time.Duration, lossRatio float64, bandwidthBps uint64) {
	q.mu.Lock()
	defer q.mu.Unlock()

	rttMs := float64(rtt.Milliseconds())

	jitter := math.Abs(rttMs - q.lastRTT)
	q.lastRTT = rttMs

	q.SmoothedRTTMs = q.rttEWMA.Update(rttMs)
	q.SmoothedLossRatio = q.lossEWMA.Update(lossRatio)
	q.SmoothedJitterMs = q.jitterEWMA.Update(jitter)
	q.BandwidthBps = bandwidthBps
	q.LastUpdated = time.Now()
	q.Score = q.computeQoE()
}

func (q *ConnQuality) UpdateWithJitter(rtt time.Duration, jitter time.Duration, lossRatio float64, bandwidthBps uint64) {
	q.mu.Lock()
	defer q.mu.Unlock()

	rttMs := float64(rtt.Milliseconds())
	jitterMs := float64(jitter.Milliseconds())

	q.SmoothedRTTMs = q.rttEWMA.Update(rttMs)
	q.SmoothedLossRatio = q.lossEWMA.Update(lossRatio)
	q.SmoothedJitterMs = q.jitterEWMA.Update(jitterMs)
	q.BandwidthBps = bandwidthBps
	q.LastUpdated = time.Now()
	q.Score = q.computeQoE()
}

func (q *ConnQuality) computeQoE() float64 {
	rttScore := 1.0 - math.Min(q.SmoothedRTTMs/maxRTTMs, 1.0)
	lossScore := 1.0 - math.Min(q.SmoothedLossRatio/maxPacketLoss, 1.0)
	jitterScore := 1.0 - math.Min(q.SmoothedJitterMs/maxJitterMs, 1.0)
	bwScore := math.Min(float64(q.BandwidthBps)/50_000_000.0, 1.0)

	raw := (rttScore*scoreRTTWeight +
		lossScore*scoreLossWeight +
		jitterScore*scoreJitterWeight +
		bwScore*scoreBandwidthWeight)

	mos := 1.0 + 4.0*raw
	score := (mos - 1.0) / 4.0 * 100.0
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score
}

func (q *ConnQuality) GetScore() float64 {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.Score
}

func (q *ConnQuality) Snapshot() ConnQualitySnapshot {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return ConnQualitySnapshot{
		RouteID:           q.RouteID,
		SmoothedRTTMs:     q.SmoothedRTTMs,
		SmoothedLossRatio: q.SmoothedLossRatio,
		SmoothedJitterMs:  q.SmoothedJitterMs,
		BandwidthBps:      q.BandwidthBps,
		Score:             q.Score,
		LastUpdated:       q.LastUpdated,
	}
}

type ConnQualitySnapshot struct {
	RouteID           string    `json:"routeId"`
	SmoothedRTTMs     float64   `json:"smoothedRttMs"`
	SmoothedLossRatio float64   `json:"smoothedLossRatio"`
	SmoothedJitterMs  float64   `json:"smoothedJitterMs"`
	BandwidthBps      uint64    `json:"bandwidthBps"`
	Score             float64   `json:"score"`
	LastUpdated       time.Time `json:"lastUpdated"`
}

type QualityCategory string

const (
	QualityExcellent QualityCategory = "excellent"
	QualityGood      QualityCategory = "good"
	QualityFair      QualityCategory = "fair"
	QualityPoor      QualityCategory = "poor"
)

func (q *ConnQuality) Category(thresholds QualityThresholds) QualityCategory {
	score := q.GetScore()
	switch {
	case score >= thresholds.ExcellentScore:
		return QualityExcellent
	case score >= thresholds.GoodScore:
		return QualityGood
	case score >= thresholds.FairScore:
		return QualityFair
	default:
		return QualityPoor
	}
}

type QualityTracker struct {
	mu         sync.RWMutex
	qualities  map[string]*ConnQuality
	thresholds QualityThresholds
}

func NewQualityTracker(thresholds QualityThresholds) *QualityTracker {
	return &QualityTracker{
		qualities:  make(map[string]*ConnQuality),
		thresholds: thresholds,
	}
}

func (t *QualityTracker) GetOrCreate(routeID string) *ConnQuality {
	t.mu.Lock()
	defer t.mu.Unlock()

	if q, ok := t.qualities[routeID]; ok {
		return q
	}
	q := NewConnQuality(routeID)
	t.qualities[routeID] = q
	return q
}

func (t *QualityTracker) Get(routeID string) *ConnQuality {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.qualities[routeID]
}

func (t *QualityTracker) Remove(routeID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.qualities, routeID)
}

func (t *QualityTracker) All() []ConnQualitySnapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()

	snaps := make([]ConnQualitySnapshot, 0, len(t.qualities))
	for _, q := range t.qualities {
		snaps = append(snaps, q.Snapshot())
	}
	return snaps
}

func (t *QualityTracker) SetThresholds(thresholds QualityThresholds) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.thresholds = thresholds
}
