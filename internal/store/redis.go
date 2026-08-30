// Package store implements Redis storage for TickHook.
// PRD Reference: Section 8 - Redis Data Model
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/cr0hn/tickhook/internal/model"
)

// ErrJobNotFound is returned when a job is not found.
var ErrJobNotFound = errors.New("job not found")

// RedisStore implements the Store interface using Redis.
type RedisStore struct {
	client    *redis.Client
	namespace string
}

// NewRedisStore creates a new Redis store.
func NewRedisStore(redisURL, namespace string) (*RedisStore, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	return &RedisStore{
		client:    client,
		namespace: namespace,
	}, nil
}

const (
	// Default lease time for jobs (5 minutes in milliseconds)
	DefaultJobLeaseMs = 5 * 60 * 1000
)

// schedulesKey returns the key for the schedules ZSET.
func (s *RedisStore) schedulesKey() string {
	return fmt.Sprintf("%s:schedules", s.namespace)
}

// runningKey returns the key for the running jobs ZSET (jobs with active leases).
func (s *RedisStore) runningKey() string {
	return fmt.Sprintf("%s:running", s.namespace)
}

// jobKey returns the key for a job hash.
func (s *RedisStore) jobKey(jobID string) string {
	return fmt.Sprintf("%s:job:%s", s.namespace, jobID)
}

// CreateJob stores a new job and adds it to the schedule.
func (s *RedisStore) CreateJob(ctx context.Context, job *model.Job) error {
	jobData, err := s.serializeJob(job)
	if err != nil {
		return fmt.Errorf("failed to serialize job: %w", err)
	}

	pipe := s.client.TxPipeline()

	// Store job hash
	pipe.HSet(ctx, s.jobKey(job.ID), jobData)

	// Add to schedule ZSET
	pipe.ZAdd(ctx, s.schedulesKey(), redis.Z{
		Score:  float64(job.DueAtMs),
		Member: job.ID,
	})

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	return nil
}

// GetJob retrieves a job by ID.
func (s *RedisStore) GetJob(ctx context.Context, jobID string) (*model.Job, error) {
	data, err := s.client.HGetAll(ctx, s.jobKey(jobID)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	if len(data) == 0 {
		return nil, ErrJobNotFound
	}

	return s.deserializeJob(data)
}

// UpdateJob updates an existing job.
func (s *RedisStore) UpdateJob(ctx context.Context, job *model.Job) error {
	jobData, err := s.serializeJob(job)
	if err != nil {
		return fmt.Errorf("failed to serialize job: %w", err)
	}

	pipe := s.client.TxPipeline()
	pipe.HSet(ctx, s.jobKey(job.ID), jobData)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	return nil
}

// DeleteJob removes a job from storage and schedule.
func (s *RedisStore) DeleteJob(ctx context.Context, jobID string) error {
	pipe := s.client.TxPipeline()
	pipe.Del(ctx, s.jobKey(jobID))
	pipe.ZRem(ctx, s.schedulesKey(), jobID)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}
	return nil
}

