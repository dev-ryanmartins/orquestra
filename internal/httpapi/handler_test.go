package httpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dev-ryanmartins/orquestra/internal/httpapi"
	"github.com/dev-ryanmartins/orquestra/internal/service"
	"github.com/dev-ryanmartins/orquestra/internal/task"
)

func TestCreateAndGetTask(t *testing.T) {
	taskService, err := service.New(4, 2, func(context.Context, *task.Task) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	taskService.Start(context.Background())
	defer taskService.Close()

	server := httptest.NewServer(httpapi.New(taskService, slog.Default(), "/api"))
	defer server.Close()

	response, err := http.Post(
		server.URL+"/api/tasks",
		"application/json",
		strings.NewReader(`{"name":"enviar-relatorio","payload":{"cliente":"acme"}}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("POST status = %d, body = %s", response.StatusCode, body)
	}

	var created task.Task
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.Status != task.StatusPending {
		t.Fatalf("created status = %q, want %q", created.Status, task.StatusPending)
	}

	var current task.Task
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		getResponse, err := http.Get(server.URL + "/api/tasks/" + created.ID)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.NewDecoder(getResponse.Body).Decode(&current); err != nil {
			getResponse.Body.Close()
			t.Fatal(err)
		}
		getResponse.Body.Close()
		if current.Status == task.StatusCompleted {
			break
		}
		time.Sleep(time.Millisecond)
	}

	if current.Status != task.StatusCompleted {
		t.Fatalf("final status = %q, want %q", current.Status, task.StatusCompleted)
	}
}

func TestUnknownTaskReturnsNotFound(t *testing.T) {
	taskService, err := service.New(1, 1, func(context.Context, *task.Task) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	taskService.Start(context.Background())
	defer taskService.Close()

	request := httptest.NewRequest(http.MethodGet, "/api/tasks/does-not-exist", nil)
	recorder := httptest.NewRecorder()
	httpapi.New(taskService, slog.Default(), "/api").ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}
