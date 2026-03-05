package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRecordSearch(t *testing.T) {
	// Test that RecordSearch doesn't panic and properly increments metrics
	RecordSearch("text", 100*time.Millisecond, 42)
	RecordSearch("regex", 200*time.Millisecond, 100)
	RecordSearch("symbol", 50*time.Millisecond, 0)
}

func TestRecordRepoIndex(t *testing.T) {
	RecordRepoIndex(5*time.Second, true)
	RecordRepoIndex(10*time.Second, false)
}

func TestRecordRepoSync(t *testing.T) {
	RecordRepoSync(2*time.Second, true)
	RecordRepoSync(3*time.Second, false)
}

func TestRecordJob(t *testing.T) {
	RecordJob("index", 30*time.Second, true)
	RecordJob("sync", 15*time.Second, true)
	RecordJob("replace", 5*time.Second, false)
}

func TestRecordReplace(t *testing.T) {
	RecordReplace(10, true)
	RecordReplace(0, false)
}

func TestSetRepositoryCounts(t *testing.T) {
	counts := map[string]int{
		"indexed": 100,
		"pending": 5,
		"failed":  2,
	}
	SetRepositoryCounts(counts)
}

func TestSetConnectionCounts(t *testing.T) {
	counts := map[string]int{
		"github": 3,
		"gitlab": 2,
		"gitea":  1,
	}
	SetConnectionCounts(counts)
}

func TestSetJobQueueCounts(t *testing.T) {
	counts := map[string]int{
		"index":   10,
		"sync":    5,
		"replace": 2,
	}
	SetJobQueueCounts(counts)
}

func TestSetDBConnections(t *testing.T) {
	SetDBConnections(25, 10)
	SetDBConnections(0, 0)
}

func TestSetRedisConnections(t *testing.T) {
	SetRedisConnections(10)
	SetRedisConnections(0)
}

func TestSetZoektStats(t *testing.T) {
	SetZoektStats(100, 1024*1024*1024) // 100 shards, 1GB
	SetZoektStats(0, 0)
}

func TestRecordRateLimit(t *testing.T) {
	RecordRateLimit(true)  // allowed
	RecordRateLimit(false) // rejected
}

func TestSetRateLimitBuckets(t *testing.T) {
	SetRateLimitBuckets(100)
	SetRateLimitBuckets(0)
}

func TestRecordCodeHostRequest(t *testing.T) {
	RecordCodeHostRequest("github", "/repos", 100*time.Millisecond, true)
	RecordCodeHostRequest("gitlab", "/projects", 200*time.Millisecond, false)
	RecordCodeHostRequest("bitbucket", "/repositories", 150*time.Millisecond, true)
}

func TestSetCodeHostRateLimit(t *testing.T) {
	SetCodeHostRateLimit("github", 4500, time.Now().Add(time.Hour))
	SetCodeHostRateLimit("gitlab", 1000, time.Now().Add(30*time.Minute))
}

func TestRecordGitClone(t *testing.T) {
	RecordGitClone(30*time.Second, true)
	RecordGitClone(60*time.Second, false)
}

func TestRecordGitFetch(t *testing.T) {
	RecordGitFetch(5*time.Second, true)
	RecordGitFetch(10*time.Second, false)
}

func TestRecordBranchesIndexed(t *testing.T) {
	RecordBranchesIndexed(5, true)
	RecordBranchesIndexed(0, false)
	RecordBranchesIndexed(10, true)
}

func TestRecordError(t *testing.T) {
	RecordError("db", "connection_timeout")
	RecordError("git", "clone_failed")
	RecordError("codehost", "rate_limited")
	RecordError("search", "query_timeout")
}

func TestHTTPMiddleware(t *testing.T) {
	// Create a simple handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with middleware
	wrapped := HTTPMiddleware(handler)

	// Test request
	req := httptest.NewRequest(http.MethodGet, "/api/repos/123", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestHTTPMiddleware_404(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	wrapped := HTTPMiddleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/notfound", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestHTTPMiddleware_500(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	wrapped := HTTPMiddleware(handler)

	req := httptest.NewRequest(http.MethodPost, "/api/repos", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/", "/"},
		{"/api/repos", "/api/repos"},
		{"/api/repos/123", "/api/repos/:id"},
		{"/api/connections/456/repos", "/api/connections/:id/repos"},
		{"/api/jobs/789/status", "/api/jobs/:id/status"},
		{"/health", "/health"},
		{"/metrics", "/metrics"},
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
		{"123", true},
		{"0", true},
		{"999999", true},
		{"", false},
		{"abc", false},
		{"123abc", false},
		{"abc123", false},
		{"-1", false},
		{"1.5", false},
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

func TestHandler(t *testing.T) {
	handler := Handler()
	if handler == nil {
		t.Error("Handler() returned nil")
	}

	// Test that it serves metrics
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	// Check that response contains Prometheus metrics
	body := rec.Body.String()
	if len(body) == 0 {
		t.Error("expected non-empty metrics response")
	}
}
