package replace

import (
	"testing"
	"time"

	"github.com/techquestsdev/code-search/internal/gitutil"
	"github.com/techquestsdev/code-search/internal/repos"
)

func TestConstants(t *testing.T) {
	if MaxFileSize != 10*1024*1024 {
		t.Errorf("MaxFileSize = %v, want %v (10MB)", MaxFileSize, 10*1024*1024)
	}

	if CloneTimeout != 10*time.Minute {
		t.Errorf("CloneTimeout = %v, want 10m", CloneTimeout)
	}

	if PushTimeout != 5*time.Minute {
		t.Errorf("PushTimeout = %v, want 5m", PushTimeout)
	}

	if DefaultConcurrency != 3 {
		t.Errorf("DefaultConcurrency = %v, want 3", DefaultConcurrency)
	}
}

func TestReplaceOptions(t *testing.T) {
	opts := ReplaceOptions{
		SearchPattern: "old_func",
		ReplaceWith:   "new_func",
		IsRegex:       false,
		CaseSensitive: true,
		FilePatterns:  []string{"*.go", "*.ts"},
		MRTitle:       "Rename function",
		MRDescription: "Automated rename",
		BranchName:    "feature/rename",
		DryRun:        true,
		Concurrency:   5,
	}

	if opts.SearchPattern != "old_func" {
		t.Errorf("SearchPattern = %v, want old_func", opts.SearchPattern)
	}

	if opts.ReplaceWith != "new_func" {
		t.Errorf("ReplaceWith = %v, want new_func", opts.ReplaceWith)
	}

	if opts.IsRegex {
		t.Error("IsRegex should be false")
	}

	if !opts.CaseSensitive {
		t.Error("CaseSensitive should be true")
	}

	if len(opts.FilePatterns) != 2 {
		t.Errorf("FilePatterns length = %v, want 2", len(opts.FilePatterns))
	}

	if !opts.DryRun {
		t.Error("DryRun should be true")
	}

	if opts.Concurrency != 5 {
		t.Errorf("Concurrency = %v, want 5", opts.Concurrency)
	}
}

func TestPrecomputedMatch(t *testing.T) {
	match := PrecomputedMatch{
		RepositoryID:   123,
		RepositoryName: "owner/repo",
		FilePath:       "src/main.go",
	}

	if match.RepositoryID != 123 {
		t.Errorf("RepositoryID = %v, want 123", match.RepositoryID)
	}

	if match.RepositoryName != "owner/repo" {
		t.Errorf("RepositoryName = %v, want owner/repo", match.RepositoryName)
	}

	if match.FilePath != "src/main.go" {
		t.Errorf("FilePath = %v, want src/main.go", match.FilePath)
	}
}

func TestFileMatch(t *testing.T) {
	match := FileMatch{
		Repo:     "owner/repo",
		FilePath: "internal/service.go",
		Matches:  5,
	}

	if match.Repo != "owner/repo" {
		t.Errorf("Repo = %v, want owner/repo", match.Repo)
	}

	if match.FilePath != "internal/service.go" {
		t.Errorf("FilePath = %v, want internal/service.go", match.FilePath)
	}

	if match.Matches != 5 {
		t.Errorf("Matches = %v, want 5", match.Matches)
	}
}

func TestReplacementResult(t *testing.T) {
	result := ReplacementResult{
		RepositoryID:   456,
		RepositoryName: "org/project",
		FilesModified:  10,
		MatchCount:     25,
		Error:          "",
	}

	if result.RepositoryID != 456 {
		t.Errorf("RepositoryID = %v, want 456", result.RepositoryID)
	}

	if result.RepositoryName != "org/project" {
		t.Errorf("RepositoryName = %v, want org/project", result.RepositoryName)
	}

	if result.FilesModified != 10 {
		t.Errorf("FilesModified = %v, want 10", result.FilesModified)
	}

	if result.MatchCount != 25 {
		t.Errorf("MatchCount = %v, want 25", result.MatchCount)
	}

	if result.Error != "" {
		t.Errorf("Error = %v, want empty", result.Error)
	}
}

func TestReplacementResult_WithError(t *testing.T) {
	result := ReplacementResult{
		RepositoryID:   789,
		RepositoryName: "failed/repo",
		Error:          "permission denied",
	}

	if result.Error != "permission denied" {
		t.Errorf("Error = %v, want 'permission denied'", result.Error)
	}
}

func TestServiceConfig(t *testing.T) {
	cfg := &ServiceConfig{
		Concurrency:  10,
		CloneTimeout: 15 * time.Minute,
		PushTimeout:  10 * time.Minute,
		MaxFileSize:  20 * 1024 * 1024,
	}

	if cfg.Concurrency != 10 {
		t.Errorf("Concurrency = %v, want 10", cfg.Concurrency)
	}

	if cfg.CloneTimeout != 15*time.Minute {
		t.Errorf("CloneTimeout = %v, want 15m", cfg.CloneTimeout)
	}

	if cfg.PushTimeout != 10*time.Minute {
		t.Errorf("PushTimeout = %v, want 10m", cfg.PushTimeout)
	}

	if cfg.MaxFileSize != 20*1024*1024 {
		t.Errorf("MaxFileSize = %v, want 20MB", cfg.MaxFileSize)
	}
}

