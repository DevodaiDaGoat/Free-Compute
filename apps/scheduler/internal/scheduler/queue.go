package scheduler

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrQueueFull       = errors.New("queue is full")
	ErrAlreadyInQueue  = errors.New("user already in queue")
	ErrNotInQueue      = errors.New("user not in queue")
)

// QueueEntry represents a user waiting in the allocation queue.
type QueueEntry struct {
	UserID   string
	JoinedAt time.Time
	Request  VMRequest
}

// QueueStatus is the public view of a user's position.
type QueueStatus struct {
	Position      int
	EstimatedWait time.Duration
	JoinedAt      time.Time
}

// Queue manages a FIFO queue of VM allocation requests.
type Queue struct {
	mu       sync.RWMutex
	entries  []QueueEntry
	index    map[string]int // userID -> position in entries
	maxSize  int
	avgWait  time.Duration // average processing time per entry
}

// NewQueue creates a new allocation queue.
func NewQueue(maxSize int, avgWait time.Duration) *Queue {
	return &Queue{
		entries: make([]QueueEntry, 0),
		index:   make(map[string]int),
		maxSize: maxSize,
		avgWait: avgWait,
	}
}

// Join adds a user to the queue.
func (q *Queue) Join(userID string, req VMRequest) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if _, exists := q.index[userID]; exists {
		return ErrAlreadyInQueue
	}
	if len(q.entries) >= q.maxSize {
		return ErrQueueFull
	}

	entry := QueueEntry{
		UserID:   userID,
		JoinedAt: time.Now(),
		Request:  req,
	}
	q.entries = append(q.entries, entry)
	q.index[userID] = len(q.entries) - 1
	return nil
}

// Leave removes a user from the queue.
func (q *Queue) Leave(userID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	pos, exists := q.index[userID]
	if !exists {
		return ErrNotInQueue
	}

	q.entries = append(q.entries[:pos], q.entries[pos+1:]...)
	delete(q.index, userID)
	q.rebuildIndex()
	return nil
}

// Status returns the queue position and estimated wait for a user.
func (q *Queue) Status(userID string) (QueueStatus, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	pos, exists := q.index[userID]
	if !exists {
		return QueueStatus{}, ErrNotInQueue
	}

	return QueueStatus{
		Position:      pos + 1, // 1-indexed
		EstimatedWait: time.Duration(pos+1) * q.avgWait,
		JoinedAt:      q.entries[pos].JoinedAt,
	}, nil
}

// Len returns the current queue length.
func (q *Queue) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.entries)
}

// Peek returns the next entry without removing it.
func (q *Queue) Peek() (QueueEntry, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if len(q.entries) == 0 {
		return QueueEntry{}, false
	}
	return q.entries[0], true
}

// Dequeue removes and returns the next entry.
func (q *Queue) Dequeue() (QueueEntry, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.entries) == 0 {
		return QueueEntry{}, false
	}

	entry := q.entries[0]
	q.entries = q.entries[1:]
	delete(q.index, entry.UserID)
	q.rebuildIndex()
	return entry, true
}

// rebuildIndex re-maps user IDs to their current positions. Must hold mu.
func (q *Queue) rebuildIndex() {
	for i, e := range q.entries {
		q.index[e.UserID] = i
	}
}
