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
	running    bool
	stopChan   chan struct{}
}

type QueuedSession struct {
	SessionID string
	Request   *CreateSessionRequest
	QueuedAt  time.Time
	Priority  int
	index     int
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
	s.running = true
	go s.run(config)
}

func (s *SessionScheduler) Stop() {
	s.running = false
	close(s.stopChan)
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
		s.logger.Printf("failed to allocate host for session %s: %v", queued.SessionID, err)
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