// GetDueJobs fetches jobs that are due for execution.
func (s *RedisStore) GetDueJobs(ctx context.Context, nowMs int64, limit int) ([]string, error) {
	result, err := s.client.ZRangeByScore(ctx, s.schedulesKey(), &redis.ZRangeBy{
		Min:   "-inf",
		Max:   strconv.FormatInt(nowMs, 10),
		Count: int64(limit),
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get due jobs: %w", err)
	}
	return result, nil
}

// RemoveFromSchedule removes a job from the schedule ZSET.
func (s *RedisStore) RemoveFromSchedule(ctx context.Context, jobID string) error {
	err := s.client.ZRem(ctx, s.schedulesKey(), jobID).Err()
	if err != nil {
		return fmt.Errorf("failed to remove from schedule: %w", err)
	}
	return nil
}

// AddToSchedule adds a job to the schedule ZSET with the given due time.
func (s *RedisStore) AddToSchedule(ctx context.Context, jobID string, dueAtMs int64) error {
	err := s.client.ZAdd(ctx, s.schedulesKey(), redis.Z{
		Score:  float64(dueAtMs),
		Member: jobID,
	}).Err()
	if err != nil {
		return fmt.Errorf("failed to add to schedule: %w", err)
	}
	return nil
}

// Close closes the Redis connection.
func (s *RedisStore) Close() error {
	return s.client.Close()
}

// Ping checks if Redis is reachable.
func (s *RedisStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

// ClaimJobs atomically claims jobs for execution with a lease mechanism.
// This prevents job loss during crashes by moving jobs to a "running" set with expiry.
func (s *RedisStore) ClaimJobs(ctx context.Context, nowMs int64, limit int, leaseMs int64) ([]string, error) {
	leaseUntil := nowMs + leaseMs
	
	luaScript := `
	local schedules_key = KEYS[1]
	local running_key = KEYS[2]
	local now_ms = tonumber(ARGV[1])
	local limit = tonumber(ARGV[2])
	local lease_until = tonumber(ARGV[3])
	
	-- Get due jobs
	local due_jobs = redis.call('ZRANGEBYSCORE', schedules_key, '-inf', now_ms, 'LIMIT', 0, limit)
	
	-- Move each job to running set with lease expiry
	for i, job_id in ipairs(due_jobs) do
		redis.call('ZADD', running_key, lease_until, job_id)
		redis.call('ZREM', schedules_key, job_id)
	end
	
	return due_jobs
	`
	
	script := redis.NewScript(luaScript)
	keys := []string{s.schedulesKey(), s.runningKey()}
	
	result, err := script.Run(ctx, s.client, keys, nowMs, limit, leaseUntil).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to claim jobs: %w", err)
	}
	
	jobIDs, ok := result.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected result type from Lua script")
	}
	
	strings := make([]string, len(jobIDs))
	for i, v := range jobIDs {
		strings[i] = v.(string)
	}
	
	return strings, nil
}

// RenewJobLease renews the lease for a job that's still being processed.
func (s *RedisStore) RenewJobLease(ctx context.Context, jobID string, leaseMs int64) error {
	nowMs := NowMs()
	leaseUntil := nowMs + leaseMs
	
	// Update the score in the running ZSET
	err := s.client.ZAdd(ctx, s.runningKey(), redis.Z{
		Score:  float64(leaseUntil),
		Member: jobID,
	}).Err()
	
	if err != nil {
		return fmt.Errorf("failed to renew job lease: %w", err)
	}
	
	return nil
}

// ReleaseJobLease releases a job from the running set after successful execution.
func (s *RedisStore) ReleaseJobLease(ctx context.Context, jobID string) error {
	err := s.client.ZRem(ctx, s.runningKey(), jobID).Err()
	if err != nil {
		return fmt.Errorf("failed to release job lease: %w", err)
	}
	return nil
}

// GetStaleJobs returns jobs whose leases have expired (crashed workers).
func (s *RedisStore) GetStaleJobs(ctx context.Context, nowMs int64, limit int) ([]string, error) {
	result, err := s.client.ZRangeByScore(ctx, s.runningKey(), &redis.ZRangeBy{
		Min:   "-inf",
		Max:   strconv.FormatInt(nowMs, 10),
		Count: int64(limit),
	}).Result()
	
	if err != nil {
		return nil, fmt.Errorf("failed to get stale jobs: %w", err)
	}
	
	return result, nil
}

// NowMs returns current time in milliseconds (helper for store package).
func NowMs() int64 {
	return time.Now().UTC().UnixMilli()
}

