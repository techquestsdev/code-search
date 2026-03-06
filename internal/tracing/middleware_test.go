package tracing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPMiddleware(t *testing.T) {
	t.Run("successful request", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		wrapped := HTTPMiddleware(handler)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/repos", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		if rec.Body.String() != "OK" {
			t.Errorf("expected body 'OK', got %q", rec.Body.String())
		}
	})

	t.Run("404 request", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Not Found"))
		})

		wrapped := HTTPMiddleware(handler)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/notfound", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", rec.Code)
		}
	})

	t.Run("500 request", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})

		wrapped := HTTPMiddleware(handler)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/repos", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", rec.Code)
		}
	})

	t.Run("POST request", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST method, got %s", r.Method)
			}

			w.WriteHeader(http.StatusCreated)
		})

		wrapped := HTTPMiddleware(handler)

		req := httptest.NewRequest(http.MethodPost, "/api/connections", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("expected status 201, got %d", rec.Code)
		}
	})

	t.Run("with trace context headers", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		wrapped := HTTPMiddleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/search", nil)
		// Add W3C trace context headers
		req.Header.Set("Traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")

		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("preserves context", func(t *testing.T) {
		var contextPassed bool

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify that context has a span
			span := SpanFromContext(r.Context())
			if span != nil {
				contextPassed = true
			}

			w.WriteHeader(http.StatusOK)
		})

		wrapped := HTTPMiddleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		if !contextPassed {
			t.Error("expected context to have span")
		}
	})
}

func TestResponseWriter(t *testing.T) {
	t.Run("captures status code", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

		rw.WriteHeader(http.StatusCreated)

		if rw.statusCode != http.StatusCreated {
			t.Errorf("expected status 201, got %d", rw.statusCode)
		}
	})

	t.Run("default status code", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

		if rw.statusCode != http.StatusOK {
			t.Errorf("expected default status 200, got %d", rw.statusCode)
		}
	})

	t.Run("write without WriteHeader", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

		rw.Write([]byte("test"))

		// Status code should still be 200 (default)
		if rw.statusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", rw.statusCode)
		}
	})
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/", "/"},
		{"/api", "/api"},
		{"/api/repos", "/api/repos"},
		{"/api/repos/123", "/api/repos/:id"},
		{"/api/repos/456/branches", "/api/repos/:id/branches"},
		{"/api/connections/789", "/api/connections/:id"},
		{"/api/jobs/999/status", "/api/jobs/:id/status"},
		{"/health", "/health"},
		{"/metrics", "/metrics"},
		{"/api/search", "/api/search"},
		{"/api/v1/repos/123/commits/456", "/api/v1/repos/:id/commits/:id"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizePath(tt.input)
			if result != tt.expected {
				t.Errorf("normalizePath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"0", true},
		{"1", true},
		{"123", true},
		{"999999999", true},
		{"", false},
		{"abc", false},
		{"123abc", false},
		{"abc123", false},
		{"-1", false},
		{"+1", false},
		{"1.5", false},
		{"1e10", false},
		{" 123", false},
		{"123 ", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isNumeric(tt.input)
			if result != tt.expected {
				t.Errorf("isNumeric(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func BenchmarkHTTPMiddleware(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := HTTPMiddleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/repos/123", nil)

	for b.Loop() {
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}
}

func BenchmarkNormalizePath(b *testing.B) {
	paths := []string{
		"/api/repos",
		"/api/repos/123",
		"/api/connections/456/repos/789",
		"/health",
	}

	for b.Loop() {
		for _, p := range paths {
			normalizePath(p)
		}
	}
}
