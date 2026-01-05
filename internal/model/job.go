// Package model defines the core data structures for TickHook jobs.
// PRD Reference: Section 8 - Redis Data Model
package model

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// JobType represents the type of job (one-shot or recurring).
type JobType string

const (
	JobTypeOneShot   JobType = "one_shot"
	JobTypeRecurring JobType = "recurring"
)

// JobStatus represents the current status of a job.
type JobStatus string

const (
	JobStatusPending JobStatus = "pending"
	JobStatusFailed  JobStatus = "failed"
)

// AllowedMethods contains the HTTP methods allowed for webhooks.
var AllowedMethods = map[string]bool{
	"GET":    true,
	"POST":   true,
	"PUT":    true,
	"PATCH":  true,
	"DELETE": true,
}

// Job represents a scheduled webhook job.
type Job struct {
	ID            string            `json:"job_id"`
	Type          JobType           `json:"type"`
	DueAtMs       int64             `json:"due_at_ms"`
	IntervalMs    int64             `json:"interval_ms,omitempty"` // Only for recurring jobs
	Attempt       int               `json:"attempt"`
	MaxAttempts   int               `json:"max_attempts"`
	BackoffBaseMs int64             `json:"backoff_base_ms"`
	URL           string            `json:"url"`
	Method        string            `json:"method"`
	Headers       map[string]string `json:"headers,omitempty"`
	Body          map[string]any    `json:"body,omitempty"`
	TimeoutMs     int               `json:"timeout_ms"`
	CreatedAtMs   int64             `json:"created_at_ms"`
	UpdatedAtMs   int64             `json:"updated_at_ms"`
	LastError     string            `json:"last_error,omitempty"`
	LastHTTPCode  int               `json:"last_http_code,omitempty"`
	Status        JobStatus         `json:"status"`
}

// WebhookConfig holds the webhook configuration from API requests.
type WebhookConfig struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers,omitempty"`
	Body      map[string]any    `json:"body,omitempty"`
	TimeoutMs int               `json:"timeout_ms,omitempty"`
}

// RetryConfig holds retry configuration from API requests.
type RetryConfig struct {
	MaxAttempts   int   `json:"max_attempts"`
	BackoffBaseMs int64 `json:"backoff_base_ms"`
}

// CreateOneShotRequest is the request body for creating a one-shot job.
type CreateOneShotRequest struct {
	ExecuteAt time.Time      `json:"execute_at"`
	Webhook   WebhookConfig  `json:"webhook"`
	Retry     *RetryConfig   `json:"retry,omitempty"`
}

// CreateRecurringRequest is the request body for creating a recurring job.
type CreateRecurringRequest struct {
	StartAt    time.Time     `json:"start_at"`
	IntervalMs int64         `json:"interval_ms"`
	Webhook    WebhookConfig `json:"webhook"`
	Retry      *RetryConfig  `json:"retry,omitempty"`
}

// CreateJobResponse is the response for job creation.
type CreateJobResponse struct {
	JobID string `json:"job_id"`
}

// DeleteJobResponse is the response for job deletion.
type DeleteJobResponse struct {
	Deleted bool `json:"deleted"`
}

// ErrorResponse is the standard error response format.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// Validate validates a WebhookConfig.
func (w *WebhookConfig) Validate(defaultTimeoutMs int) error {
	if w.URL == "" {
		return errors.New("webhook.url is required")
	}

	parsedURL, err := url.Parse(w.URL)
	if err != nil {
		return fmt.Errorf("webhook.url is invalid: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("webhook.url must be http:// or https://, got %s://", parsedURL.Scheme)
	}

	if parsedURL.Host == "" {
		return errors.New("webhook.url must have a host")
	}

	// Validate method
	method := strings.ToUpper(w.Method)
	if method == "" {
		method = "POST"
		w.Method = method
	}
	if !AllowedMethods[method] {
		return fmt.Errorf("webhook.method must be one of GET, POST, PUT, PATCH, DELETE; got %s", w.Method)
	}
	w.Method = method

	// Set default timeout
	if w.TimeoutMs <= 0 {
		w.TimeoutMs = defaultTimeoutMs
	}

	return nil
}

// Validate validates a RetryConfig.
func (r *RetryConfig) Validate() error {
	if r.MaxAttempts < 1 {
		return errors.New("retry.max_attempts must be >= 1")
	}
	if r.BackoffBaseMs <= 0 {
		return errors.New("retry.backoff_base_ms must be > 0")
	}
	return nil
}

// Validate validates a CreateOneShotRequest.
func (r *CreateOneShotRequest) Validate(defaultTimeoutMs int) error {
	if r.ExecuteAt.IsZero() {
		return errors.New("execute_at is required")
	}

	if err := r.Webhook.Validate(defaultTimeoutMs); err != nil {
		return err
	}

	if r.Retry != nil {
		if err := r.Retry.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// Validate validates a CreateRecurringRequest.
func (r *CreateRecurringRequest) Validate(defaultTimeoutMs int) error {
	if r.StartAt.IsZero() {
		return errors.New("start_at is required")
	}

	if r.IntervalMs <= 0 {
		return errors.New("interval_ms must be > 0")
	}

	if err := r.Webhook.Validate(defaultTimeoutMs); err != nil {
		return err
	}

	if r.Retry != nil {
		if err := r.Retry.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// DefaultRetryConfig returns default retry configuration.
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:   3,
		BackoffBaseMs: 1000,
	}
}

// NewOneShotJob creates a new one-shot job from a request.
func NewOneShotJob(id string, req *CreateOneShotRequest, defaultTimeoutMs int) *Job {
	retry := req.Retry
	if retry == nil {
		retry = DefaultRetryConfig()
	}

	now := time.Now().UTC().UnixMilli()
	timeoutMs := req.Webhook.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = defaultTimeoutMs
	}

	return &Job{
		ID:            id,
		Type:          JobTypeOneShot,
		DueAtMs:       req.ExecuteAt.UnixMilli(),
		Attempt:       0,
		MaxAttempts:   retry.MaxAttempts,
		BackoffBaseMs: retry.BackoffBaseMs,
		URL:           req.Webhook.URL,
		Method:        req.Webhook.Method,
		Headers:       req.Webhook.Headers,
		Body:          req.Webhook.Body,
		TimeoutMs:     timeoutMs,
		CreatedAtMs:   now,
		UpdatedAtMs:   now,
		Status:        JobStatusPending,
	}
}

// NewRecurringJob creates a new recurring job from a request.
func NewRecurringJob(id string, req *CreateRecurringRequest, defaultTimeoutMs int) *Job {
	retry := req.Retry
	if retry == nil {
		retry = DefaultRetryConfig()
	}

	now := time.Now().UTC().UnixMilli()
	timeoutMs := req.Webhook.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = defaultTimeoutMs
	}

	return &Job{
		ID:            id,
		Type:          JobTypeRecurring,
		DueAtMs:       req.StartAt.UnixMilli(),
		IntervalMs:    req.IntervalMs,
		Attempt:       0,
		MaxAttempts:   retry.MaxAttempts,
		BackoffBaseMs: retry.BackoffBaseMs,
		URL:           req.Webhook.URL,
		Method:        req.Webhook.Method,
		Headers:       req.Webhook.Headers,
		Body:          req.Webhook.Body,
		TimeoutMs:     timeoutMs,
		CreatedAtMs:   now,
		UpdatedAtMs:   now,
		Status:        JobStatusPending,
	}
}
