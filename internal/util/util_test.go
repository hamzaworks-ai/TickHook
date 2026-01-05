package util

import (
	"testing"
)

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
		wantErr  bool
	}{
		{
			name:     "simple http URL",
			url:      "http://example.com/path",
			expected: "example.com",
		},
		{
			name:     "https URL",
			url:      "https://example.com/path",
			expected: "example.com",
		},
		{
			name:     "URL with port",
			url:      "https://example.com:8080/path",
			expected: "example.com",
		},
		{
			name:     "URL with subdomain",
			url:      "https://api.example.com/v1/webhook",
			expected: "api.example.com",
		},
		{
			name:     "uppercase domain",
			url:      "https://EXAMPLE.COM/path",
			expected: "example.com",
		},
		{
			name:     "mixed case domain",
			url:      "https://Api.Example.Com/path",
			expected: "api.example.com",
		},
		{
			name:     "localhost",
			url:      "http://localhost:8080/hook",
			expected: "localhost",
		},
		{
			name:     "IP address",
			url:      "http://192.168.1.1:3000/webhook",
			expected: "192.168.1.1",
		},
		{
			name:    "invalid URL",
			url:     "not-a-url",
			wantErr: false, // url.Parse doesn't error on this, but host will be empty
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractDomain(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractDomain() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("ExtractDomain() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCalculateBackoff(t *testing.T) {
	tests := []struct {
		name          string
		attempt       int
		backoffBaseMs int64
		jitterMaxMs   int
	}{
		{
			name:          "first attempt",
			attempt:       1,
			backoffBaseMs: 1000,
			jitterMaxMs:   0,
		},
		{
			name:          "second attempt",
			attempt:       2,
			backoffBaseMs: 1000,
			jitterMaxMs:   0,
		},
		{
			name:          "third attempt",
			attempt:       3,
			backoffBaseMs: 1000,
			jitterMaxMs:   0,
		},
		{
			name:          "with jitter",
			attempt:       1,
			backoffBaseMs: 1000,
			jitterMaxMs:   250,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateBackoff(tt.attempt, tt.backoffBaseMs, tt.jitterMaxMs)

			// Calculate expected base (without jitter)
			expectedBase := tt.backoffBaseMs
			for i := 1; i < tt.attempt; i++ {
				expectedBase *= 2
			}
			if expectedBase > MaxBackoffMs {
				expectedBase = MaxBackoffMs
			}

			// Result should be between expectedBase and expectedBase + jitterMaxMs
			if got < expectedBase {
				t.Errorf("CalculateBackoff() = %v, want >= %v", got, expectedBase)
			}
			if got > expectedBase+int64(tt.jitterMaxMs) {
				t.Errorf("CalculateBackoff() = %v, want <= %v", got, expectedBase+int64(tt.jitterMaxMs))
			}
		})
	}
}

func TestCalculateBackoff_Cap(t *testing.T) {
	// Test that backoff is capped at MaxBackoffMs
	got := CalculateBackoff(100, 1000, 0) // Very high attempt number
	if got > MaxBackoffMs {
		t.Errorf("CalculateBackoff() = %v, want <= MaxBackoffMs (%v)", got, MaxBackoffMs)
	}
}

func TestCalculateBackoff_ZeroAttempt(t *testing.T) {
	// Test that attempt 0 is treated as attempt 1
	got := CalculateBackoff(0, 1000, 0)
	expected := int64(1000) // Should be base * 2^0 = base
	if got != expected {
		t.Errorf("CalculateBackoff(0) = %v, want %v", got, expected)
	}
}

func TestIsRetryableHTTPStatus(t *testing.T) {
	tests := []struct {
		status   int
		expected bool
	}{
		{200, false},
		{201, false},
		{400, false},
		{401, false},
		{403, false},
		{404, false},
		{429, true},  // Too Many Requests - retryable
		{500, true},  // Internal Server Error
		{502, true},  // Bad Gateway
		{503, true},  // Service Unavailable
		{504, true},  // Gateway Timeout
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := IsRetryableHTTPStatus(tt.status)
			if got != tt.expected {
				t.Errorf("IsRetryableHTTPStatus(%d) = %v, want %v", tt.status, got, tt.expected)
			}
		})
	}
}

func TestIsSuccessHTTPStatus(t *testing.T) {
	tests := []struct {
		status   int
		expected bool
	}{
		{199, false},
		{200, true},
		{201, true},
		{204, true},
		{299, true},
		{300, false},
		{400, false},
		{500, false},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := IsSuccessHTTPStatus(tt.status)
			if got != tt.expected {
				t.Errorf("IsSuccessHTTPStatus(%d) = %v, want %v", tt.status, got, tt.expected)
			}
		})
	}
}

func TestIsClientErrorHTTPStatus(t *testing.T) {
	tests := []struct {
		status   int
		expected bool
	}{
		{200, false},
		{399, false},
		{400, true},
		{401, true},
		{403, true},
		{404, true},
		{429, false}, // 429 is retryable, not a final client error
		{499, true},
		{500, false},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := IsClientErrorHTTPStatus(tt.status)
			if got != tt.expected {
				t.Errorf("IsClientErrorHTTPStatus(%d) = %v, want %v", tt.status, got, tt.expected)
			}
		})
	}
}
