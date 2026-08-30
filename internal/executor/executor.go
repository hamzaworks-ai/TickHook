// Package executor implements the worker pool for webhook execution.
// PRD Reference: Section 11 - Webhook Execution
package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"syscall"
	"time"

	"github.com/cr0hn/tickhook/internal/config"
	"github.com/cr0hn/tickhook/internal/model"
	"github.com/cr0hn/tickhook/internal/store"
	"github.com/cr0hn/tickhook/internal/util"
)

// dialerWithSSRFProtection creates a custom dialer that prevents SSRF attacks
// SECURITY FIX: Blocks connections to private IPs at the network level
func dialerWithSSRFProtection() *net.Dialer {
	return &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return fmt.Errorf("failed to parse address: %w", err)
			}
			
			ip := net.ParseIP(host)
			if ip == nil {
				// If not an IP, try to resolve it
				ips, err := net.LookupIP(host)
				if err != nil {
					return fmt.Errorf("failed to resolve host: %w", err)
				}
				if len(ips) > 0 {
					ip = ips[0]
				}
			}
			
			if ip != nil && util.IsPrivateIP(ip) {
				return fmt.Errorf("connection to private IP blocked: %s", ip)
			}
			
			return nil
		},
	}
}

// Executor executes webhook jobs with concurrency control.
type Executor struct {
	cfg          *config.Config
	store        store.Store
	logger       *slog.Logger
	httpClient   *http.Client
	globalSem    chan struct{}
	domainSems   map[string]chan struct{}
	domainSemsMu sync.RWMutex
	jobQueue     chan string
	wg           sync.WaitGroup
	stopCh       chan struct{}
}

// NewExecutor creates a new executor.
func NewExecutor(cfg *config.Config, store store.Store, logger *slog.Logger) *Executor {
	// SECURITY FIX: Configure HTTP client with SSRF protection and proper timeouts
	dialer := dialerWithSSRFProtection()
	
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			ForceAttemptHTTP2:     true,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// SECURITY FIX: Prevent redirect loops and limit redirects
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		},
	}

	return &Executor{
		cfg:        cfg,
		store:      store,
		logger:     logger,
		httpClient: httpClient,
		globalSem:  make(chan struct{}, cfg.MaxInflight),
		domainSems: make(map[string]chan struct{}),
		jobQueue:   make(chan string, cfg.MaxInflight*2),
		stopCh:     make(chan struct{}),
	}
}

// Start starts the executor workers.
func (e *Executor) Start(ctx context.Context) {
	e.logger.Info("Executor starting", "max_inflight", e.cfg.MaxInflight, "max_per_domain", e.cfg.MaxPerDomain)

	// Start worker goroutines
	for i := 0; i < e.cfg.MaxInflight; i++ {
		e.wg.Add(1)
		go e.worker(ctx)
	}
}

// Stop signals the executor to stop.
func (e *Executor) Stop() {
	close(e.stopCh)
	close(e.jobQueue)
}

// Wait waits for all workers to finish.
func (e *Executor) Wait() {
	e.wg.Wait()
}

// Dispatch adds a job to the execution queue.
func (e *Executor) Dispatch(jobID string) {
	select {
	case e.jobQueue <- jobID:
	case <-e.stopCh:
		e.logger.Warn("Executor stopped, job not dispatched", "job_id", jobID)
	}
}

// worker processes jobs from the queue.
func (e *Executor) worker(ctx context.Context) {
	defer e.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case jobID, ok := <-e.jobQueue:
			if !ok {
				return
			}
			e.executeJob(ctx, jobID)
		}
	}
}

// executeJob executes a single job.
func (e *Executor) executeJob(ctx context.Context, jobID string) {
	logger := e.logger.With("job_id", jobID)

	// Acquire global semaphore
	select {
	case e.globalSem <- struct{}{}:
		defer func() { <-e.globalSem }()
	case <-ctx.Done():
		return
	case <-e.stopCh:
		return
	}

	// Load job from store
	job, err := e.store.GetJob(ctx, jobID)
	if err != nil {
		logger.Error("Failed to load job", "error", err)
		return
	}

	// Extract domain for per-domain limiting
	domain, err := util.ExtractDomain(job.URL)
	if err != nil {
		logger.Error("Failed to extract domain", "url", job.URL, "error", err)
		e.handleExecutionFailure(ctx, job, "invalid URL", 0)
		return
	}

	// Acquire domain semaphore
	domainSem := e.getDomainSemaphore(domain)
	select {
	case domainSem <- struct{}{}:
		defer func() { <-domainSem }()
	case <-ctx.Done():
		return
	case <-e.stopCh:
		return
	}

	logger.Info("Executing webhook", "url", job.URL, "method", job.Method, "attempt", job.Attempt+1)

	// Execute the webhook
	statusCode, execErr := e.doWebhook(ctx, job)

	if execErr != nil {
		logger.Warn("Webhook execution failed", "error", execErr, "status_code", statusCode)
		e.handleExecutionFailure(ctx, job, execErr.Error(), statusCode)
		return
	}

	if util.IsSuccessHTTPStatus(statusCode) {
		logger.Info("Webhook executed successfully", "status_code", statusCode)
		e.handleExecutionSuccess(ctx, job)
		return
	}

	if util.IsClientErrorHTTPStatus(statusCode) {
		logger.Warn("Webhook returned client error (non-retryable)", "status_code", statusCode)
		e.handleNonRetryableFailure(ctx, job, fmt.Sprintf("HTTP %d", statusCode), statusCode)
		return
	}

	// Retryable error (5xx or 429)
	logger.Warn("Webhook returned retryable error", "status_code", statusCode)
	e.handleExecutionFailure(ctx, job, fmt.Sprintf("HTTP %d", statusCode), statusCode)
}

