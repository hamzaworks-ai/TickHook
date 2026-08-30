// Package store defines the storage interface for TickHook.
// PRD Reference: Section 8 - Redis Data Model
package store

import (
	"context"

	"github.com/cr0hn/tickhook/internal/model"
)

// Store defines the interface for job storage operations.
// This interface allows for future implementations (e.g., HA in V2).
type Store interface {
	// CreateJob stores a new job and adds it to the schedule.
	CreateJob(ctx context.Context, job *model.Job) error

	// GetJob retrieves a job by ID.
	GetJob(ctx context.Context, jobID string) (*model.Job, error)

	// UpdateJob updates an existing job.
	UpdateJob(ctx context.Context, job *model.Job) error

	// DeleteJob removes a job from storage and schedule.
	DeleteJob(ctx context.Context, jobID string) error

	// GetDueJobs fetches jobs that are due for execution.
	// Returns job IDs with due_at_ms <= nowMs, up to limit.
	GetDueJobs(ctx context.Context, nowMs int64, limit int) ([]string, error)

	// RemoveFromSchedule removes a job from the schedule ZSET.
	RemoveFromSchedule(ctx context.Context, jobID string) error

	// AddToSchedule adds a job to the schedule ZSET with the given due time.
	AddToSchedule(ctx context.Context, jobID string, dueAtMs int64) error

	// Close closes the store connection.
	Close() error

	// Ping checks if the store is reachable.
	Ping(ctx context.Context) error

	// ClaimJobs atomically claims jobs for execution with a lease mechanism.
	// Returns job IDs that were successfully claimed and moved to the running set.
	// This prevents job loss during crashes.
	ClaimJobs(ctx context.Context, nowMs int64, limit int, leaseMs int64) ([]string, error)

	// RenewJobLease renews the lease for a job that's still being processed.
	RenewJobLease(ctx context.Context, jobID string, leaseMs int64) error

	// ReleaseJobLease releases a job from the running set after successful execution.
	ReleaseJobLease(ctx context.Context, jobID string) error

	// GetStaleJobs returns jobs whose leases have expired (crashed workers).
	GetStaleJobs(ctx context.Context, nowMs int64, limit int) ([]string, error)
}
