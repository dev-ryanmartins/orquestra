package queue

import (
	"context"
	"errors"
	"sync"

	"github.com/dev-ryanmartins/orquestra/internal/task"
)

var (
	ErrFull   = errors.New("queue is full")
	ErrClosed = errors.New("queue is closed")
)

type Handler func(context.Context, *task.Task) error

type Queue struct {
	jobs    chan *task.Task
	handler Handler
	workers int

	mu     sync.RWMutex
	closed bool
	wg     sync.WaitGroup
}

func New(capacity, workers int, handler Handler) (*Queue, error) {
	if capacity < 1 {
		return nil, errors.New("queue capacity must be greater than zero")
	}
	if workers < 1 {
		return nil, errors.New("worker count must be greater than zero")
	}
	if handler == nil {
		return nil, errors.New("queue handler is required")
	}

	return &Queue{
		jobs:    make(chan *task.Task, capacity),
		handler: handler,
		workers: workers,
	}, nil
}

func (q *Queue) Start(ctx context.Context) {
	q.wg.Add(q.workers)
	for i := 0; i < q.workers; i++ {
		go q.worker(ctx)
	}
}

func (q *Queue) Enqueue(value *task.Task) error {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if q.closed {
		return ErrClosed
	}

	select {
	case q.jobs <- value:
		return nil
	default:
		return ErrFull
	}
}

func (q *Queue) Len() int {
	return len(q.jobs)
}

func (q *Queue) Capacity() int {
	return cap(q.jobs)
}

func (q *Queue) Close() {
	q.mu.Lock()
	if !q.closed {
		q.closed = true
		close(q.jobs)
	}
	q.mu.Unlock()
	q.wg.Wait()
}

func (q *Queue) worker(ctx context.Context) {
	defer q.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case value, ok := <-q.jobs:
			if !ok {
				return
			}
			_ = q.handler(ctx, value)
		}
	}
}
