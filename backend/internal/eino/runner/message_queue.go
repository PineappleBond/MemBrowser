package runner

import "sync"

type MessageQueue struct {
	mu     sync.Mutex
	queues map[string][]string
}

func NewMessageQueue() *MessageQueue {
	return &MessageQueue{queues: make(map[string][]string)}
}

func (q *MessageQueue) Enqueue(taskID string, content string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.queues[taskID] = append(q.queues[taskID], content)
}

func (q *MessageQueue) Drain(taskID string) []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	msgs := q.queues[taskID]
	q.queues[taskID] = nil
	return msgs
}

func (q *MessageQueue) HasPending(taskID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.queues[taskID]) > 0
}

func (q *MessageQueue) Cleanup(taskID string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.queues, taskID)
}
