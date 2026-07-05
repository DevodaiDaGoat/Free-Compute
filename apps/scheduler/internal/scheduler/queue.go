package scheduler

// Queue manages the scheduling queue for VM requests.
// When no hosts are immediately available, requests enter the queue.
// The Galaxy processor continuously dequeues and attempts placement.

// TODO: Implement queue operations:
// - AddToQueue(userID, request) — with position assignment
// - RemoveFromQueue(userID) — cancel waiting
// - GetPosition(userID) — current position and estimated wait
// - ProcessNext() — dequeue and schedule (called by background worker)
