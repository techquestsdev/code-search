package scheduler

import (
	"testing"
	"time"

	"github.com/techquestsdev/code-search/internal/gitutil"
	"github.com/techquestsdev/code-search/internal/repos"
)

func TestConfig_Defaults(t *testing.T) {
	cfg := Config{
		Enabled:             true,
		DefaultPollInterval: 6 * time.Hour,
		CheckInterval:       5 * time.Minute,
		StaleThreshold:      24 * time.Hour,
		MaxConcurrentChecks: 5,
		ReposPath:           "/data/repos",
		JobRetentionPeriod:  1 * time.Hour,
	}

	if !cfg.Enabled {
		t.Error("Enabled should be true")
	}

	if cfg.DefaultPollInterval != 6*time.Hour {
		t.Errorf("DefaultPollInterval = %v, want 6h", cfg.DefaultPollInterval)
	}

	if cfg.CheckInterval != 5*time.Minute {
		t.Errorf("CheckInterval = %v, want 5m", cfg.CheckInterval)
	}

	if cfg.StaleThreshold != 24*time.Hour {
		t.Errorf("StaleThreshold = %v, want 24h", cfg.StaleThreshold)
	}

	if cfg.MaxConcurrentChecks != 5 {
		t.Errorf("MaxConcurrentChecks = %v, want 5", cfg.MaxConcurrentChecks)
	}

	if cfg.ReposPath != "/data/repos" {
		t.Errorf("ReposPath = %v, want /data/repos", cfg.ReposPath)
	}

	if cfg.JobRetentionPeriod != 1*time.Hour {
		t.Errorf("JobRetentionPeriod = %v, want 1h", cfg.JobRetentionPeriod)
	}
}

func TestStats(t *testing.T) {
	stats := Stats{
		TotalCount:    100,
		IndexedCount:  80,
		PendingCount:  5,
		IndexingCount: 3,
		FailedCount:   12,
		StaleCount:    15,
		NextCheckAt:   time.Now().Add(5 * time.Minute),
	}

	if stats.TotalCount != 100 {
		t.Errorf("TotalCount = %v, want 100", stats.TotalCount)
	}

	if stats.IndexedCount != 80 {
		t.Errorf("IndexedCount = %v, want 80", stats.IndexedCount)
	}

	if stats.PendingCount != 5 {
		t.Errorf("PendingCount = %v, want 5", stats.PendingCount)
	}

	if stats.IndexingCount != 3 {
		t.Errorf("IndexingCount = %v, want 3", stats.IndexingCount)
	}

	if stats.FailedCount != 12 {
		t.Errorf("FailedCount = %v, want 12", stats.FailedCount)
	}

	if stats.StaleCount != 15 {
		t.Errorf("StaleCount = %v, want 15", stats.StaleCount)
	}

	if stats.NextCheckAt.IsZero() {
		t.Error("NextCheckAt should not be zero")
	}
}

func TestRepoNeedingSync(t *testing.T) {
	now := time.Now()
	pollInterval := 6 * time.Hour

	repo := RepoNeedingSync{
		ID:            123,
		ConnectionID:  456,
		Name:          "owner/repo",
		CloneURL:      "https://github.com/owner/repo.git",
		DefaultBranch: "main",
		IndexStatus:   "indexed",
		LastIndexed:   &now,
		PollInterval:  &pollInterval,
		LocalPath:     "/data/repos/owner_repo.git",
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

	if repo.IndexStatus != "indexed" {
		t.Errorf("IndexStatus = %v, want indexed", repo.IndexStatus)
	}

	if repo.LastIndexed == nil || !repo.LastIndexed.Equal(now) {
		t.Error("LastIndexed mismatch")
	}

	if repo.PollInterval == nil || *repo.PollInterval != pollInterval {
		t.Error("PollInterval mismatch")
	}

	if repo.LocalPath != "/data/repos/owner_repo.git" {
		t.Errorf("LocalPath = %v, want /data/repos/owner_repo.git", repo.LocalPath)
	}
}

func TestRepoNeedingSync_NilOptionalFields(t *testing.T) {
	repo := RepoNeedingSync{
		ID:            1,
		ConnectionID:  1,
		Name:          "test/repo",
		CloneURL:      "https://example.com/test/repo.git",
		DefaultBranch: "master",
		IndexStatus:   "pending",
		LastIndexed:   nil,
		PollInterval:  nil,
		LocalPath:     "",
	}

	if repo.LastIndexed != nil {
		t.Error("LastIndexed should be nil")
	}

	if repo.PollInterval != nil {
		t.Error("PollInterval should be nil")
	}

	if repo.LocalPath != "" {
		t.Error("LocalPath should be empty")
	}
}

func TestAddAuthToURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		conn     *repos.Connection
		expected string
	}{
		{
			name:     "nil connection",
			url:      "https://github.com/owner/repo.git",
			conn:     nil,
			expected: "https://github.com/owner/repo.git",
		},
		{
			name:     "empty token",
			url:      "https://github.com/owner/repo.git",
			conn:     &repos.Connection{Type: "github", Token: ""},
			expected: "https://github.com/owner/repo.git",
		},
		{
			name:     "github with token",
			url:      "https://github.com/owner/repo.git",
			conn:     &repos.Connection{Type: "github", Token: "ghp_test123"},
			expected: "https://x-access-token:ghp_test123@github.com/owner/repo.git",
		},
		{
			name:     "gitlab with token",
			url:      "https://gitlab.com/owner/repo.git",
			conn:     &repos.Connection{Type: "gitlab", Token: "glpat_test123"},
			expected: "https://oauth2:glpat_test123@gitlab.com/owner/repo.git",
		},
		{
			name: "self-hosted gitlab",
			url:  "https://git.example.com/owner/repo.git",
			conn: &repos.Connection{
				Type:  "gitlab",
				URL:   "https://git.example.com",
				Token: "glpat_test123",
			},
			expected: "https://oauth2:glpat_test123@git.example.com/owner/repo.git",
		},
		{
			name: "gitea with token",
			url:  "https://gitea.example.com/owner/repo.git",
			conn: &repos.Connection{
				Type:  "gitea",
				URL:   "https://gitea.example.com",
				Token: "gitea_token",
			},
			expected: "https://git:gitea_token@gitea.example.com/owner/repo.git",
		},
		{
			name:     "bitbucket with token",
			url:      "https://bitbucket.org/owner/repo.git",
			conn:     &repos.Connection{Type: "bitbucket", Token: "bb_token"},
			expected: "https://x-token-auth:bb_token@bitbucket.org/owner/repo.git",
		},
		{
			name:     "unknown provider",
			url:      "https://unknown.com/owner/repo.git",
			conn:     &repos.Connection{Type: "unknown", Token: "token"},
			expected: "https://unknown.com/owner/repo.git",
		},
		{
			name:     "already has auth",
			url:      "https://user:pass@gitlab.com/owner/repo.git",
			conn:     &repos.Connection{Type: "gitlab", Token: "token"},
			expected: "https://user:pass@gitlab.com/owner/repo.git",
		},
		{
			name:     "non-github url with github type",
			url:      "https://example.com/owner/repo.git",
			conn:     &repos.Connection{Type: "github", Token: "token"},
			expected: "https://example.com/owner/repo.git",
		},
		{
			name:     "non-bitbucket url with bitbucket type",
			url:      "https://example.com/owner/repo.git",
			conn:     &repos.Connection{Type: "bitbucket", Token: "token"},
			expected: "https://example.com/owner/repo.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gitutil.AddAuthToURL(tt.url, tt.conn)
			if result != tt.expected {
				t.Errorf(
					"AddAuthToURL(%q, %+v) = %q, want %q",
					tt.url,
					tt.conn,
					result,
					tt.expected,
				)
			}
		})
	}
}

