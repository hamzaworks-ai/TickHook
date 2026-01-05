package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cr0hn/tickhook/internal/model"
)

func getTestRedisURL() string {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		url = "redis://localhost:6379"
	}
	return url
}

func BenchmarkCreateJob(b *testing.B) {
	store, err := NewRedisStore(getTestRedisURL(), "bench")
	if err != nil {
		b.Skip("Redis not available:", err)
	}
	defer store.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		job := &model.Job{
			ID:            uuid.New().String(),
			Type:          model.JobTypeOneShot,
			DueAtMs:       time.Now().Add(time.Hour).UnixMilli(),
			Attempt:       0,
			MaxAttempts:   3,
			BackoffBaseMs: 1000,
			URL:           "https://example.com/webhook",
			Method:        "POST",
			Headers:       map[string]string{"X-Test": "value"},
			Body:          map[string]any{"key": "value"},
			TimeoutMs:     5000,
			CreatedAtMs:   time.Now().UnixMilli(),
			UpdatedAtMs:   time.Now().UnixMilli(),
			Status:        model.JobStatusPending,
		}
		_ = store.CreateJob(ctx, job)
	}
}

func BenchmarkGetJob(b *testing.B) {
	store, err := NewRedisStore(getTestRedisURL(), "bench")
	if err != nil {
		b.Skip("Redis not available:", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create a job first
	job := &model.Job{
		ID:            "bench-get-job",
		Type:          model.JobTypeOneShot,
		DueAtMs:       time.Now().Add(time.Hour).UnixMilli(),
		Attempt:       0,
		MaxAttempts:   3,
		BackoffBaseMs: 1000,
		URL:           "https://example.com/webhook",
		Method:        "POST",
		TimeoutMs:     5000,
		CreatedAtMs:   time.Now().UnixMilli(),
		UpdatedAtMs:   time.Now().UnixMilli(),
		Status:        model.JobStatusPending,
	}
	_ = store.CreateJob(ctx, job)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.GetJob(ctx, "bench-get-job")
	}
}

func BenchmarkGetDueJobs(b *testing.B) {
	store, err := NewRedisStore(getTestRedisURL(), "bench")
	if err != nil {
		b.Skip("Redis not available:", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create many due jobs
	for i := 0; i < 1000; i++ {
		job := &model.Job{
			ID:            fmt.Sprintf("bench-due-%d", i),
			Type:          model.JobTypeOneShot,
			DueAtMs:       time.Now().Add(-time.Hour).UnixMilli(), // Already due
			Attempt:       0,
			MaxAttempts:   3,
			BackoffBaseMs: 1000,
			URL:           "https://example.com/webhook",
			Method:        "POST",
			TimeoutMs:     5000,
			CreatedAtMs:   time.Now().UnixMilli(),
			UpdatedAtMs:   time.Now().UnixMilli(),
			Status:        model.JobStatusPending,
		}
		_ = store.CreateJob(ctx, job)
	}

	nowMs := time.Now().UnixMilli()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.GetDueJobs(ctx, nowMs, 200)
	}
}

func BenchmarkUpdateJob(b *testing.B) {
	store, err := NewRedisStore(getTestRedisURL(), "bench")
	if err != nil {
		b.Skip("Redis not available:", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create a job first
	job := &model.Job{
		ID:            "bench-update-job",
		Type:          model.JobTypeOneShot,
		DueAtMs:       time.Now().Add(time.Hour).UnixMilli(),
		Attempt:       0,
		MaxAttempts:   3,
		BackoffBaseMs: 1000,
		URL:           "https://example.com/webhook",
		Method:        "POST",
		TimeoutMs:     5000,
		CreatedAtMs:   time.Now().UnixMilli(),
		UpdatedAtMs:   time.Now().UnixMilli(),
		Status:        model.JobStatusPending,
	}
	_ = store.CreateJob(ctx, job)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		job.Attempt = i % 10
		job.UpdatedAtMs = time.Now().UnixMilli()
		_ = store.UpdateJob(ctx, job)
	}
}

func BenchmarkScheduleOperations(b *testing.B) {
	store, err := NewRedisStore(getTestRedisURL(), "bench")
	if err != nil {
		b.Skip("Redis not available:", err)
	}
	defer store.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		jobID := fmt.Sprintf("bench-sched-%d", i)
		dueAtMs := time.Now().Add(time.Hour).UnixMilli()

		_ = store.AddToSchedule(ctx, jobID, dueAtMs)
		_ = store.RemoveFromSchedule(ctx, jobID)
	}
}
