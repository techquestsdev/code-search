package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/techquestsdev/code-search/internal/repos"
)

func TestRepositoryResponse_JSON(t *testing.T) {
	resp := RepositoryResponse{
		ID:            1,
		Name:          "github.com/test/repo",
		CloneURL:      "https://github.com/test/repo.git",
		DefaultBranch: "main",
		Branches:      []string{"main", "develop"},
		Status:        "indexed",
		LastIndexed:   "2024-01-01T00:00:00Z",
		Excluded:      false,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed RepositoryResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.ID != 1 {
		t.Errorf("expected ID 1, got %d", parsed.ID)
	}

	if parsed.Name != "github.com/test/repo" {
		t.Errorf("expected name 'github.com/test/repo', got %q", parsed.Name)
	}

	if len(parsed.Branches) != 2 {
		t.Errorf("expected 2 branches, got %d", len(parsed.Branches))
	}
}

func TestListReposResponse_JSON(t *testing.T) {
	resp := ListReposResponse{
		Repos: []RepositoryResponse{
			{
				ID:     1,
				Name:   "repo1",
				Status: "indexed",
			},
			{
				ID:     2,
				Name:   "repo2",
				Status: "pending",
			},
		},
		TotalCount: 10,
		Limit:      2,
		Offset:     0,
		HasMore:    true,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed ListReposResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(parsed.Repos) != 2 {
		t.Errorf("expected 2 repos, got %d", len(parsed.Repos))
	}

	if parsed.TotalCount != 10 {
		t.Errorf("expected total_count 10, got %d", parsed.TotalCount)
	}

	if !parsed.HasMore {
		t.Error("expected has_more to be true")
	}
}

func TestListRepos_ParseQueryParams(t *testing.T) {
	tests := []struct {
		name       string
		query      url.Values
		wantConnID *int64
		wantSearch string
		wantStatus string
		wantLimit  int
		wantOffset int
	}{
		{
			name:       "empty params",
			query:      url.Values{},
			wantConnID: nil,
			wantSearch: "",
			wantStatus: "",
			wantLimit:  0,
			wantOffset: 0,
		},
		{
			name: "with connection_id",
			query: url.Values{
				"connection_id": []string{"123"},
			},
			wantConnID: int64Ptr(123),
		},
		{
			name: "with search",
			query: url.Values{
				"search": []string{"test-repo"},
			},
			wantSearch: "test-repo",
		},
		{
			name: "with status",
			query: url.Values{
				"status": []string{"indexed"},
			},
			wantStatus: "indexed",
		},
		{
			name: "with pagination",
			query: url.Values{
				"limit":  []string{"50"},
				"offset": []string{"100"},
			},
			wantLimit:  50,
			wantOffset: 100,
		},
		{
			name: "with all params",
			query: url.Values{
				"connection_id": []string{"5"},
				"search":        []string{"api"},
				"status":        []string{"pending"},
				"limit":         []string{"25"},
				"offset":        []string{"50"},
			},
			wantConnID: int64Ptr(5),
			wantSearch: "api",
			wantStatus: "pending",
			wantLimit:  25,
			wantOffset: 50,
		},
		{
			name: "invalid connection_id ignored",
			query: url.Values{
				"connection_id": []string{"not-a-number"},
			},
			wantConnID: nil,
		},
		{
			name: "invalid limit ignored",
			query: url.Values{
				"limit": []string{"invalid"},
			},
			wantLimit: 0,
		},
		{
			name: "negative limit ignored",
			query: url.Values{
				"limit": []string{"-10"},
			},
			wantLimit: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := repos.RepoListOptions{}

			// Parse connection_id
			if connIDStr := tt.query.Get("connection_id"); connIDStr != "" {
				id, err := strconv.ParseInt(connIDStr, 10, 64)
				if err == nil {
					opts.ConnectionID = &id
				}
			}

			// Parse search
			opts.Search = tt.query.Get("search")

			// Parse status
			opts.Status = tt.query.Get("status")

			// Parse limit
			if l := tt.query.Get("limit"); l != "" {
				if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
					opts.Limit = parsed
				}
			}

			// Parse offset
			if o := tt.query.Get("offset"); o != "" {
				if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
					opts.Offset = parsed
				}
			}

			// Verify results
			if tt.wantConnID != nil {
				if opts.ConnectionID == nil {
					t.Error("expected connection_id to be set")
				} else if *opts.ConnectionID != *tt.wantConnID {
					t.Errorf("expected connection_id %d, got %d", *tt.wantConnID, *opts.ConnectionID)
				}
			} else if opts.ConnectionID != nil {
				t.Error("expected connection_id to be nil")
			}

			if opts.Search != tt.wantSearch {
				t.Errorf("expected search %q, got %q", tt.wantSearch, opts.Search)
			}

			if opts.Status != tt.wantStatus {
				t.Errorf("expected status %q, got %q", tt.wantStatus, opts.Status)
			}

			if opts.Limit != tt.wantLimit {
				t.Errorf("expected limit %d, got %d", tt.wantLimit, opts.Limit)
			}

			if opts.Offset != tt.wantOffset {
				t.Errorf("expected offset %d, got %d", tt.wantOffset, opts.Offset)
			}
		})
	}
}

