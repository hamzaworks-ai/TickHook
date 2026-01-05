package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/cr0hn/tickhook/internal/config"
	"github.com/cr0hn/tickhook/internal/model"
)

// mockStore implements store.Store for testing
type mockStore struct {
	mu       sync.Mutex
	dueJobs  []string
	removed  []string
	schedule map[string]int64
}

func newMockStore() *mockStore {
	return &mockStore{
		dueJobs:  []string{},
		removed:  []string{},
		schedule: make(map[string]int64),
	}
}

func (m *mockStore) CreateJob(ctx context.Context, job *model.Job) error {
	return nil
}

func (m *mockStore) GetJob(ctx context.Context, jobID string) (*model.Job, error) {
	return nil, nil
}

func (m *mockStore) UpdateJob(ctx context.Context, job *model.Job) error {
	return nil
}

func (m *mockStore) DeleteJob(ctx context.Context, jobID string) error {
	return nil
}

func (m *mockStore) GetDueJobs(ctx context.Context, nowMs int64, limit int) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]string, len(m.dueJobs))
	copy(result, m.dueJobs)

	// Clear for next call
	m.dueJobs = []string{}
	return result, nil
}

func (m *mockStore) RemoveFromSchedule(ctx context.Context, jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removed = append(m.removed, jobID)
	return nil
}

func (m *mockStore) AddToSchedule(ctx context.Context, jobID string, dueAtMs int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.schedule[jobID] = dueAtMs
	return nil
}

func (m *mockStore) Close() error {
	return nil
}

func (m *mockStore) Ping(ctx context.Context) error {
	return nil
}

func (m *mockStore) setDueJobs(jobs []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dueJobs = jobs
}

func (m *mockStore) getRemovedJobs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.removed))
	copy(result, m.removed)
	return result
}

func TestScheduler_DispatchesDueJobs(t *testing.T) {
	cfg := &config.Config{
		PollMs: 50, // Fast polling for test
		Batch:  100,
	}
	store := newMockStore()
	logger := slog.Default()

	var dispatched []string
	var mu sync.Mutex

	dispatcher := func(jobID string) {
		mu.Lock()
		dispatched = append(dispatched, jobID)
		mu.Unlock()
	}

	sched := NewScheduler(cfg, store, logger, dispatcher)

	// Set up due jobs
	store.setDueJobs([]string{"job-1", "job-2", "job-3"})

	ctx, cancel := context.WithCancel(context.Background())

	// Start scheduler in background
	go sched.Start(ctx)

	// Wait for poll cycle
	time.Sleep(100 * time.Millisecond)

	cancel()
	sched.Wait()

	mu.Lock()
	defer mu.Unlock()

	if len(dispatched) != 3 {
		t.Errorf("Dispatched %d jobs, want 3", len(dispatched))
	}

	// Verify jobs were removed from schedule
	removed := store.getRemovedJobs()
	if len(removed) != 3 {
		t.Errorf("Removed %d jobs from schedule, want 3", len(removed))
	}
}

func TestScheduler_StopsOnContextCancel(t *testing.T) {
	cfg := &config.Config{
		PollMs: 10,
		Batch:  100,
	}
	store := newMockStore()
	logger := slog.Default()

	dispatcher := func(jobID string) {}

	sched := NewScheduler(cfg, store, logger, dispatcher)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		sched.Start(ctx)
		close(done)
	}()

	// Cancel context
	cancel()

	// Should stop within reasonable time
	select {
	case <-done:
		// Expected
	case <-time.After(time.Second):
		t.Error("Scheduler did not stop after context cancel")
	}
}

func TestScheduler_StopsOnStopSignal(t *testing.T) {
	cfg := &config.Config{
		PollMs: 10,
		Batch:  100,
	}
	store := newMockStore()
	logger := slog.Default()

	dispatcher := func(jobID string) {}

	sched := NewScheduler(cfg, store, logger, dispatcher)

	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		sched.Start(ctx)
		close(done)
	}()

	// Stop scheduler
	sched.Stop()

	// Should stop within reasonable time
	select {
	case <-done:
		// Expected
	case <-time.After(time.Second):
		t.Error("Scheduler did not stop after Stop() call")
	}
}

func TestScheduler_NoDueJobs(t *testing.T) {
	cfg := &config.Config{
		PollMs: 50,
		Batch:  100,
	}
	store := newMockStore()
	logger := slog.Default()

	dispatchCount := 0
	var mu sync.Mutex

	dispatcher := func(jobID string) {
		mu.Lock()
		dispatchCount++
		mu.Unlock()
	}

	sched := NewScheduler(cfg, store, logger, dispatcher)

	ctx, cancel := context.WithCancel(context.Background())

	go sched.Start(ctx)

	// Wait for a few poll cycles with no jobs
	time.Sleep(150 * time.Millisecond)

	cancel()
	sched.Wait()

	mu.Lock()
	defer mu.Unlock()

	if dispatchCount != 0 {
		t.Errorf("Dispatched %d jobs, want 0", dispatchCount)
	}
}
