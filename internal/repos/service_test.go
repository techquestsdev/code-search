package repos

import (
	"os"
	"testing"
	"time"

	"github.com/aanogueira/code-search/internal/db"
)

func TestCodeHostConfig_ResolveToken(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		envVar   string
		envValue string
		expected string
	}{
		{
			name:     "literal token",
			token:    "ghp_abc123",
			expected: "ghp_abc123",
		},
		{
			name:     "env var reference",
			token:    "$TEST_TOKEN",
			envVar:   "TEST_TOKEN",
			envValue: "secret_from_env",
			expected: "secret_from_env",
		},
		{
			name:     "env var not set",
			token:    "$NONEXISTENT_VAR",
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
			if tt.envVar != "" {
				os.Setenv(tt.envVar, tt.envValue)
				defer os.Unsetenv(tt.envVar)
			}

			cfg := &CodeHostConfig{Token: tt.token}
			result := cfg.ResolveToken()

			if result != tt.expected {
				t.Errorf("ResolveToken() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestRepository_Fields(t *testing.T) {
	now := time.Now()
	lastIndexed := now.Add(-1 * time.Hour)

	repo := Repository{
		ID:            123,
		ConnectionID:  456,
		Name:          "owner/repo",
		CloneURL:      "https://github.com/owner/repo.git",
		DefaultBranch: "main",
		Branches:      db.StringArray{"main", "develop"},
		LastIndexed:   &lastIndexed,
		IndexStatus:   "indexed",
		Excluded:      false,
		Deleted:       false,
		Archived:      false,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if repo.ID != 123 {
		t.Errorf("ID = %v, want 123", repo.ID)
	}

	if repo.ConnectionID != 456 {
		t.Errorf("ConnectionID = %v, want 456", repo.ConnectionID)
	}

	if repo.Name != "owner/repo" {
		t.Errorf("Name = %v, want owner/repo", repo.Name)
	}

	if repo.CloneURL != "https://github.com/owner/repo.git" {
		t.Errorf("CloneURL = %v, want https://github.com/owner/repo.git", repo.CloneURL)
	}

	if repo.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %v, want main", repo.DefaultBranch)
	}

	if len(repo.Branches) != 2 {
		t.Errorf("Branches length = %v, want 2", len(repo.Branches))
	}

	if repo.IndexStatus != "indexed" {
		t.Errorf("IndexStatus = %v, want indexed", repo.IndexStatus)
	}

	if repo.Excluded {
		t.Error("Excluded should be false")
	}

	if repo.Deleted {
		t.Error("Deleted should be false")
	}

	if repo.Archived {
		t.Error("Archived should be false")
	}
}

func TestRepository_DeletedState(t *testing.T) {
	now := time.Now()

	// Test a deleted repository
	repo := Repository{
		ID:          1,
		Name:        "owner/deleted-repo",
		IndexStatus: "indexed",
		Excluded:    true, // Deleted repos are also excluded
		Deleted:     true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if !repo.Deleted {
		t.Error("Deleted should be true")
	}

	if !repo.Excluded {
		t.Error("Excluded should be true for deleted repos")
	}

	// Test restoring a deleted repository
	repo.Deleted = false
	repo.Excluded = false

	if repo.Deleted {
		t.Error("Deleted should be false after restore")
	}

	if repo.Excluded {
		t.Error("Excluded should be false after restore")
	}
}

func TestConnection_Fields(t *testing.T) {
	now := time.Now()

	conn := Connection{
		ID:              1,
		Name:            "github-main",
		Type:            "github",
		URL:             "https://api.github.com",
		Token:           "ghp_test123",
		ExcludeArchived: true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if conn.ID != 1 {
		t.Errorf("ID = %v, want 1", conn.ID)
	}

	if conn.Name != "github-main" {
		t.Errorf("Name = %v, want github-main", conn.Name)
	}

	if conn.Type != "github" {
		t.Errorf("Type = %v, want github", conn.Type)
	}

	if conn.URL != "https://api.github.com" {
		t.Errorf("URL = %v, want https://api.github.com", conn.URL)
	}

	if conn.Token != "ghp_test123" {
		t.Errorf("Token = %v, want ghp_test123", conn.Token)
	}

	if !conn.ExcludeArchived {
		t.Error("ExcludeArchived should be true")
	}
}

func TestRepoStats(t *testing.T) {
	stats := RepoStats{
		Total:    100,
		Indexed:  80,
		Pending:  5,
		Indexing: 3,
		Failed:   2,
		Excluded: 7,
		Deleted:  3,
	}

	if stats.Total != 100 {
		t.Errorf("Total = %v, want 100", stats.Total)
	}

	if stats.Indexed != 80 {
		t.Errorf("Indexed = %v, want 80", stats.Indexed)
	}

	if stats.Pending != 5 {
		t.Errorf("Pending = %v, want 5", stats.Pending)
	}

	if stats.Indexing != 3 {
		t.Errorf("Indexing = %v, want 3", stats.Indexing)
	}

	if stats.Failed != 2 {
		t.Errorf("Failed = %v, want 2", stats.Failed)
	}

	if stats.Excluded != 7 {
		t.Errorf("Excluded = %v, want 7", stats.Excluded)
	}

	if stats.Deleted != 3 {
		t.Errorf("Deleted = %v, want 3", stats.Deleted)
	}

	activeRepos := stats.Indexed + stats.Pending + stats.Indexing + stats.Failed
	if activeRepos != 90 {
		t.Errorf("Active repos = %v, want 90", activeRepos)
	}
}

func TestRepoListOptions(t *testing.T) {
	connID := int64(123)

	opts := RepoListOptions{
		ConnectionID: &connID,
		Search:       "test",
		Status:       "indexed",
		Limit:        50,
		Offset:       100,
	}

	if opts.ConnectionID == nil || *opts.ConnectionID != 123 {
		t.Error("ConnectionID mismatch")
	}

	if opts.Search != "test" {
		t.Errorf("Search = %v, want test", opts.Search)
	}

	if opts.Status != "indexed" {
		t.Errorf("Status = %v, want indexed", opts.Status)
	}

	if opts.Limit != 50 {
		t.Errorf("Limit = %v, want 50", opts.Limit)
	}

	if opts.Offset != 100 {
		t.Errorf("Offset = %v, want 100", opts.Offset)
	}
}

func TestRepoListResult(t *testing.T) {
	result := RepoListResult{
		Repos:      []Repository{{ID: 1}, {ID: 2}},
		TotalCount: 100,
		Limit:      50,
		Offset:     0,
		HasMore:    true,
	}

	if len(result.Repos) != 2 {
		t.Errorf("Repos length = %v, want 2", len(result.Repos))
	}

	if result.TotalCount != 100 {
		t.Errorf("TotalCount = %v, want 100", result.TotalCount)
	}

	if result.Limit != 50 {
		t.Errorf("Limit = %v, want 50", result.Limit)
	}

	if result.Offset != 0 {
		t.Errorf("Offset = %v, want 0", result.Offset)
	}

	if !result.HasMore {
		t.Error("HasMore should be true")
	}
}

func TestCodeHostRepository(t *testing.T) {
	repo := CodeHostRepository{
		Name:          "repo",
		FullName:      "owner/repo",
		CloneURL:      "https://github.com/owner/repo.git",
		DefaultBranch: "main",
		Private:       true,
		Archived:      false,
	}

	if repo.Name != "repo" {
		t.Errorf("Name = %v, want repo", repo.Name)
	}

	if repo.FullName != "owner/repo" {
		t.Errorf("FullName = %v, want owner/repo", repo.FullName)
	}

	if repo.CloneURL != "https://github.com/owner/repo.git" {
		t.Errorf("CloneURL = %v, want https://github.com/owner/repo.git", repo.CloneURL)
	}

	if repo.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %v, want main", repo.DefaultBranch)
	}

	if !repo.Private {
		t.Error("Private should be true")
	}

	if repo.Archived {
		t.Error("Archived should be false")
	}
}

func TestPrepareBranchesArg_PostgreSQL(t *testing.T) {
	branches := []string{"main", "develop", "feature/test"}

	result := prepareBranchesArg(db.DriverPostgres, branches)

	resultSlice, ok := result.([]string)
	if !ok {
		t.Fatalf("Expected []string for PostgreSQL, got %T", result)
	}

	if len(resultSlice) != 3 {
		t.Errorf("Length = %v, want 3", len(resultSlice))
	}
}

func TestPrepareBranchesArg_MySQL(t *testing.T) {
	branches := []string{"main", "develop"}

	result := prepareBranchesArg(db.DriverMySQL, branches)

	resultStr, ok := result.(string)
	if !ok {
		t.Fatalf("Expected string for MySQL, got %T", result)
	}

	if resultStr != `["main","develop"]` {
		t.Errorf("Result = %v, want [\"main\",\"develop\"]", resultStr)
	}
}

func TestPrepareBranchesArg_MySQL_Nil(t *testing.T) {
	result := prepareBranchesArg(db.DriverMySQL, nil)

	resultStr, ok := result.(string)
	if !ok {
		t.Fatalf("Expected string for MySQL, got %T", result)
	}

	if resultStr != "[]" {
		t.Errorf("Result = %v, want []", resultStr)
	}
}

func TestCodeHostConfig_Types(t *testing.T) {
	types := []string{"github", "gitlab", "gitea", "bitbucket"}

	for _, connType := range types {
		t.Run(connType, func(t *testing.T) {
			cfg := CodeHostConfig{
				Type:            connType,
				URL:             "https://" + connType + ".com",
				Token:           "test_token",
				ExcludeArchived: true,
			}

			if cfg.Type != connType {
				t.Errorf("Type = %v, want %v", cfg.Type, connType)
			}

			if cfg.Token != "test_token" {
				t.Errorf("Token = %v, want test_token", cfg.Token)
			}

			if !cfg.ExcludeArchived {
				t.Error("ExcludeArchived should be true")
			}
		})
	}
}

func TestRepository_IndexStatuses(t *testing.T) {
	statuses := []string{"pending", "indexing", "indexed", "failed"}

	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			repo := Repository{IndexStatus: status}

			if repo.IndexStatus != status {
				t.Errorf("IndexStatus = %v, want %v", repo.IndexStatus, status)
			}
		})
	}
}

func TestRepoListOptions_Defaults(t *testing.T) {
	opts := RepoListOptions{}

	if opts.ConnectionID != nil {
		t.Error("ConnectionID should be nil by default")
	}

	if opts.Search != "" {
		t.Error("Search should be empty by default")
	}

	if opts.Status != "" {
		t.Error("Status should be empty by default")
	}

	if opts.Limit != 0 {
		t.Error("Limit should be 0 by default (will be set to 50 in service)")
	}

	if opts.Offset != 0 {
		t.Error("Offset should be 0 by default")
	}
}

func TestRepoListResult_NoMore(t *testing.T) {
	result := RepoListResult{
		Repos:      []Repository{{ID: 1}, {ID: 2}},
		TotalCount: 2,
		Limit:      50,
		Offset:     0,
		HasMore:    false,
	}

	if result.HasMore {
		t.Error("HasMore should be false when all results are returned")
	}

	if len(result.Repos) != result.TotalCount {
		t.Error("Repos length should equal TotalCount when HasMore is false")
	}
}