func TestNewService_NilConfig(t *testing.T) {
	svc := NewService(nil, nil, "", nil, nil)

	if svc == nil {
		t.Fatal("NewService should not return nil")
	}

	if svc.workDir != "/tmp/codesearch-replace" {
		t.Errorf("workDir = %v, want /tmp/codesearch-replace", svc.workDir)
	}

	if svc.concurrency != DefaultConcurrency {
		t.Errorf("concurrency = %v, want %v", svc.concurrency, DefaultConcurrency)
	}

	if svc.cloneTimeout != CloneTimeout {
		t.Errorf("cloneTimeout = %v, want %v", svc.cloneTimeout, CloneTimeout)
	}

	if svc.pushTimeout != PushTimeout {
		t.Errorf("pushTimeout = %v, want %v", svc.pushTimeout, PushTimeout)
	}

	if svc.maxFileSize != MaxFileSize {
		t.Errorf("maxFileSize = %v, want %v", svc.maxFileSize, MaxFileSize)
	}
}

func TestNewService_CustomConfig(t *testing.T) {
	cfg := &ServiceConfig{
		Concurrency:  5,
		CloneTimeout: 5 * time.Minute,
		PushTimeout:  3 * time.Minute,
		MaxFileSize:  5 * 1024 * 1024,
	}

	svc := NewService(nil, nil, "/custom/path", nil, cfg)

	if svc.workDir != "/custom/path" {
		t.Errorf("workDir = %v, want /custom/path", svc.workDir)
	}

	if svc.concurrency != 5 {
		t.Errorf("concurrency = %v, want 5", svc.concurrency)
	}

	if svc.cloneTimeout != 5*time.Minute {
		t.Errorf("cloneTimeout = %v, want 5m", svc.cloneTimeout)
	}

	if svc.pushTimeout != 3*time.Minute {
		t.Errorf("pushTimeout = %v, want 3m", svc.pushTimeout)
	}

	if svc.maxFileSize != 5*1024*1024 {
		t.Errorf("maxFileSize = %v, want 5MB", svc.maxFileSize)
	}
}

func TestSanitizeBranchName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple branch",
			input:    "feature/test",
			expected: "feature/test",
		},
		{
			name:     "with spaces",
			input:    "feature test",
			expected: "feature-test",
		},
		{
			name:     "special characters",
			input:    "fix: bug #123",
			expected: "fix-bug-123",
		},
		{
			name:     "consecutive invalid chars",
			input:    "fix  bug",
			expected: "fix-bug",
		},
		{
			name:     "leading/trailing hyphens",
			input:    "-test-branch-",
			expected: "test-branch",
		},
		{
			name:     "leading/trailing dots",
			input:    ".feature.branch.",
			expected: "feature.branch",
		},
		{
			name:     "ends with .lock",
			input:    "branch.lock",
			expected: "branch",
		},
		{
			name:     "double slashes",
			input:    "feature//test//branch",
			expected: "feature/test/branch",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "codesearch-replace",
		},
		{
			name:     "only invalid chars",
			input:    "!@#$%",
			expected: "codesearch-replace",
		},
		{
			name:     "underscores allowed",
			input:    "feature_test_branch",
			expected: "feature_test_branch",
		},
		{
			name:     "alphanumeric allowed",
			input:    "feature123branch456",
			expected: "feature123branch456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeBranchName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeBranchName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
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
			conn:     &repos.Connection{Type: "github", Token: "ghp_token123"},
			expected: "https://x-access-token:ghp_token123@github.com/owner/repo.git",
		},
		{
			name: "gitlab with token",
			url:  "https://gitlab.com/owner/repo.git",
			conn: &repos.Connection{
				Type:  "gitlab",
				URL:   "https://gitlab.com",
				Token: "glpat_token",
			},
			expected: "https://oauth2:glpat_token@gitlab.com/owner/repo.git",
		},
		{
			name:     "gitlab default URL",
			url:      "https://gitlab.com/owner/repo.git",
			conn:     &repos.Connection{Type: "gitlab", Token: "glpat_token"},
			expected: "https://oauth2:glpat_token@gitlab.com/owner/repo.git",
		},
		{
			name:     "bitbucket with token",
			url:      "https://bitbucket.org/owner/repo.git",
			conn:     &repos.Connection{Type: "bitbucket", Token: "bb_token"},
			expected: "https://x-token-auth:bb_token@bitbucket.org/owner/repo.git",
		},
		{
			name: "github enterprise",
			url:  "https://ghe.company.com/owner/repo.git",
			conn: &repos.Connection{
				Type:  "github_enterprise",
				URL:   "https://ghe.company.com",
				Token: "token",
			},
			expected: "https://x-access-token:token@ghe.company.com/owner/repo.git",
		},
		{
			name: "gitea",
			url:  "https://gitea.example.com/owner/repo.git",
			conn: &repos.Connection{
				Type:  "gitea",
				URL:   "https://gitea.example.com",
				Token: "token",
			},
			expected: "https://git:token@gitea.example.com/owner/repo.git",
		},
		{
			name:     "unknown type",
			url:      "https://example.com/owner/repo.git",
			conn:     &repos.Connection{Type: "unknown", Token: "token"},
			expected: "https://example.com/owner/repo.git",
		},
		{
			name:     "github url without github.com",
			url:      "https://other.com/owner/repo.git",
			conn:     &repos.Connection{Type: "github", Token: "token"},
			expected: "https://other.com/owner/repo.git",
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

func TestExtractHost(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "https with path",
			url:      "https://github.com/owner/repo",
			expected: "github.com",
		},
		{
			name:     "https no path",
			url:      "https://github.com",
			expected: "github.com",
		},
		{
			name:     "https trailing slash",
			url:      "https://github.com/",
			expected: "github.com",
		},
		{
			name:     "http",
			url:      "http://github.com/owner/repo",
			expected: "github.com",
		},
		{
			name:     "with port",
			url:      "https://github.com:443/owner/repo",
			expected: "github.com:443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gitutil.ExtractHost(tt.url)
			if result != tt.expected {
				t.Errorf("ExtractHost(%q) = %q, want %q", tt.url, result, tt.expected)
			}
		})
	}
}

func TestSanitizeGitOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		conn     *repos.Connection
		expected string
	}{
		{
			name:     "nil connection",
			output:   "fatal: authentication failed for 'https://github.com'",
			conn:     nil,
			expected: "fatal: authentication failed for 'https://github.com'",
		},
		{
			name:     "empty token",
			output:   "fatal: authentication failed for 'https://github.com'",
			conn:     &repos.Connection{Token: ""},
			expected: "fatal: authentication failed for 'https://github.com'",
		},
		{
			name:     "token in output",
			output:   "fatal: unable to access 'https://ghp_secret123@github.com/repo': could not connect",
			conn:     &repos.Connection{Token: "ghp_secret123"},
			expected: "fatal: unable to access 'https://[REDACTED]@github.com/repo': could not connect",
		},
		{
			name:     "multiple occurrences",
			output:   "token=abc123 and secret=abc123",
			conn:     &repos.Connection{Token: "abc123"},
			expected: "token=[REDACTED] and secret=[REDACTED]",
		},
		{
			name:     "no token in output",
			output:   "Everything up-to-date",
			conn:     &repos.Connection{Token: "secret"},
			expected: "Everything up-to-date",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gitutil.SanitizeGitOutput(tt.output, tt.conn)
			if result != tt.expected {
				t.Errorf(
					"SanitizeGitOutput(%q, %+v) = %q, want %q",
					tt.output,
					tt.conn,
					result,
					tt.expected,
				)
			}
		})
	}
}

func TestIsBinaryContent(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		expected bool
	}{
		{
			name:     "text content",
			content:  []byte("Hello, World!"),
			expected: false,
		},
		{
			name:     "binary with NUL",
			content:  []byte("hello\x00world"),
			expected: true,
		},
		{
			name:     "empty",
			content:  []byte{},
			expected: false,
		},
		{
			name:     "unicode text",
			content:  []byte("Hello, 世界! 🌍"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isBinaryContent(tt.content)
			if result != tt.expected {
				t.Errorf("isBinaryContent(%q) = %v, want %v", tt.content, result, tt.expected)
			}
		})
	}
}

func TestCountFileMatches(t *testing.T) {
	tests := []struct {
		name     string
		files    []FileMatch
		expected int
	}{
		{
			name:     "empty",
			files:    []FileMatch{},
			expected: 0,
		},
		{
			name: "single file",
			files: []FileMatch{
				{Matches: 5},
			},
			expected: 5,
		},
		{
			name: "multiple files",
			files: []FileMatch{
				{Matches: 5},
				{Matches: 3},
				{Matches: 2},
			},
			expected: 10,
		},
		{
			name: "with zero matches",
			files: []FileMatch{
				{Matches: 5},
				{Matches: 0},
				{Matches: 3},
			},
			expected: 8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countFileMatches(tt.files)
			if result != tt.expected {
				t.Errorf("countFileMatches(%v) = %d, want %d", tt.files, result, tt.expected)
			}
		})
	}
}

func TestGenerateRandomSuffix(t *testing.T) {
	suffix1 := generateRandomSuffix()
	suffix2 := generateRandomSuffix()

	if len(suffix1) != 6 {
		t.Errorf("generateRandomSuffix() returned length %d, want 6", len(suffix1))
	}

	// Check that two calls produce different results (with high probability)
	if suffix1 == suffix2 {
		t.Logf(
			"Warning: two calls to generateRandomSuffix produced same result (possible but unlikely)",
		)
	}

	// Verify it's valid hex
	for _, c := range suffix1 {
		isDigit := c >= '0' && c <= '9'
		isHexLetter := c >= 'a' && c <= 'f'

		if !isDigit && !isHexLetter {
			t.Errorf("generateRandomSuffix() returned invalid hex character: %c", c)
		}
	}
}