// serializeJob converts a Job to a map for Redis HSET.
func (s *RedisStore) serializeJob(job *model.Job) (map[string]any, error) {
	data := map[string]any{
		"id":              job.ID,
		"type":            string(job.Type),
		"due_at_ms":       strconv.FormatInt(job.DueAtMs, 10),
		"attempt":         strconv.Itoa(job.Attempt),
		"max_attempts":    strconv.Itoa(job.MaxAttempts),
		"backoff_base_ms": strconv.FormatInt(job.BackoffBaseMs, 10),
		"url":             job.URL,
		"method":          job.Method,
		"timeout_ms":      strconv.Itoa(job.TimeoutMs),
		"created_at_ms":   strconv.FormatInt(job.CreatedAtMs, 10),
		"updated_at_ms":   strconv.FormatInt(job.UpdatedAtMs, 10),
		"status":          string(job.Status),
	}

	if job.IntervalMs > 0 {
		data["interval_ms"] = strconv.FormatInt(job.IntervalMs, 10)
	}

	if job.Headers != nil {
		headersJSON, err := json.Marshal(job.Headers)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal headers: %w", err)
		}
		data["headers_json"] = string(headersJSON)
	}

	if job.Body != nil {
		bodyJSON, err := json.Marshal(job.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body: %w", err)
		}
		data["body_json"] = string(bodyJSON)
	}

	if job.LastError != "" {
		data["last_error"] = job.LastError
	}

	if job.LastHTTPCode != 0 {
		data["last_http_code"] = strconv.Itoa(job.LastHTTPCode)
	}

	return data, nil
}

// deserializeJob converts a Redis hash map to a Job.
func (s *RedisStore) deserializeJob(data map[string]string) (*model.Job, error) {
	job := &model.Job{}

	job.ID = data["id"]
	job.Type = model.JobType(data["type"])

	var err error
	job.DueAtMs, err = strconv.ParseInt(data["due_at_ms"], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid due_at_ms: %w", err)
	}

	job.Attempt, err = strconv.Atoi(data["attempt"])
	if err != nil {
		return nil, fmt.Errorf("invalid attempt: %w", err)
	}

	job.MaxAttempts, err = strconv.Atoi(data["max_attempts"])
	if err != nil {
		return nil, fmt.Errorf("invalid max_attempts: %w", err)
	}

	job.BackoffBaseMs, err = strconv.ParseInt(data["backoff_base_ms"], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid backoff_base_ms: %w", err)
	}

	job.URL = data["url"]
	job.Method = data["method"]

	job.TimeoutMs, err = strconv.Atoi(data["timeout_ms"])
	if err != nil {
		return nil, fmt.Errorf("invalid timeout_ms: %w", err)
	}

	job.CreatedAtMs, err = strconv.ParseInt(data["created_at_ms"], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid created_at_ms: %w", err)
	}

	job.UpdatedAtMs, err = strconv.ParseInt(data["updated_at_ms"], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid updated_at_ms: %w", err)
	}

	job.Status = model.JobStatus(data["status"])

	if intervalStr, ok := data["interval_ms"]; ok && intervalStr != "" {
		job.IntervalMs, err = strconv.ParseInt(intervalStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid interval_ms: %w", err)
		}
	}

	if headersJSON, ok := data["headers_json"]; ok && headersJSON != "" {
		if err := json.Unmarshal([]byte(headersJSON), &job.Headers); err != nil {
			return nil, fmt.Errorf("invalid headers_json: %w", err)
		}
	}

	if bodyJSON, ok := data["body_json"]; ok && bodyJSON != "" {
		if err := json.Unmarshal([]byte(bodyJSON), &job.Body); err != nil {
			return nil, fmt.Errorf("invalid body_json: %w", err)
		}
	}

	job.LastError = data["last_error"]

	if lastHTTPCodeStr, ok := data["last_http_code"]; ok && lastHTTPCodeStr != "" {
		job.LastHTTPCode, err = strconv.Atoi(lastHTTPCodeStr)
		if err != nil {
			return nil, fmt.Errorf("invalid last_http_code: %w", err)
		}
	}

	return job, nil
}
