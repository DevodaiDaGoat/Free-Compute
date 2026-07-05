package scheduler

import (
	"errors"
	"testing"
	"time"
)

func TestNewQueue(t *testing.T) {
	q := NewQueue(100, 30*time.Second)
	if q.Len() != 0 {
		t.Errorf("new queue length = %d, want 0", q.Len())
	}
	if q.maxSize != 100 {
		t.Errorf("maxSize = %d, want 100", q.maxSize)
	}
}

func TestQueue_JoinAndLen(t *testing.T) {
	q := NewQueue(10, time.Minute)

	if err := q.Join("user-1", validRequest()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q.Len() != 1 {
		t.Errorf("length = %d, want 1", q.Len())
	}

	if err := q.Join("user-2", validRequest()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q.Len() != 2 {
		t.Errorf("length = %d, want 2", q.Len())
	}
}

func TestQueue_JoinDuplicate(t *testing.T) {
	q := NewQueue(10, time.Minute)
	q.Join("user-1", validRequest())
	err := q.Join("user-1", validRequest())
	if !errors.Is(err, ErrAlreadyInQueue) {
		t.Errorf("expected ErrAlreadyInQueue, got %v", err)
	}
}

func TestQueue_JoinFull(t *testing.T) {
	q := NewQueue(2, time.Minute)
	q.Join("user-1", validRequest())
	q.Join("user-2", validRequest())
	err := q.Join("user-3", validRequest())
	if !errors.Is(err, ErrQueueFull) {
		t.Errorf("expected ErrQueueFull, got %v", err)
	}
}

func TestQueue_Leave(t *testing.T) {
	q := NewQueue(10, time.Minute)
	q.Join("user-1", validRequest())
	q.Join("user-2", validRequest())

	if err := q.Leave("user-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q.Len() != 1 {
		t.Errorf("length = %d, want 1", q.Len())
	}

	// user-2 should now be at position 1
	status, err := q.Status("user-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Position != 1 {
		t.Errorf("position = %d, want 1", status.Position)
	}
}

func TestQueue_LeaveNotInQueue(t *testing.T) {
	q := NewQueue(10, time.Minute)
	err := q.Leave("nobody")
	if !errors.Is(err, ErrNotInQueue) {
		t.Errorf("expected ErrNotInQueue, got %v", err)
	}
}

func TestQueue_Status(t *testing.T) {
	q := NewQueue(10, 30*time.Second)
	q.Join("user-1", validRequest())
	q.Join("user-2", validRequest())
	q.Join("user-3", validRequest())

	status, err := q.Status("user-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Position != 2 {
		t.Errorf("position = %d, want 2", status.Position)
	}
	if status.EstimatedWait != 60*time.Second {
		t.Errorf("estimated wait = %v, want 60s", status.EstimatedWait)
	}
}

func TestQueue_StatusNotInQueue(t *testing.T) {
	q := NewQueue(10, time.Minute)
	_, err := q.Status("nobody")
	if !errors.Is(err, ErrNotInQueue) {
		t.Errorf("expected ErrNotInQueue, got %v", err)
	}
}

func TestQueue_Peek(t *testing.T) {
	q := NewQueue(10, time.Minute)

	_, ok := q.Peek()
	if ok {
		t.Error("peek on empty queue should return false")
	}

	q.Join("user-1", validRequest())
	q.Join("user-2", validRequest())

	entry, ok := q.Peek()
	if !ok {
		t.Fatal("peek should return true")
	}
	if entry.UserID != "user-1" {
		t.Errorf("peek user = %s, want user-1", entry.UserID)
	}
	if q.Len() != 2 {
		t.Error("peek should not remove entries")
	}
}

func TestQueue_Dequeue(t *testing.T) {
	q := NewQueue(10, time.Minute)

	_, ok := q.Dequeue()
	if ok {
		t.Error("dequeue on empty queue should return false")
	}

	q.Join("user-1", validRequest())
	q.Join("user-2", validRequest())

	entry, ok := q.Dequeue()
	if !ok {
		t.Fatal("dequeue should return true")
	}
	if entry.UserID != "user-1" {
		t.Errorf("dequeued user = %s, want user-1", entry.UserID)
	}
	if q.Len() != 1 {
		t.Errorf("length after dequeue = %d, want 1", q.Len())
	}

	entry, ok = q.Dequeue()
	if !ok || entry.UserID != "user-2" {
		t.Error("second dequeue should return user-2")
	}
	if q.Len() != 0 {
		t.Error("queue should be empty after dequeuing all")
	}
}

func TestQueue_DequeueUpdatesIndex(t *testing.T) {
	q := NewQueue(10, time.Minute)
	q.Join("user-1", validRequest())
	q.Join("user-2", validRequest())
	q.Join("user-3", validRequest())

	q.Dequeue()

	status, err := q.Status("user-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Position != 1 {
		t.Errorf("position after dequeue = %d, want 1", status.Position)
	}
}

func TestQueue_FIFO(t *testing.T) {
	q := NewQueue(5, time.Minute)
	users := []string{"a", "b", "c", "d", "e"}
	for _, u := range users {
		q.Join(u, validRequest())
	}

	for _, want := range users {
		entry, ok := q.Dequeue()
		if !ok {
			t.Fatal("unexpected empty queue")
		}
		if entry.UserID != want {
			t.Errorf("dequeued %s, want %s", entry.UserID, want)
		}
	}
}

func TestQueue_RejoinAfterLeave(t *testing.T) {
	q := NewQueue(10, time.Minute)
	q.Join("user-1", validRequest())
	q.Leave("user-1")

	err := q.Join("user-1", validRequest())
	if err != nil {
		t.Errorf("should be able to rejoin after leaving: %v", err)
	}
}
