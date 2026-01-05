// Package util provides helper functions for TickHook.
// PRD Reference: Various utility needs across the application
package util

import (
	"math"
	"math/rand"
	"net/url"
	"strings"
	"time"
)

const (
	// MaxBackoffMs is the maximum backoff delay (10 minutes).
	MaxBackoffMs = 10 * 60 * 1000
	// DefaultJitterMaxMs is the default maximum jitter (250ms).
	DefaultJitterMaxMs = 250
)

// ExtractDomain extracts and normalizes the domain from a URL.
// Returns lowercase host without port.
func ExtractDomain(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	host := parsed.Hostname() // This strips the port
	return strings.ToLower(host), nil
}

// CalculateBackoff calculates the backoff delay with exponential backoff and jitter.
// attempt is 1-indexed (first retry is attempt 1).
// Returns the delay in milliseconds.
func CalculateBackoff(attempt int, backoffBaseMs int64, jitterMaxMs int) int64 {
	if attempt < 1 {
		attempt = 1
	}

	// Exponential backoff: base * 2^(attempt-1)
	exponential := float64(backoffBaseMs) * math.Pow(2, float64(attempt-1))

	// Cap to maximum
	if exponential > float64(MaxBackoffMs) {
		exponential = float64(MaxBackoffMs)
	}

	// Add jitter (0 to jitterMaxMs)
	jitter := rand.Int63n(int64(jitterMaxMs) + 1)

	return int64(exponential) + jitter
}

// NowMs returns the current UTC time in milliseconds.
func NowMs() int64 {
	return time.Now().UTC().UnixMilli()
}

// MsToTime converts milliseconds to time.Time.
func MsToTime(ms int64) time.Time {
	return time.UnixMilli(ms).UTC()
}

// TimeToMs converts time.Time to milliseconds.
func TimeToMs(t time.Time) int64 {
	return t.UnixMilli()
}

// IsRetryableHTTPStatus returns true if the HTTP status code is retryable.
// Retryable: 5xx server errors and 429 (Too Many Requests).
func IsRetryableHTTPStatus(statusCode int) bool {
	return statusCode == 429 || (statusCode >= 500 && statusCode <= 599)
}

// IsSuccessHTTPStatus returns true if the HTTP status code indicates success.
func IsSuccessHTTPStatus(statusCode int) bool {
	return statusCode >= 200 && statusCode <= 299
}

// IsClientErrorHTTPStatus returns true if the HTTP status code is a non-retryable client error.
// Client errors (4xx except 429) are non-retryable.
func IsClientErrorHTTPStatus(statusCode int) bool {
	return statusCode >= 400 && statusCode <= 499 && statusCode != 429
}
