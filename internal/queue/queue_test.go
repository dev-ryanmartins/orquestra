package queue_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dev-ryanmartins/orquestra/internal/queue"
	"github.com/dev-ryanmartins/orquestra/internal/task"
)

func TestQueueProcessesJobsWithMultipleWorkers(t *testing.T) {
	const jobs = 12
	var processed atomic.Int32

	taskQueue, err := queue.New(jobs, 4, func(_ context.Context, _ *task.Task) error {
		time.Sleep(10 * time.Millisecond)
		processed.Add(1)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	taskQueue.Start(context.Background())

	for range jobs {
		value, err := task.New("teste", nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := taskQueue.Enqueue(value); err != nil {
			t.Fatal(err)
		}
	}

	taskQueue.Close()

	if got := processed.Load(); got != jobs {
		t.Fatalf("processed %d jobs, want %d", got, jobs)
	}
}

func TestQueueCloseRejectsNewJobs(t *testing.T) {
	taskQueue, err := queue.New(1, 1, func(context.Context, *task.Task) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	taskQueue.Start(context.Background())
	taskQueue.Close()

	value, err := task.New("depois-do-fechamento", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := taskQueue.Enqueue(value); err != queue.ErrClosed {
		t.Fatalf("enqueue after close returned %v, want %v", err, queue.ErrClosed)
	}
}

func TestQueueHandlerRunsConcurrently(t *testing.T) {
	var running atomic.Int32
	var maxRunning atomic.Int32
	var release sync.WaitGroup
	release.Add(4)

	taskQueue, err := queue.New(4, 4, func(_ context.Context, _ *task.Task) error {
		current := running.Add(1)
		for {
			previous := maxRunning.Load()
			if current <= previous || maxRunning.CompareAndSwap(previous, current) {
				break
			}
		}
		release.Done()
		time.Sleep(20 * time.Millisecond)
		running.Add(-1)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	taskQueue.Start(context.Background())

	for range 4 {
		value, err := task.New("concorrente", nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := taskQueue.Enqueue(value); err != nil {
			t.Fatal(err)
		}
	}
	release.Wait()
	taskQueue.Close()

	if got := maxRunning.Load(); got < 2 {
		t.Fatalf("maximum concurrent handlers was %d, want at least 2", got)
	}
}
