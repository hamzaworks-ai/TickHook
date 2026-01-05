// Package main is the entrypoint for TickHook.
// TickHook is a lightweight, self-hosted webhook scheduler.
// PRD Reference: Section 1 - Overview
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cr0hn/tickhook/internal/config"
	"github.com/cr0hn/tickhook/internal/executor"
	"github.com/cr0hn/tickhook/internal/httpapi"
	"github.com/cr0hn/tickhook/internal/scheduler"
	"github.com/cr0hn/tickhook/internal/store"
)

func main() {
	flag.Usage = config.Usage

	cfg, err := config.Parse()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		config.Usage()
		os.Exit(1)
	}

	// Setup logger
	logLevel := parseLogLevel(cfg.LogLevel)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	cfg.PrintStartup(logger)

	// Connect to Redis
	redisStore, err := store.NewRedisStore(cfg.RedisURL, cfg.Namespace)
	if err != nil {
		logger.Error("Failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	defer redisStore.Close()

	// Verify Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := redisStore.Ping(ctx); err != nil {
		cancel()
		logger.Error("Failed to ping Redis", "error", err)
		os.Exit(1)
	}
	cancel()
	logger.Info("Connected to Redis")

	// Create executor
	exec := executor.NewExecutor(cfg, redisStore, logger)

	// Create scheduler with executor as dispatcher
	sched := scheduler.NewScheduler(cfg, redisStore, logger, exec.Dispatch)

	// Create HTTP API server
	server := httpapi.NewServer(cfg, redisStore, logger)

	// Setup shutdown handling
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start components
	exec.Start(ctx)

	go sched.Start(ctx)

	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
			stop()
		}
	}()

	logger.Info("TickHook is running", "bind", cfg.Bind)

	// Wait for shutdown signal
	<-ctx.Done()
	logger.Info("Shutdown signal received")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Stop scheduler first (stops dispatching new jobs)
	sched.Stop()
	sched.Wait()
	logger.Info("Scheduler stopped")

	// Stop executor (waits for in-flight jobs to complete)
	exec.Stop()
	exec.Wait()
	logger.Info("Executor stopped")

	// Stop HTTP server
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error", "error", err)
	} else {
		logger.Info("HTTP server stopped")
	}

	logger.Info("TickHook shutdown complete")
}

func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
