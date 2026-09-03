package store

import (
	"errors"
	"sync"
	"time"

	"github.com/dev-ryanmartins/orquestra/internal/task"
)

var ErrNotFound = errors.New("task not found")

type Snapshot struct {
	Total      int            `json:"total"`
	Pending    int            `json:"pending"`
	Processing int            `json:"processing"`
	Completed  int            `json:"completed"`
	Failed     int            `json:"failed"`
	ByStatus   map[string]int `json:"-"`
}

type Memory struct {
	mu    sync.RWMutex
	tasks map[string]*task.Task
}

func NewMemory() *Memory {
	return &Memory{tasks: make(map[string]*task.Task)}
}

func (s *Memory) Create(value *task.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[value.ID]; exists {
		return errors.New("task already exists")
	}
	s.tasks[value.ID] = value.Clone()
	return nil
}

func (s *Memory) Get(id string) (*task.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, exists := s.tasks[id]
	if !exists {
		return nil, ErrNotFound
	}
	return value.Clone(), nil
}

func (s *Memory) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tasks, id)
}

func (s *Memory) UpdateStatus(id string, status task.Status, processingError error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	value, exists := s.tasks[id]
	if !exists {
		return ErrNotFound
	}

	now := timeNow()
	value.Status = status
	value.Error = ""

	switch status {
	case task.StatusProcessing:
		value.StartedAt = &now
	case task.StatusCompleted, task.StatusFailed:
		value.CompletedAt = &now
		if processingError != nil {
			value.Error = processingError.Error()
		}
	}

	return nil
}

func (s *Memory) Stats() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := Snapshot{
		Total:    len(s.tasks),
		ByStatus: make(map[string]int),
	}
	for _, value := range s.tasks {
		snapshot.ByStatus[string(value.Status)]++
	}

	snapshot.Pending = snapshot.ByStatus[string(task.StatusPending)]
	snapshot.Processing = snapshot.ByStatus[string(task.StatusProcessing)]
	snapshot.Completed = snapshot.ByStatus[string(task.StatusCompleted)]
	snapshot.Failed = snapshot.ByStatus[string(task.StatusFailed)]
	return snapshot
}

// timeNow is a variable to keep timestamp creation easy to control in focused tests.
var timeNow = nowUTC

func nowUTC() time.Time {
	return time.Now().UTC()
}
