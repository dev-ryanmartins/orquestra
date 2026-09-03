package processor

import (
	"context"
	"log/slog"
	"time"

	"github.com/dev-ryanmartins/orquestra/internal/task"
)

type Processor struct {
	delay  time.Duration
	logger *slog.Logger
}

func New(delay time.Duration, logger *slog.Logger) Processor {
	return Processor{delay: delay, logger: logger}
}

func (p Processor) Process(ctx context.Context, value *task.Task) error {
	if p.delay > 0 {
		timer := time.NewTimer(p.delay)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}

	p.logger.Info("tarefa processada", "task_id", value.ID, "name", value.Name)
	return nil
}
