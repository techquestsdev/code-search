package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig should not return nil")
	}

	if cfg.RequestsPerSecond != 100 {
		t.Errorf("RequestsPerSecond = %v, want 100", cfg.RequestsPerSecond)
	}

	if cfg.BurstSize != 200 {
		t.Errorf("BurstSize = %v, want 200", cfg.BurstSize)
	}

	if cfg.CleanupInterval != time.Minute {
		t.Errorf("CleanupInterval = %v, want 1m", cfg.CleanupInterval)
	}

	if !cfg.Enabled {
		t.Error("Enabled should default to true")
	}
}

func TestNewLimiter(t *testing.T) {
	t.Run("with config", func(t *testing.T) {
		cfg := &Config{
			RequestsPerSecond: 10,
			BurstSize:         20,
			CleanupInterval:   time.Second,
			Enabled:           true,
		}

		limiter := NewLimiter(cfg)
		defer limiter.Stop()

		if limiter == nil {
			t.Fatal("NewLimiter should not return nil")
		}

		if limiter.config.RequestsPerSecond != 10 {
			t.Errorf("config.RequestsPerSecond = %v, want 10", limiter.config.RequestsPerSecond)
		}
	})

	t.Run("with nil config", func(t *testing.T) {
		limiter := NewLimiter(nil)
		defer limiter.Stop()

		if limiter == nil {
			t.Fatal("NewLimiter should not return nil")
		}

		if limiter.config.RequestsPerSecond != 100 {
			t.Errorf(
				"Should use default config, got RequestsPerSecond = %v",
				limiter.config.RequestsPerSecond,
			)
		}
	})
}

func TestLimiter_Allow(t *testing.T) {
	t.Run("allows requests up to burst", func(t *testing.T) {
		cfg := &Config{
			RequestsPerSecond: 10,
			BurstSize:         5,
			CleanupInterval:   time.Minute,
			Enabled:           true,
		}

		limiter := NewLimiter(cfg)
		defer limiter.Stop()

		for i := range 5 {
			if !limiter.Allow("test-ip") {
				t.Errorf("Request %d should be allowed", i+1)
			}
		}

		if limiter.Allow("test-ip") {
			t.Error("Request beyond burst should be rejected")
		}
	})

	t.Run("disabled limiter allows all", func(t *testing.T) {
		cfg := &Config{
			RequestsPerSecond: 1,
			BurstSize:         1,
			CleanupInterval:   time.Minute,
			Enabled:           false,
		}

		limiter := NewLimiter(cfg)
		defer limiter.Stop()

		for range 100 {
			if !limiter.Allow("test-ip") {
				t.Errorf("Disabled limiter should allow all requests")
			}
		}
	})

	t.Run("different keys have separate buckets", func(t *testing.T) {
		cfg := &Config{
			RequestsPerSecond: 10,
			BurstSize:         2,
			CleanupInterval:   time.Minute,
			Enabled:           true,
		}

		limiter := NewLimiter(cfg)
		defer limiter.Stop()

		limiter.Allow("ip1")
		limiter.Allow("ip1")

		if !limiter.Allow("ip2") {
			t.Error("Different IP should have its own bucket")
		}
	})
}

func TestLimiter_RemainingTokens(t *testing.T) {
	cfg := &Config{
		RequestsPerSecond: 10,
		BurstSize:         10,
		CleanupInterval:   time.Minute,
		Enabled:           true,
	}

	limiter := NewLimiter(cfg)
	defer limiter.Stop()

	remaining := limiter.RemainingTokens("new-ip")
	if remaining != 10 {
		t.Errorf("RemainingTokens = %v, want 10", remaining)
	}

	limiter.Allow("used-ip")
	limiter.Allow("used-ip")
	limiter.Allow("used-ip")

	remaining = limiter.RemainingTokens("used-ip")
	if remaining > 7 {
		t.Errorf("RemainingTokens after 3 requests = %v, should be <= 7", remaining)
	}
}

func TestLimiter_ResetAfter(t *testing.T) {
	cfg := &Config{
		RequestsPerSecond: 10,
		BurstSize:         1,
		CleanupInterval:   time.Minute,
		Enabled:           true,
	}

	limiter := NewLimiter(cfg)
	defer limiter.Stop()

	limiter.Allow("test-ip")

	resetAfter := limiter.ResetAfter("test-ip")
	if resetAfter == 0 {
		t.Error("ResetAfter should be > 0 when bucket is exhausted")
	}

	if resetAfter > time.Second {
		t.Errorf("ResetAfter = %v, seems too high", resetAfter)
	}
}

func TestLimiter_Middleware(t *testing.T) {
	cfg := &Config{
		RequestsPerSecond: 10,
		BurstSize:         2,
		CleanupInterval:   time.Minute,
		Enabled:           true,
	}

	limiter := NewLimiter(cfg)
	defer limiter.Stop()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	wrapped := limiter.Middleware(handler)

	t.Run("allowed requests pass through", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"

		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", rr.Code, http.StatusOK)
		}

		if rr.Header().Get("X-Ratelimit-Limit") == "" {
			t.Error("Should set X-RateLimit-Limit header")
		}

		if rr.Header().Get("X-Ratelimit-Remaining") == "" {
			t.Error("Should set X-RateLimit-Remaining header")
		}
	})

	t.Run("rate limited requests return 429", func(t *testing.T) {
		for i := range 10 {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = "10.0.0.1:12345"

			rr := httptest.NewRecorder()
			wrapped.ServeHTTP(rr, req)

			if i >= 2 && rr.Code != http.StatusTooManyRequests {
				t.Errorf(
					"Request %d: Status = %v, want %v",
					i+1,
					rr.Code,
					http.StatusTooManyRequests,
				)
			}
		}
	})
}

func TestLimiter_Middleware_Disabled(t *testing.T) {
	cfg := &Config{
		RequestsPerSecond: 1,
		BurstSize:         1,
		CleanupInterval:   time.Minute,
		Enabled:           false,
	}

	limiter := NewLimiter(cfg)
	defer limiter.Stop()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := limiter.Middleware(handler)

	for range 10 {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "10.0.0.1:12345"

		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Disabled limiter should allow all requests, got status %v", rr.Code)
		}
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		expected   string
	}{
		{
			name:       "from RemoteAddr with port",
			remoteAddr: "192.168.1.1:12345",
			headers:    nil,
			expected:   "192.168.1.1",
		},
		{
			name:       "from X-Forwarded-For single",
			remoteAddr: "10.0.0.1:12345",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.50"},
			expected:   "203.0.113.50",
		},
		{
			name:       "from X-Forwarded-For multiple",
			remoteAddr: "10.0.0.1:12345",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.50, 70.41.3.18"},
			expected:   "203.0.113.50",
		},
		{
			name:       "from X-Real-IP",
			remoteAddr: "10.0.0.1:12345",
			headers:    map[string]string{"X-Real-IP": "203.0.113.100"},
			expected:   "203.0.113.100",
		},
		{
			name:       "X-Forwarded-For takes precedence",
			remoteAddr: "10.0.0.1:12345",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.50",
				"X-Real-IP":       "203.0.113.100",
			},
			expected: "203.0.113.50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = tt.remoteAddr

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			result := getClientIP(req)
			if result != tt.expected {
				t.Errorf("getClientIP() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMiddlewareFunc(t *testing.T) {
	middleware := MiddlewareFunc(10, 5)

	if middleware == nil {
		t.Error("MiddlewareFunc should not return nil")
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := middleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Status = %v, want %v", rr.Code, http.StatusOK)
	}
}
