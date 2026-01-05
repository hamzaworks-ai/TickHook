// Package scheduler implements the polling loop for TickHook.
// PRD Reference: Section 10 - Scheduler Loop
package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/cr0hn/tickhook/internal/config"
	"github.com/cr0hn/tickhook/internal/store"
	"github.com/cr0hn/tickhook/internal/util"
)

// JobDispatcher is called when a job is due for execution.
type JobDispatcher func(jobID string)

// Scheduler polls Redis for due jobs and dispatches them.
type Scheduler struct {
	cfg        *config.Config
	store      store.Store
	logger     *slog.Logger
	dispatcher JobDispatcher
	stopCh     chan struct{}
	doneCh     chan struct{}
}

// NewScheduler creates a new scheduler.
func NewScheduler(cfg *config.Config, store store.Store, logger *slog.Logger, dispatcher JobDispatcher) *Scheduler {
	return &Scheduler{
		cfg:        cfg,
		store:      store,
		logger:     logger,
		dispatcher: dispatcher,
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
	}
}

// Start starts the scheduler polling loop.
func (s *Scheduler) Start(ctx context.Context) {
	s.logger.Info("Scheduler starting", "poll_ms", s.cfg.PollMs, "batch", s.cfg.Batch)

	ticker := time.NewTicker(time.Duration(s.cfg.PollMs) * time.Millisecond)
	defer ticker.Stop()
	defer close(s.doneCh)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Scheduler stopping (context cancelled)")
			return
		case <-s.stopCh:
			s.logger.Info("Scheduler stopping (stop signal)")
			return
		case <-ticker.C:
			s.poll(ctx)
		}
	}
}

// Stop signals the scheduler to stop.
func (s *Scheduler) Stop() {
	close(s.stopCh)
}

// Wait waits for the scheduler to finish.
func (s *Scheduler) Wait() {
	<-s.doneCh
}

// poll fetches due jobs and dispatches them.
func (s *Scheduler) poll(ctx context.Context) {
	nowMs := util.NowMs()

	jobIDs, err := s.store.GetDueJobs(ctx, nowMs, s.cfg.Batch)
	if err != nil {
		s.logger.Error("Failed to fetch due jobs", "error", err)
		return
	}

	if len(jobIDs) == 0 {
		return
	}

	s.logger.Debug("Found due jobs", "count", len(jobIDs))

	for _, jobID := range jobIDs {
		// Remove from schedule to prevent re-processing in V1 single-consumer model
		// Note: This creates a crash window where job could be lost if process crashes
		// after ZREM but before completion. Acceptable for V1.
		if err := s.store.RemoveFromSchedule(ctx, jobID); err != nil {
			s.logger.Error("Failed to remove job from schedule", "job_id", jobID, "error", err)
			continue
		}

		s.logger.Debug("Dispatching job", "job_id", jobID)
		s.dispatcher(jobID)
	}
}
