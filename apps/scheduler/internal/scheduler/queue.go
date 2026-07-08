package scheduler

import (
	"container/heap"
	"sync"
	"time"
)

type queuedItem struct {
	item    *QueueItem
	index   int
	enqueuedAt time.Time
}

type priorityQueue []*queuedItem

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	if pq[i].item.Priority != pq[j].item.Priority {
		return pq[i].item.Priority > pq[j].item.Priority
	}
	return pq[i].enqueuedAt.Before(pq[j].enqueuedAt)
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *priorityQueue) Push(x any) {
	n := len(*pq)
	qi := x.(*queuedItem)
	qi.index = n
	*pq = append(*pq, qi)
}

func (pq *priorityQueue) Pop() any {
	old := *pq
	n := len(old)
	qi := old[n-1]
	old[n-1] = nil
	qi.index = -1
	*pq = old[:n-1]
	return qi
}

type PriorityQueue struct {
	mu     sync.Mutex
	queue  priorityQueue
	items  map[string]*queuedItem
}

func NewPriorityQueue() *PriorityQueue {
	pq := &PriorityQueue{
		queue: make(priorityQueue, 0),
		items: make(map[string]*queuedItem),
	}
	heap.Init(&pq.queue)
	return pq
}

func (pq *PriorityQueue) Enqueue(item *QueueItem) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	qi := &queuedItem{
		item:       item,
		enqueuedAt: time.Now(),
	}
	heap.Push(&pq.queue, qi)
	pq.items[item.ID] = qi
}

func (pq *PriorityQueue) Dequeue() *QueueItem {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.queue.Len() == 0 {
		return nil
	}

	qi := heap.Pop(&pq.queue).(*queuedItem)
	delete(pq.items, qi.item.ID)
	return qi.item
}

func (pq *PriorityQueue) Peek() *QueueItem {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.queue.Len() == 0 {
		return nil
	}
	return pq.queue[0].item
}

func (pq *PriorityQueue) Remove(id string) bool {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	qi, ok := pq.items[id]
	if !ok {
		return false
	}
	heap.Remove(&pq.queue, qi.index)
	delete(pq.items, id)
	return true
}

func (pq *PriorityQueue) Len() int {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	return pq.queue.Len()
}

func (pq *PriorityQueue) List() []*QueueItem {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	result := make([]*QueueItem, 0, pq.queue.Len())
	for _, qi := range pq.queue {
		result = append(result, qi.item)
	}
	return result
}
