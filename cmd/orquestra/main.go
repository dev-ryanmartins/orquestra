package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/dev-ryanmartins/orquestra/internal/httpapi"
	"github.com/dev-ryanmartins/orquestra/internal/processor"
	"github.com/dev-ryanmartins/orquestra/internal/service"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	port := envInt("PORT", 8080)
	workers := envInt("ORQUESTRA_WORKERS", 4)
	capacity := envInt("ORQUESTRA_QUEUE_CAPACITY", 128)
	delay := envDuration("ORQUESTRA_PROCESSING_DELAY_MS", 100*time.Millisecond)

	processor := processor.New(delay, logger)
	taskService, err := service.New(capacity, workers, processor.Process)
	if err != nil {
		logger.Error("failed to create task service", "error", err)
		os.Exit(1)
	}
	taskService.Start(context.Background())

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           httpapi.New(taskService, logger, "/api"),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("Orquestra está online", "port", port, "workers", workers, "queue_capacity", capacity)
		serverErrors <- server.ListenAndServe()
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	case signal := <-shutdown:
		logger.Info("encerrando Orquestra", "signal", signal.String())

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
		}
		taskService.Close()
	}
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	milliseconds, err := strconv.Atoi(value)
	if err != nil || milliseconds < 0 {
		return fallback
	}
	return time.Duration(milliseconds) * time.Millisecond
}
