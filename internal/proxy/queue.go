package proxy

import (
	"errors"
	"sync"
	"time"
)

// QueueTimeout is the maximum time a request will wait for a model to start
const QueueTimeout = 300 * time.Second

// ErrQueueTimeout is returned when a queued request times out waiting for the model
var ErrQueueTimeout = errors.New("timeout waiting for model to start")

// ErrModelStartFailed is returned when the model fails to start
var ErrModelStartFailed = errors.New("model failed to start")

// QueuedRequest represents a request waiting for a model to become ready
type QueuedRequest struct {
	Done      chan error
	CreatedAt time.Time
}

// RequestQueue manages queued requests per model
type RequestQueue struct {
	queues map[string][]*QueuedRequest
	mu     sync.Mutex
}

// NewRequestQueue creates a new RequestQueue
func NewRequestQueue() *RequestQueue {
	return &RequestQueue{
		queues: make(map[string][]*QueuedRequest),
	}
}

// Enqueue adds a request to the queue for a model
func (q *RequestQueue) Enqueue(modelName string, req *QueuedRequest) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.queues[modelName] = append(q.queues[modelName], req)
}

// GetAndClear returns all queued requests for a model and clears the queue
func (q *RequestQueue) GetAndClear(modelName string) []*QueuedRequest {
	q.mu.Lock()
	defer q.mu.Unlock()
	reqs := q.queues[modelName]
	delete(q.queues, modelName)
	return reqs
}

// NotifyAll signals all queued requests for a model that it's ready
func (q *RequestQueue) NotifyAll(modelName string) {
	reqs := q.GetAndClear(modelName)
	for _, req := range reqs {
		select {
		case req.Done <- nil:
		default:
			// Request already timed out or was cancelled
		}
	}
}

// FailAll signals all queued requests for a model that it failed to start
func (q *RequestQueue) FailAll(modelName string, err error) {
	reqs := q.GetAndClear(modelName)
	for _, req := range reqs {
		select {
		case req.Done <- err:
		default:
			// Request already timed out or was cancelled
		}
	}
}

// QueueCount returns the number of queued requests for a model
func (q *RequestQueue) QueueCount(modelName string) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.queues[modelName])
}
