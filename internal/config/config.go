// Package config handles CLI flags parsing and configuration validation for TickHook.
// PRD Reference: Section 15 - CLI Interface
package config

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
)

// Config holds all configuration options for TickHook.
type Config struct {
	// Required flags
	RedisURL  string
	AuthToken string

	// Optional flags with defaults
	Namespace        string
	Bind             string
	PollMs           int
	Batch            int
	MaxInflight      int
	MaxPerDomain     int
	DefaultTimeoutMs int
	LogLevel         string
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() *Config {
	return &Config{
		Namespace:        "tickhook",
		Bind:             "0.0.0.0:8080",
		PollMs:           200,
		Batch:            200,
		MaxInflight:      200,
		MaxPerDomain:     5,
		DefaultTimeoutMs: 5000,
		LogLevel:         "info",
	}
}

// Parse parses command-line flags and returns a validated Config.
func Parse() (*Config, error) {
	cfg := DefaultConfig()

	flag.StringVar(&cfg.RedisURL, "redis-url", "", "Redis connection URL (required, e.g., redis://localhost:6379)")
	flag.StringVar(&cfg.AuthToken, "auth-token", "", "Bearer token for API authentication (required)")
	flag.StringVar(&cfg.Namespace, "namespace", cfg.Namespace, "Redis key namespace")
	flag.StringVar(&cfg.Bind, "bind", cfg.Bind, "HTTP server bind address")
	flag.IntVar(&cfg.PollMs, "poll-ms", cfg.PollMs, "Scheduler poll interval in milliseconds")
	flag.IntVar(&cfg.Batch, "batch", cfg.Batch, "Maximum jobs to fetch per poll")
	flag.IntVar(&cfg.MaxInflight, "max-inflight", cfg.MaxInflight, "Maximum concurrent webhook executions (global)")
	flag.IntVar(&cfg.MaxPerDomain, "max-per-domain", cfg.MaxPerDomain, "Maximum concurrent executions per domain")
	flag.IntVar(&cfg.DefaultTimeoutMs, "default-timeout-ms", cfg.DefaultTimeoutMs, "Default webhook timeout in milliseconds")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Log level (debug, info, warn, error)")

	flag.Parse()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks that all required configuration is present and valid.
func (c *Config) Validate() error {
	if c.RedisURL == "" {
		return errors.New("--redis-url is required")
	}

	// Validate Redis URL format
	parsedURL, err := url.Parse(c.RedisURL)
	if err != nil {
		return fmt.Errorf("invalid --redis-url: %w", err)
	}
	if parsedURL.Scheme != "redis" && parsedURL.Scheme != "rediss" {
		return fmt.Errorf("invalid --redis-url scheme: expected redis:// or rediss://, got %s://", parsedURL.Scheme)
	}

	if c.AuthToken == "" {
		return errors.New("--auth-token is required")
	}

	if c.PollMs <= 0 {
		return errors.New("--poll-ms must be positive")
	}

	if c.Batch <= 0 {
		return errors.New("--batch must be positive")
	}

	if c.MaxInflight <= 0 {
		return errors.New("--max-inflight must be positive")
	}

	if c.MaxPerDomain <= 0 {
		return errors.New("--max-per-domain must be positive")
	}

	if c.DefaultTimeoutMs <= 0 {
		return errors.New("--default-timeout-ms must be positive")
	}

	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[c.LogLevel] {
		return fmt.Errorf("invalid --log-level: %s (must be debug, info, warn, or error)", c.LogLevel)
	}

	return nil
}

// PrintStartup prints configuration summary on startup.
func (c *Config) PrintStartup(logger interface{ Info(msg string, args ...any) }) {
	logger.Info("TickHook starting",
		"bind", c.Bind,
		"namespace", c.Namespace,
		"poll_ms", c.PollMs,
		"batch", c.Batch,
		"max_inflight", c.MaxInflight,
		"max_per_domain", c.MaxPerDomain,
		"default_timeout_ms", c.DefaultTimeoutMs,
		"log_level", c.LogLevel,
	)
}

// Usage prints usage information.
func Usage() {
	fmt.Fprintf(os.Stderr, "Usage: tickhook [options]\n\n")
	fmt.Fprintf(os.Stderr, "TickHook is a lightweight webhook scheduler.\n\n")
	fmt.Fprintf(os.Stderr, "Options:\n")
	flag.PrintDefaults()
}
