package task

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"
)

type Status string

const (
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
)

type Task struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Payload     json.RawMessage `json:"payload"`
	Status      Status          `json:"status"`
	Error       string          `json:"error,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	StartedAt   *time.Time      `json:"started_at,omitempty"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
}

func New(name string, payload json.RawMessage) (*Task, error) {
	id, err := NewID()
	if err != nil {
		return nil, err
	}

	return &Task{
		ID:        id,
		Name:      name,
		Payload:   cloneBytes(payload),
		Status:    StatusPending,
		CreatedAt: time.Now().UTC(),
	}, nil
}

func NewID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func (t *Task) Clone() *Task {
	if t == nil {
		return nil
	}

	clone := *t
	clone.Payload = cloneBytes(t.Payload)
	return &clone
}

func cloneBytes(value []byte) []byte {
	if value == nil {
		return nil
	}
	return append([]byte(nil), value...)
}
