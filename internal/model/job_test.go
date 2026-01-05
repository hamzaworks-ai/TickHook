package model

import (
	"testing"
	"time"
)

func TestWebhookConfig_Validate(t *testing.T) {
	defaultTimeout := 5000

	tests := []struct {
		name    string
		webhook WebhookConfig
		wantErr bool
	}{
		{
			name: "valid POST webhook",
			webhook: WebhookConfig{
				URL:    "https://example.com/hook",
				Method: "POST",
			},
			wantErr: false,
		},
		{
			name: "valid GET webhook",
			webhook: WebhookConfig{
				URL:    "https://example.com/hook",
				Method: "GET",
			},
			wantErr: false,
		},
		{
			name: "empty URL",
			webhook: WebhookConfig{
				URL:    "",
				Method: "POST",
			},
			wantErr: true,
		},
		{
			name: "invalid URL scheme",
			webhook: WebhookConfig{
				URL:    "ftp://example.com/hook",
				Method: "POST",
			},
			wantErr: true,
		},
		{
			name: "invalid method",
			webhook: WebhookConfig{
				URL:    "https://example.com/hook",
				Method: "INVALID",
			},
			wantErr: true,
		},
		{
			name: "empty method defaults to POST",
			webhook: WebhookConfig{
				URL:    "https://example.com/hook",
				Method: "",
			},
			wantErr: false,
		},
		{
			name: "lowercase method is uppercased",
			webhook: WebhookConfig{
				URL:    "https://example.com/hook",
				Method: "post",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.webhook.Validate(defaultTimeout)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRetryConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		retry   RetryConfig
		wantErr bool
	}{
		{
			name: "valid config",
			retry: RetryConfig{
				MaxAttempts:   3,
				BackoffBaseMs: 1000,
			},
			wantErr: false,
		},
		{
			name: "zero max_attempts",
			retry: RetryConfig{
				MaxAttempts:   0,
				BackoffBaseMs: 1000,
			},
			wantErr: true,
		},
		{
			name: "zero backoff",
			retry: RetryConfig{
				MaxAttempts:   3,
				BackoffBaseMs: 0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.retry.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateOneShotRequest_Validate(t *testing.T) {
	defaultTimeout := 5000

	tests := []struct {
		name    string
		req     CreateOneShotRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: CreateOneShotRequest{
				ExecuteAt: time.Now().Add(time.Hour),
				Webhook: WebhookConfig{
					URL:    "https://example.com/hook",
					Method: "POST",
				},
			},
			wantErr: false,
		},
		{
			name: "missing execute_at",
			req: CreateOneShotRequest{
				Webhook: WebhookConfig{
					URL:    "https://example.com/hook",
					Method: "POST",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid webhook",
			req: CreateOneShotRequest{
				ExecuteAt: time.Now().Add(time.Hour),
				Webhook: WebhookConfig{
					URL:    "",
					Method: "POST",
				},
			},
			wantErr: true,
		},
		{
			name: "with valid retry config",
			req: CreateOneShotRequest{
				ExecuteAt: time.Now().Add(time.Hour),
				Webhook: WebhookConfig{
					URL:    "https://example.com/hook",
					Method: "POST",
				},
				Retry: &RetryConfig{
					MaxAttempts:   5,
					BackoffBaseMs: 2000,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate(defaultTimeout)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateRecurringRequest_Validate(t *testing.T) {
	defaultTimeout := 5000

	tests := []struct {
		name    string
		req     CreateRecurringRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: CreateRecurringRequest{
				StartAt:    time.Now().Add(time.Hour),
				IntervalMs: 86400000, // 24 hours
				Webhook: WebhookConfig{
					URL:    "https://example.com/hook",
					Method: "POST",
				},
			},
			wantErr: false,
		},
		{
			name: "missing start_at",
			req: CreateRecurringRequest{
				IntervalMs: 86400000,
				Webhook: WebhookConfig{
					URL:    "https://example.com/hook",
					Method: "POST",
				},
			},
			wantErr: true,
		},
		{
			name: "zero interval",
			req: CreateRecurringRequest{
				StartAt:    time.Now().Add(time.Hour),
				IntervalMs: 0,
				Webhook: WebhookConfig{
					URL:    "https://example.com/hook",
					Method: "POST",
				},
			},
			wantErr: true,
		},
		{
			name: "negative interval",
			req: CreateRecurringRequest{
				StartAt:    time.Now().Add(time.Hour),
				IntervalMs: -1000,
				Webhook: WebhookConfig{
					URL:    "https://example.com/hook",
					Method: "POST",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate(defaultTimeout)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewOneShotJob(t *testing.T) {
	req := &CreateOneShotRequest{
		ExecuteAt: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
		Webhook: WebhookConfig{
			URL:       "https://example.com/hook",
			Method:    "POST",
			Headers:   map[string]string{"X-Custom": "value"},
			Body:      map[string]any{"key": "value"},
			TimeoutMs: 10000,
		},
		Retry: &RetryConfig{
			MaxAttempts:   5,
			BackoffBaseMs: 2000,
		},
	}

	job := NewOneShotJob("test-id", req, 5000)

	if job.ID != "test-id" {
		t.Errorf("ID = %v, want test-id", job.ID)
	}
	if job.Type != JobTypeOneShot {
		t.Errorf("Type = %v, want %v", job.Type, JobTypeOneShot)
	}
	if job.DueAtMs != req.ExecuteAt.UnixMilli() {
		t.Errorf("DueAtMs = %v, want %v", job.DueAtMs, req.ExecuteAt.UnixMilli())
	}
	if job.MaxAttempts != 5 {
		t.Errorf("MaxAttempts = %v, want 5", job.MaxAttempts)
	}
	if job.BackoffBaseMs != 2000 {
		t.Errorf("BackoffBaseMs = %v, want 2000", job.BackoffBaseMs)
	}
	if job.TimeoutMs != 10000 {
		t.Errorf("TimeoutMs = %v, want 10000", job.TimeoutMs)
	}
}

func TestNewRecurringJob(t *testing.T) {
	req := &CreateRecurringRequest{
		StartAt:    time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
		IntervalMs: 3600000, // 1 hour
		Webhook: WebhookConfig{
			URL:    "https://example.com/hook",
			Method: "POST",
		},
	}

	job := NewRecurringJob("test-id", req, 5000)

	if job.ID != "test-id" {
		t.Errorf("ID = %v, want test-id", job.ID)
	}
	if job.Type != JobTypeRecurring {
		t.Errorf("Type = %v, want %v", job.Type, JobTypeRecurring)
	}
	if job.IntervalMs != 3600000 {
		t.Errorf("IntervalMs = %v, want 3600000", job.IntervalMs)
	}
}
