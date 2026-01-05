package config

import (
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: &Config{
				RedisURL:         "redis://localhost:6379",
				AuthToken:        "secret",
				Namespace:        "tickhook",
				Bind:             "0.0.0.0:8080",
				PollMs:           200,
				Batch:            200,
				MaxInflight:      200,
				MaxPerDomain:     5,
				DefaultTimeoutMs: 5000,
				LogLevel:         "info",
			},
			wantErr: false,
		},
		{
			name: "missing redis URL",
			cfg: &Config{
				AuthToken:        "secret",
				Namespace:        "tickhook",
				Bind:             "0.0.0.0:8080",
				PollMs:           200,
				Batch:            200,
				MaxInflight:      200,
				MaxPerDomain:     5,
				DefaultTimeoutMs: 5000,
				LogLevel:         "info",
			},
			wantErr: true,
		},
		{
			name: "invalid redis URL scheme",
			cfg: &Config{
				RedisURL:         "http://localhost:6379",
				AuthToken:        "secret",
				Namespace:        "tickhook",
				Bind:             "0.0.0.0:8080",
				PollMs:           200,
				Batch:            200,
				MaxInflight:      200,
				MaxPerDomain:     5,
				DefaultTimeoutMs: 5000,
				LogLevel:         "info",
			},
			wantErr: true,
		},
		{
			name: "missing auth token",
			cfg: &Config{
				RedisURL:         "redis://localhost:6379",
				AuthToken:        "",
				Namespace:        "tickhook",
				Bind:             "0.0.0.0:8080",
				PollMs:           200,
				Batch:            200,
				MaxInflight:      200,
				MaxPerDomain:     5,
				DefaultTimeoutMs: 5000,
				LogLevel:         "info",
			},
			wantErr: true,
		},
		{
			name: "invalid poll_ms",
			cfg: &Config{
				RedisURL:         "redis://localhost:6379",
				AuthToken:        "secret",
				Namespace:        "tickhook",
				Bind:             "0.0.0.0:8080",
				PollMs:           0,
				Batch:            200,
				MaxInflight:      200,
				MaxPerDomain:     5,
				DefaultTimeoutMs: 5000,
				LogLevel:         "info",
			},
			wantErr: true,
		},
		{
			name: "invalid log level",
			cfg: &Config{
				RedisURL:         "redis://localhost:6379",
				AuthToken:        "secret",
				Namespace:        "tickhook",
				Bind:             "0.0.0.0:8080",
				PollMs:           200,
				Batch:            200,
				MaxInflight:      200,
				MaxPerDomain:     5,
				DefaultTimeoutMs: 5000,
				LogLevel:         "invalid",
			},
			wantErr: true,
		},
		{
			name: "rediss scheme (TLS)",
			cfg: &Config{
				RedisURL:         "rediss://localhost:6379",
				AuthToken:        "secret",
				Namespace:        "tickhook",
				Bind:             "0.0.0.0:8080",
				PollMs:           200,
				Batch:            200,
				MaxInflight:      200,
				MaxPerDomain:     5,
				DefaultTimeoutMs: 5000,
				LogLevel:         "info",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Namespace != "tickhook" {
		t.Errorf("Namespace = %v, want tickhook", cfg.Namespace)
	}
	if cfg.Bind != "0.0.0.0:8080" {
		t.Errorf("Bind = %v, want 0.0.0.0:8080", cfg.Bind)
	}
	if cfg.PollMs != 200 {
		t.Errorf("PollMs = %v, want 200", cfg.PollMs)
	}
	if cfg.Batch != 200 {
		t.Errorf("Batch = %v, want 200", cfg.Batch)
	}
	if cfg.MaxInflight != 200 {
		t.Errorf("MaxInflight = %v, want 200", cfg.MaxInflight)
	}
	if cfg.MaxPerDomain != 5 {
		t.Errorf("MaxPerDomain = %v, want 5", cfg.MaxPerDomain)
	}
	if cfg.DefaultTimeoutMs != 5000 {
		t.Errorf("DefaultTimeoutMs = %v, want 5000", cfg.DefaultTimeoutMs)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %v, want info", cfg.LogLevel)
	}
}
