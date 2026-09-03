package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/dev-ryanmartins/orquestra/internal/queue"
	"github.com/dev-ryanmartins/orquestra/internal/store"
	"github.com/dev-ryanmartins/orquestra/internal/task"
)

var ErrQueueFull = queue.ErrFull

type Processor func(context.Context, *task.Task) error

type Service struct {
	store *store.Memory
	queue *queue.Queue
}

func New(capacity, workers int, processor Processor) (*Service, error) {
	if processor == nil {
		return nil, errors.New("processor is required")
	}

	memory := store.NewMemory()
	taskQueue, err := queue.New(capacity, workers, func(ctx context.Context, value *task.Task) error {
		if err := memory.UpdateStatus(value.ID, task.StatusProcessing, nil); err != nil {
			return err
		}

		processingError := processor(ctx, value)
		if processingError != nil {
			_ = memory.UpdateStatus(value.ID, task.StatusFailed, processingError)
			return processingError
		}

		return memory.UpdateStatus(value.ID, task.StatusCompleted, nil)
	})
	if err != nil {
		return nil, err
	}

	return &Service{store: memory, queue: taskQueue}, nil
}

func (s *Service) Start(ctx context.Context) {
	s.queue.Start(ctx)
}

func (s *Service) Close() {
	s.queue.Close()
}

func (s *Service) Submit(name string, payload json.RawMessage) (*task.Task, error) {
	value, err := task.New(name, payload)
	if err != nil {
		return nil, err
	}
	if err := s.store.Create(value); err != nil {
		return nil, err
	}

	if err := s.queue.Enqueue(value); err != nil {
		s.store.Delete(value.ID)
		return nil, err
	}

	return value.Clone(), nil
}

func (s *Service) Get(id string) (*task.Task, error) {
	return s.store.Get(id)
}

func (s *Service) Stats() store.Snapshot {
	return s.store.Stats()
}

func (s *Service) QueueDepth() int {
	return s.queue.Len()
}

func (s *Service) QueueCapacity() int {
	return s.queue.Capacity()
}
