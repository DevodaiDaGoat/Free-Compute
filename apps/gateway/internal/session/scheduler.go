package session

import (
	"container/heap"
	"context"
	"log"
	"sync"
	"time"
)

type SessionScheduler struct {
	logger     *log.Logger
	queue      priorityHeap
	queueMutex sync.RWMutex
	allocator  *HostAllocator
	stopChan   chan struct{}
	stopOnce   sync.Once
}

type QueuedSession struct {
	SessionID string
	Request   *CreateSessionRequest
	QueuedAt  time.Time
	Priority  int
	// FailedAttempts tracks consecutive AllocateHost failures. Previously the
	// scheduler only reduced priority (clamped at 0) on failure, so a job that
	// could never be scheduled (no host with matching class/region) burned
	// every tick popping and repushing itself and starved lower-priority jobs.
	FailedAttempts int
	index          int
}

type priorityHeap []*QueuedSession

func (h priorityHeap) Len() int            { return len(h) }
func (h priorityHeap) Less(i, j int) bool  { return h[i].Priority > h[j].Priority }
func (h priorityHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}
func (h *priorityHeap) Push(x any) {
	n := len(*h)
	item := x.(*QueuedSession)
	item.index = n
	*h = append(*h, item)
}
func (h *priorityHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*h = old[:n-1]
	return item
}

type SchedulerConfig struct {
	MaxConcurrent   int
	CheckInterval   time.Duration
	GamingPriority  int
	DesktopPriority int
	SupportPriority int
}

func NewSessionScheduler(logger *log.Logger, allocator *HostAllocator) *SessionScheduler {
	if logger == nil {
		logger = log.Default()
	}

	return &SessionScheduler{
		logger:    logger,
		queue:     make(priorityHeap, 0),
		allocator: allocator,
		stopChan:  make(chan struct{}),
	}
}

func (s *SessionScheduler) Start(config SchedulerConfig) {
	go s.run(config)
}

// Stop signals the scheduler goroutine to exit. Safe to call multiple times.
func (s *SessionScheduler) Stop() {
	s.stopOnce.Do(func() { close(s.stopChan) })
}

func (s *SessionScheduler) Enqueue(sessionID string, request *CreateSessionRequest) {
	s.queueMutex.Lock()
	defer s.queueMutex.Unlock()

	priority := s.calculatePriority(request)

	queued := &QueuedSession{
		SessionID: sessionID,
		Request:   request,
		QueuedAt:  time.Now(),
		Priority:  priority,
	}

	heap.Push(&s.queue, queued)
	s.logger.Printf("enqueued session %s with priority %d", sessionID, priority)
}

func (s *SessionScheduler) Dequeue(sessionID string) {
	s.queueMutex.Lock()
	defer s.queueMutex.Unlock()

	for i, queued := range s.queue {
		if queued.SessionID == sessionID {
			heap.Remove(&s.queue, i)
			s.logger.Printf("dequeued session %s", sessionID)
			return
		}
	}
}

func (s *SessionScheduler) GetQueueLength() int {
	s.queueMutex.RLock()
	defer s.queueMutex.RUnlock()
	return s.queue.Len()
}

func (s *SessionScheduler) run(config SchedulerConfig) {
	ticker := time.NewTicker(config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.processQueue(config)
		}
	}
}

func (s *SessionScheduler) processQueue(config SchedulerConfig) {
	s.queueMutex.Lock()
	if s.queue.Len() == 0 {
		s.queueMutex.Unlock()
		return
	}

	queued := heap.Pop(&s.queue).(*QueuedSession)
	s.queueMutex.Unlock()

	host, err := s.allocator.AllocateHost(
		context.Background(),
		queued.Request.Type,
		queued.Request.ResourceClass,
		queued.Request.Region,
		queued.Request.GPURequired,
	)

	if err != nil {
		queued.FailedAttempts++
		// Drop after 10 consecutive failures — otherwise a job that no host
		// can satisfy (e.g. a resource class we don't support yet) spins the
		// scheduler forever at priority 0 and blocks every other job.
		const maxFailures = 10
		if queued.FailedAttempts >= maxFailures {
			s.logger.Printf("dropping session %s after %d failed allocation attempts: %v", queued.SessionID, queued.FailedAttempts, err)
			return
		}
		s.logger.Printf("failed to allocate host for session %s (attempt %d/%d): %v", queued.SessionID, queued.FailedAttempts, maxFailures, err)
		queued.Priority = max(queued.Priority-1, 0)
		s.queueMutex.Lock()
		heap.Push(&s.queue, queued)
		s.queueMutex.Unlock()
		return
	}

	s.logger.Printf("scheduled session %s on host %s", queued.SessionID, host.ID)
}

func (s *SessionScheduler) calculatePriority(request *CreateSessionRequest) int {
	switch request.Type {
	case SessionTypeGaming:
		return 100
	case SessionTypeRemoteSupport:
		return 75
	case SessionTypeDesktop:
		return 50
	case SessionTypeHost:
		return 25
	default:
		return 10
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