// doWebhook executes the HTTP request for a job.
func (e *Executor) doWebhook(ctx context.Context, job *model.Job) (int, error) {
	// Create request body
	var bodyReader io.Reader
	if job.Body != nil {
		bodyJSON, err := json.Marshal(job.Body)
		if err != nil {
			return 0, fmt.Errorf("failed to marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyJSON)
	}

	// Create request with timeout
	timeout := time.Duration(job.TimeoutMs) * time.Millisecond
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, job.Method, job.URL, bodyReader)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	// Set Content-Type for requests with body
	if job.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Add user headers
	for key, value := range job.Headers {
		req.Header.Set(key, value)
	}

	// Add TickHook automatic headers (these cannot be overridden)
	// PRD Reference: Section 11 - Automatic Idempotency headers
	req.Header.Set("X-Job-Id", job.ID)
	req.Header.Set("Idempotency-Key", job.ID)

	// Execute request
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Drain body to allow connection reuse
	_, _ = io.Copy(io.Discard, resp.Body)

	return resp.StatusCode, nil
}

// handleExecutionSuccess handles successful webhook execution.
func (e *Executor) handleExecutionSuccess(ctx context.Context, job *model.Job) {
	logger := e.logger.With("job_id", job.ID)

	if job.Type == model.JobTypeOneShot {
		// Delete one-shot job on success
		// PRD Reference: Section 14 - One-Shot Jobs (Deleted on success)
		if err := e.store.DeleteJob(ctx, job.ID); err != nil {
			logger.Error("Failed to delete one-shot job after success", "error", err)
		}
		return
	}

	// Recurring job: schedule next execution
	// PRD Reference: Section 13 - Recurring Jobs (Fixed interval scheduling)
	// Schedule is computed from the previously scheduled due_at, not completion time
	nextDueAtMs := job.DueAtMs + job.IntervalMs
	job.DueAtMs = nextDueAtMs
	job.Attempt = 0
	job.LastError = ""
	job.LastHTTPCode = 0
	job.UpdatedAtMs = util.NowMs()
	job.Status = model.JobStatusPending

	if err := e.store.UpdateJob(ctx, job); err != nil {
		logger.Error("Failed to update recurring job", "error", err)
		return
	}

	if err := e.store.AddToSchedule(ctx, job.ID, nextDueAtMs); err != nil {
		logger.Error("Failed to reschedule recurring job", "error", err)
		return
	}

	logger.Info("Recurring job rescheduled", "next_due_at_ms", nextDueAtMs)
}

// handleExecutionFailure handles failed webhook execution (retryable).
func (e *Executor) handleExecutionFailure(ctx context.Context, job *model.Job, errMsg string, statusCode int) {
	logger := e.logger.With("job_id", job.ID)

	job.Attempt++
	job.LastError = errMsg
	job.LastHTTPCode = statusCode
	job.UpdatedAtMs = util.NowMs()

	if job.Attempt >= job.MaxAttempts {
		// Final failure
		// PRD Reference: Section 14 - One-Shot Jobs (Retained on failure)
		job.Status = model.JobStatusFailed
		if err := e.store.UpdateJob(ctx, job); err != nil {
			logger.Error("Failed to update job after final failure", "error", err)
		}
		logger.Warn("Job reached max attempts, marked as failed", "attempts", job.Attempt)
		return
	}

	// Schedule retry with backoff
	// PRD Reference: Section 12 - Retry Policy (Exponential backoff with jitter)
	backoffMs := util.CalculateBackoff(job.Attempt, job.BackoffBaseMs, util.DefaultJitterMaxMs)
	nextDueAtMs := util.NowMs() + backoffMs
	job.DueAtMs = nextDueAtMs

	if err := e.store.UpdateJob(ctx, job); err != nil {
		logger.Error("Failed to update job for retry", "error", err)
		return
	}

	if err := e.store.AddToSchedule(ctx, job.ID, nextDueAtMs); err != nil {
		logger.Error("Failed to reschedule job for retry", "error", err)
		return
	}

	logger.Info("Job scheduled for retry", "attempt", job.Attempt, "next_due_at_ms", nextDueAtMs, "backoff_ms", backoffMs)
}

// handleNonRetryableFailure handles non-retryable failures (4xx except 429).
func (e *Executor) handleNonRetryableFailure(ctx context.Context, job *model.Job, errMsg string, statusCode int) {
	logger := e.logger.With("job_id", job.ID)

	// Treat as final failure immediately
	job.Attempt = job.MaxAttempts
	job.LastError = errMsg
	job.LastHTTPCode = statusCode
	job.UpdatedAtMs = util.NowMs()
	job.Status = model.JobStatusFailed

	if err := e.store.UpdateJob(ctx, job); err != nil {
		logger.Error("Failed to update job after non-retryable failure", "error", err)
	}

	logger.Warn("Job failed with non-retryable error", "status_code", statusCode)
}

// getDomainSemaphore returns the semaphore for a domain, creating it if needed.
func (e *Executor) getDomainSemaphore(domain string) chan struct{} {
	e.domainSemsMu.RLock()
	sem, ok := e.domainSems[domain]
	e.domainSemsMu.RUnlock()

	if ok {
		return sem
	}

	e.domainSemsMu.Lock()
	defer e.domainSemsMu.Unlock()

	// Double-check after acquiring write lock
	if sem, ok = e.domainSems[domain]; ok {
		return sem
	}

	sem = make(chan struct{}, e.cfg.MaxPerDomain)
	e.domainSems[domain] = sem
	return sem
}