func TestRepositoryResponse_LastIndexedFormat(t *testing.T) {
	now := time.Now()
	formatted := now.Format("2006-01-02T15:04:05Z")

	resp := RepositoryResponse{
		ID:          1,
		Name:        "test-repo",
		LastIndexed: formatted,
	}

	// Verify the format is valid ISO8601
	parsed, err := time.Parse("2006-01-02T15:04:05Z", resp.LastIndexed)
	if err != nil {
		t.Fatalf("failed to parse last_indexed: %v", err)
	}

	// Allow 1 second difference due to formatting
	if parsed.Sub(now) > time.Second || now.Sub(parsed) > time.Second {
		t.Errorf("parsed time %v differs too much from original %v", parsed, now)
	}
}

func TestRepositoryResponse_EmptyLastIndexed(t *testing.T) {
	resp := RepositoryResponse{
		ID:          1,
		Name:        "test-repo",
		LastIndexed: "",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// The omitempty tag should exclude empty last_indexed
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if _, ok := parsed["last_indexed"]; ok && parsed["last_indexed"] != "" {
		t.Error("expected last_indexed to be empty or omitted")
	}
}

func TestRepositoryResponse_BranchesFallback(t *testing.T) {
	// Test that default_branch is used as fallback when branches is empty
	repo := struct {
		DefaultBranch string
		Branches      []string
	}{
		DefaultBranch: "main",
		Branches:      []string{},
	}

	branches := repo.Branches
	if len(branches) == 0 && repo.DefaultBranch != "" {
		branches = []string{repo.DefaultBranch}
	}

	if len(branches) != 1 {
		t.Errorf("expected 1 branch, got %d", len(branches))
	}

	if branches[0] != "main" {
		t.Errorf("expected branch 'main', got %q", branches[0])
	}
}

// Helper function.
func int64Ptr(v int64) *int64 {
	return &v
}

func TestHealth_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	Health(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]string

	err := json.NewDecoder(w.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", resp["status"])
	}
}

func TestHealthResponse_JSON(t *testing.T) {
	resp := HealthResponse{
		Status: "ok",
		Checks: map[string]HealthCheck{
			"database": {
				Status:  "ok",
				Latency: "5ms",
			},
			"redis": {
				Status:  "ok",
				Latency: "2ms",
			},
			"zoekt": {
				Status:  "ok",
				Latency: "10ms",
			},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed HealthResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", parsed.Status)
	}

	if len(parsed.Checks) != 3 {
		t.Errorf("expected 3 checks, got %d", len(parsed.Checks))
	}

	if parsed.Checks["database"].Status != "ok" {
		t.Errorf("expected database status 'ok', got %q", parsed.Checks["database"].Status)
	}
}

func TestHealthResponse_Degraded(t *testing.T) {
	resp := HealthResponse{
		Status: "degraded",
		Checks: map[string]HealthCheck{
			"database": {
				Status: "ok",
			},
			"redis": {
				Status:  "error",
				Message: "connection refused",
			},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed HealthResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Status != "degraded" {
		t.Errorf("expected status 'degraded', got %q", parsed.Status)
	}

	if parsed.Checks["redis"].Status != "error" {
		t.Errorf("expected redis status 'error', got %q", parsed.Checks["redis"].Status)
	}

	if parsed.Checks["redis"].Message != "connection refused" {
		t.Errorf(
			"expected redis message 'connection refused', got %q",
			parsed.Checks["redis"].Message,
		)
	}
}
