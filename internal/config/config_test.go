package config

import (
	"os"
	"testing"
	"time"
)

func TestCodeHost_ResolveToken(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		envKey   string
		envValue string
		expected string
	}{
		{
			name:     "literal token",
			token:    "ghp_xxxxxxxxxxxx",
			expected: "ghp_xxxxxxxxxxxx",
		},
		{
			name:     "env var reference",
			token:    "$TEST_TOKEN",
			envKey:   "TEST_TOKEN",
			envValue: "secret_from_env",
			expected: "secret_from_env",
		},
		{
			name:     "env var reference not set",
			token:    "$MISSING_TOKEN",
			expected: "",
		},
		{
			name:     "empty token",
			token:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envKey != "" {
				os.Setenv(tt.envKey, tt.envValue)
				defer os.Unsetenv(tt.envKey)
			}

			ch := &CodeHost{Token: tt.token}
			result := ch.ResolveToken()

			if result != tt.expected {
				t.Errorf("ResolveToken() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestConfig_GetCodeHost(t *testing.T) {
	cfg := &Config{
		CodeHosts: map[string]CodeHost{
			"github": {Type: "github", Token: "token1"},
			"gitlab": {Type: "gitlab", Token: "token2"},
		},
	}

	tests := []struct {
		name      string
		hostName  string
		wantFound bool
		wantType  string
	}{
		{"existing github", "github", true, "github"},
		{"existing gitlab", "gitlab", true, "gitlab"},
		{"non-existing", "bitbucket", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch, found := cfg.GetCodeHost(tt.hostName)

			if found != tt.wantFound {
				t.Errorf("GetCodeHost() found = %v, want %v", found, tt.wantFound)
			}

			if found && ch.Type != tt.wantType {
				t.Errorf("GetCodeHost() type = %v, want %v", ch.Type, tt.wantType)
			}
		})
	}

	nilCfg := &Config{}

	_, found := nilCfg.GetCodeHost("github")
	if found {
		t.Error("GetCodeHost() should return false for nil CodeHosts")
	}
}

func TestDatabaseConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  DatabaseConfig
	}{
		{
			name: "postgres url",
			cfg: DatabaseConfig{
				Driver:          "postgres",
				URL:             "postgres://user:pass@localhost/db",
				MaxOpenConns:    25,
				MaxIdleConns:    5,
				ConnMaxLifetime: 5 * time.Minute,
			},
		},
		{
			name: "mysql url",
			cfg: DatabaseConfig{
				Driver:          "mysql",
				URL:             "user:pass@tcp(localhost:3306)/db",
				MaxOpenConns:    10,
				MaxIdleConns:    2,
				ConnMaxLifetime: 10 * time.Minute,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cfg.URL == "" {
				t.Error("URL should not be empty")
			}
		})
	}
}

func TestTracingConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  TracingConfig
	}{
		{
			name: "disabled tracing",
			cfg: TracingConfig{
				Enabled: false,
			},
		},
		{
			name: "enabled with grpc",
			cfg: TracingConfig{
				Enabled:     true,
				ServiceName: "test-service",
				Endpoint:    "localhost:4317",
				Protocol:    "grpc",
				SampleRate:  1.0,
				Insecure:    true,
			},
		},
		{
			name: "enabled with http",
			cfg: TracingConfig{
				Enabled:     true,
				ServiceName: "test-service",
				Endpoint:    "localhost:4318",
				Protocol:    "http",
				SampleRate:  0.5,
				Insecure:    false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cfg.Enabled && tt.cfg.ServiceName == "" {
				t.Error("ServiceName should be set when tracing is enabled")
			}

			if tt.cfg.SampleRate < 0 || tt.cfg.SampleRate > 1 {
				t.Errorf("SampleRate %v should be between 0 and 1", tt.cfg.SampleRate)
			}
		})
	}
}

func TestSchedulerConfig(t *testing.T) {
	cfg := SchedulerConfig{
		Enabled:             true,
		PollInterval:        6 * time.Hour,
		CheckInterval:       5 * time.Minute,
		StaleThreshold:      24 * time.Hour,
		MaxConcurrentChecks: 5,
		JobRetention:        1 * time.Hour,
	}

	if !cfg.Enabled {
		t.Error("Scheduler should be enabled")
	}

	if cfg.PollInterval < cfg.CheckInterval {
		t.Error("PollInterval should be >= CheckInterval")
	}

	if cfg.StaleThreshold < cfg.PollInterval {
		t.Error("StaleThreshold should be >= PollInterval")
	}
}

func TestReplaceConfig(t *testing.T) {
	cfg := ReplaceConfig{
		Concurrency:  3,
		CloneTimeout: 10 * time.Minute,
		PushTimeout:  5 * time.Minute,
		MaxFileSize:  10 * 1024 * 1024,
	}

	if cfg.Concurrency <= 0 {
		t.Error("Concurrency should be positive")
	}

	if cfg.CloneTimeout < cfg.PushTimeout {
		t.Error("CloneTimeout should typically be >= PushTimeout")
	}

	if cfg.MaxFileSize <= 0 {
		t.Error("MaxFileSize should be positive")
	}
}

func TestRateLimitConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  RateLimitConfig
	}{
		{
			name: "disabled",
			cfg: RateLimitConfig{
				Enabled: false,
			},
		},
		{
			name: "enabled",
			cfg: RateLimitConfig{
				Enabled:           true,
				RequestsPerSecond: 10.0,
				BurstSize:         20,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cfg.Enabled {
				if tt.cfg.RequestsPerSecond <= 0 {
					t.Error("RequestsPerSecond should be positive when enabled")
				}

				if tt.cfg.BurstSize <= 0 {
					t.Error("BurstSize should be positive when enabled")
				}
			}
		})
	}
}