func TestAddAuthToURL_HTTPOnly(t *testing.T) {
	conn := &repos.Connection{Type: "gitea", Token: "token"}

	httpURL := "http://gitea.example.com/owner/repo.git"
	result := gitutil.AddAuthToURL(httpURL, conn)

	if result != httpURL {
		t.Errorf("HTTP URL should not be modified for gitea, got %q", result)
	}
}

func TestExtractRepoNameFromShard(t *testing.T) {
	tests := []struct {
		name      string
		shardName string
		expected  string
	}{
		{
			name:      "simple repo",
			shardName: "owner_repo_v16.00000.zoekt",
			expected:  "owner_repo",
		},
		{
			name:      "multi-level repo",
			shardName: "org_team_project_v16.00001.zoekt",
			expected:  "org_team_project",
		},
		{
			name:      "gitlab style with subgroups",
			shardName: "gitlab.example.com_group_subgroup_repo_v16.00000.zoekt",
			expected:  "gitlab.example.com_group_subgroup_repo",
		},
		{
			name:      "URL-encoded gitlab repo",
			shardName: "gitlab.molops.io%2Fapps%2Fterminal-app%2Fterminal-app-infra_v16.00000.zoekt",
			expected:  "gitlab.molops.io%2Fapps%2Fterminal-app%2Fterminal-app-infra",
		},
		{
			name:      "URL-encoded simple path",
			shardName: "gitlab.molops.io%2Futils%2Fgoogle-pso-code_v16.00000.zoekt",
			expected:  "gitlab.molops.io%2Futils%2Fgoogle-pso-code",
		},
		{
			name:      "different version",
			shardName: "myrepo_v17.00000.zoekt",
			expected:  "myrepo",
		},
		{
			name:      "multiple shards",
			shardName: "big_repo_v16.00005.zoekt",
			expected:  "big_repo",
		},
		{
			name:      "no version pattern",
			shardName: "somefile.zoekt",
			expected:  "",
		},
		{
			name:      "repo with v in name",
			shardName: "my_v2_project_v16.00000.zoekt",
			expected:  "my_v2_project",
		},
		{
			name:      "compound shard should not match",
			shardName: "compound_12345.zoekt",
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRepoNameFromShard(tt.shardName)
			if result != tt.expected {
				t.Errorf("extractRepoNameFromShard(%q) = %q, want %q",
					tt.shardName, result, tt.expected)
			}
		})
	}
}

func TestIsVersionSuffix(t *testing.T) {
	tests := []struct {
		suffix   string
		expected bool
	}{
		{"16.00000", true},
		{"17.00001", true},
		{"1.0", true},
		{"123.456789", true},
		{"abc.def", false},
		{"16.abc", false},
		{"abc.00000", false},
		{".00000", false},
		{"16.", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.suffix, func(t *testing.T) {
			result := isVersionSuffix(tt.suffix)
			if result != tt.expected {
				t.Errorf("isVersionSuffix(%q) = %v, want %v",
					tt.suffix, result, tt.expected)
			}
		})
	}
}
