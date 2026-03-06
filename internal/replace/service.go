package replace

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/sourcegraph/zoekt/query"
	"go.uber.org/zap"

	"github.com/techquestsdev/code-search/internal/codehost"
	"github.com/techquestsdev/code-search/internal/gitutil"
	"github.com/techquestsdev/code-search/internal/queue"
	"github.com/techquestsdev/code-search/internal/regexutil"
	"github.com/techquestsdev/code-search/internal/repos"
	"github.com/techquestsdev/code-search/internal/search"
)

const (
	// MaxFileSize is the maximum file size to process (10MB).
	MaxFileSize = 10 * 1024 * 1024
	// CloneTimeout is the timeout for git clone operations.
	CloneTimeout = 10 * time.Minute
	// PushTimeout is the timeout for git push operations.
	PushTimeout = 5 * time.Minute
	// DefaultConcurrency is the default number of concurrent repository operations.
	DefaultConcurrency = 3
)

// ReplacementResult represents the result of a replacement operation.
type ReplacementResult struct {
	RepositoryID   int64
	RepositoryName string
	FilesModified  int
	MatchCount     int
	MergeRequest   *codehost.MergeRequest
	Error          string
}

// ExecuteOptions configures a replace execution (used by federated client).
type ExecuteOptions struct {
	SearchPattern string               `json:"search_pattern"`
	ReplaceWith   string               `json:"replace_with"`
	IsRegex       bool                 `json:"is_regex"`
	CaseSensitive bool                 `json:"case_sensitive"`
	Matches       []queue.ReplaceMatch `json:"matches"`
	BranchName    string               `json:"branch_name,omitempty"`
	MRTitle       string               `json:"mr_title,omitempty"`
	MRDescription string               `json:"mr_description,omitempty"`
	UserTokens    map[string]string    `json:"user_tokens,omitempty"`
	ReposReadOnly bool                 `json:"repos_readonly"`
}

// ExecuteResult represents the result of a replace execution.
type ExecuteResult struct {
	TotalFiles    int          `json:"total_files"`
	ModifiedFiles int          `json:"modified_files"`
	RepoResults   []RepoResult `json:"repo_results"`
}

// RepoResult represents the result for a single repository.
type RepoResult struct {
	RepositoryID   int64  `json:"repository_id"`
	RepositoryName string `json:"repository_name"`
	FilesModified  int    `json:"files_modified"`
	MRUrl          string `json:"mr_url,omitempty"`
	Error          string `json:"error,omitempty"`
}

// ReplaceOptions configures a replace operation.
type ReplaceOptions struct {
	SearchPattern string
	ReplaceWith   string
	IsRegex       bool
	CaseSensitive bool
	FilePatterns  []string // e.g., "*.go", "*.ts"
	Languages     []string // e.g., "go", "typescript"
	ContextLines  int      // Lines of context around matches (default: 2)
	MRTitle       string
	MRDescription string
	BranchName    string
	DryRun        bool
	Limit         int // Max results for preview, 0 means unlimited

	// PrecomputedMatches contains matches from preview (required for Execute).
	// Call Preview first to get matches, then pass them to Execute.
	PrecomputedMatches []PrecomputedMatch

	// Concurrency limits the number of repositories processed in parallel.
	// Defaults to DefaultConcurrency if not set.
	Concurrency int

	// UserTokens maps connection_id to user-provided token for repos without server-side auth.
	// Keys are connection IDs as strings (e.g., "123").
	UserTokens map[string]string

	// ReposReadOnly indicates the system is in read-only mode.
	// When true, only user-provided tokens can be used (never DB tokens).
	// DB tokens are reserved for indexing operations only.
	ReposReadOnly bool
}

// PrecomputedMatch represents a file match from a previous preview.
type PrecomputedMatch struct {
	RepositoryID   int64
	RepositoryName string
	FilePath       string
}

// FileMatch represents a file with matches for replacement.
type FileMatch struct {
	Repo     string
	FilePath string
	Matches  int
}

// repoInfo groups files by repository for batch processing.
type repoInfo struct {
	repoID int64
	files  []FileMatch
}

// Service handles search and replace operations.
type Service struct {
	searchService *search.Service
	repoService   *repos.Service
	workDir       string
	logger        *zap.Logger
	// Config options (can be overridden per-request via ReplaceOptions)
	concurrency  int
	cloneTimeout time.Duration
	pushTimeout  time.Duration
	maxFileSize  int64
}

// ServiceConfig holds configuration for the replace service.
type ServiceConfig struct {
	Concurrency  int
	CloneTimeout time.Duration
	PushTimeout  time.Duration
	MaxFileSize  int64
}

// NewService creates a new replace service.
func NewService(
	searchService *search.Service,
	repoService *repos.Service,
	workDir string,
	logger *zap.Logger,
	cfg *ServiceConfig,
) *Service {
	if workDir == "" {
		workDir = "/tmp/codesearch-replace"
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	// Apply defaults
	concurrency := DefaultConcurrency
	cloneTimeout := CloneTimeout
	pushTimeout := PushTimeout
	maxFileSize := int64(MaxFileSize)

	if cfg != nil {
		if cfg.Concurrency > 0 {
			concurrency = cfg.Concurrency
		}

		if cfg.CloneTimeout > 0 {
			cloneTimeout = cfg.CloneTimeout
		}

		if cfg.PushTimeout > 0 {
			pushTimeout = cfg.PushTimeout
		}

		if cfg.MaxFileSize > 0 {
			maxFileSize = cfg.MaxFileSize
		}
	}

	return &Service{
		searchService: searchService,
		repoService:   repoService,
		workDir:       workDir,
		logger:        logger,
		concurrency:   concurrency,
		cloneTimeout:  cloneTimeout,
		pushTimeout:   pushTimeout,
		maxFileSize:   maxFileSize,
	}
}

// Preview shows what would be replaced without making changes.
func (s *Service) Preview(ctx context.Context, opts ReplaceOptions) (*search.SearchResults, error) {
	// Limit behavior:
	// - 0: unlimited (uses search default of 10000)
	// - >0: explicit limit
	// The handler defaults to 1000 if not specified in request
	contextLines := opts.ContextLines
	if contextLines == 0 {
		contextLines = 2 // Default
	}

	req := search.SearchRequest{
		Query:         opts.SearchPattern,
		CaseSensitive: opts.CaseSensitive,
		IsRegex:       opts.IsRegex,
		FilePatterns:  opts.FilePatterns,
		Languages:     opts.Languages,
		Limit:         opts.Limit,
		ContextLines:  contextLines,
	}

	return s.searchService.Search(ctx, req)
}

// ExecuteFromOptions performs the replace operation using ExecuteOptions (federated API format).
// Converts ExecuteOptions to ReplaceOptions and calls Execute.
func (s *Service) ExecuteFromOptions(
	ctx context.Context,
	opts ExecuteOptions,
) (*ExecuteResult, error) {
	// Convert queue.ReplaceMatch to PrecomputedMatch
	precomputed := make([]PrecomputedMatch, len(opts.Matches))
	for i, m := range opts.Matches {
		precomputed[i] = PrecomputedMatch{
			RepositoryID:   m.RepositoryID,
			RepositoryName: m.RepositoryName,
			FilePath:       m.FilePath,
		}
	}

	replaceOpts := ReplaceOptions{
		SearchPattern:      opts.SearchPattern,
		ReplaceWith:        opts.ReplaceWith,
		IsRegex:            opts.IsRegex,
		CaseSensitive:      opts.CaseSensitive,
		BranchName:         opts.BranchName,
		MRTitle:            opts.MRTitle,
		MRDescription:      opts.MRDescription,
		UserTokens:         opts.UserTokens,
		ReposReadOnly:      opts.ReposReadOnly,
		PrecomputedMatches: precomputed,
	}

	results, err := s.Execute(ctx, replaceOpts)
	if err != nil {
		return nil, err
	}

	// Convert ReplacementResult to RepoResult
	execResult := &ExecuteResult{
		RepoResults: make([]RepoResult, len(results)),
	}

	for i, r := range results {
		execResult.TotalFiles += r.FilesModified
		if r.Error == "" {
			execResult.ModifiedFiles += r.FilesModified
		}

		repoResult := RepoResult{
			RepositoryID:   r.RepositoryID,
			RepositoryName: r.RepositoryName,
			FilesModified:  r.FilesModified,
			Error:          r.Error,
		}
		if r.MergeRequest != nil {
			repoResult.MRUrl = r.MergeRequest.URL
		}

		execResult.RepoResults[i] = repoResult
	}

	return execResult, nil
}

// Execute performs the replace operation using precomputed matches from preview.
// Matches must be provided - call Preview first to get the matches.
func (s *Service) Execute(ctx context.Context, opts ReplaceOptions) ([]ReplacementResult, error) {
	// Require precomputed matches - no search fallback
	if len(opts.PrecomputedMatches) == 0 {
		return nil, errors.New("no matches provided: call Preview first to get matches")
	}

	// Group matches by repository
	repoMatches := make(map[string]*repoInfo)

	for _, m := range opts.PrecomputedMatches {
		key := m.RepositoryName
		if key == "" {
			key = fmt.Sprintf("id:%d", m.RepositoryID)
		}

		if repoMatches[key] == nil {
			repoMatches[key] = &repoInfo{repoID: m.RepositoryID}
		}

		repoMatches[key].files = append(repoMatches[key].files, FileMatch{
			Repo:     m.RepositoryName,
			FilePath: m.FilePath,
			Matches:  1,
		})
	}

	// Deduplicate files (multiple matches in same file)
	for _, info := range repoMatches {
		seen := make(map[string]*FileMatch)
		for _, f := range info.files {
			if existing, ok := seen[f.FilePath]; ok {
				existing.Matches++
			} else {
				fm := f
				seen[f.FilePath] = &fm
			}
		}

		deduped := make([]FileMatch, 0, len(seen))
		for _, fm := range seen {
			deduped = append(deduped, *fm)
		}

		info.files = deduped
	}

	// Collect all repo keys from precomputed matches
	var targetRepoKeys []string
	for key := range repoMatches {
		targetRepoKeys = append(targetRepoKeys, key)
	}

	// Set concurrency limit (request option overrides service default)
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = s.concurrency
	}

	// Process repositories concurrently with a semaphore
	type repoJob struct {
		repoKey string
		info    *repoInfo
	}

	jobs := make(chan repoJob, len(targetRepoKeys))
	resultsChan := make(chan ReplacementResult, len(targetRepoKeys))

	// Start worker goroutines
	var wg sync.WaitGroup
	for range concurrency {
		wg.Go(func() {
			for job := range jobs {
				result := s.processRepoJob(ctx, job.repoKey, job.info, opts)
				resultsChan <- result
			}
		})
	}

	// Send jobs
	for _, repoKey := range targetRepoKeys {
		info, ok := repoMatches[repoKey]
		if !ok || len(info.files) == 0 {
			continue
		}

		jobs <- repoJob{repoKey: repoKey, info: info}
	}

	close(jobs)

	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	var results []ReplacementResult
	for result := range resultsChan {
		results = append(results, result)
	}

	return results, nil
}

// processRepoJob processes a single repository job.
func (s *Service) processRepoJob(
	ctx context.Context,
	repoKey string,
	info *repoInfo,
	opts ReplaceOptions,
) ReplacementResult {
	result := ReplacementResult{
		RepositoryName: repoKey,
		MatchCount:     countFileMatches(info.files),
	}

	// Get repository info
	var (
		repo *repos.Repository
		err  error
	)

	if info.repoID > 0 {
		// Use precomputed repo ID directly
		repo, err = s.repoService.GetRepository(ctx, info.repoID)
	} else {
		// Look up by name - use FindRepositoryByZoektName for flexible matching
		// since Zoekt may return names like "gitlab.example.com/group/repo" while
		// the database stores "group/repo"
		repo, err = s.repoService.FindRepositoryByZoektName(ctx, repoKey)
	}

	if err != nil {
		result.Error = err.Error()
		return result
	}

	if repo == nil {
		result.Error = "repository not found in database: " + repoKey
		return result
	}

	result.RepositoryID = repo.ID
	result.RepositoryName = repo.Name

	if opts.DryRun {
		result.FilesModified = len(info.files)
		return result
	}

	// Clone and modify
	modifiedCount, mr, err := s.processRepository(ctx, repo, info.files, opts)
	if err != nil {
		result.Error = err.Error()
	} else {
		result.FilesModified = modifiedCount
		result.MergeRequest = mr
	}

	return result
}

func (s *Service) processRepository(
	ctx context.Context,
	repo *repos.Repository,
	files []FileMatch,
	opts ReplaceOptions,
) (int, *codehost.MergeRequest, error) {
	// Get connection for authentication
	conn, err := s.repoService.GetConnection(ctx, repo.ConnectionID)
	if err != nil {
		return 0, nil, fmt.Errorf("get connection: %w", err)
	}

	if conn == nil {
		return 0, nil, fmt.Errorf("connection not found: %d", repo.ConnectionID)
	}

	// Determine the effective token for authentication
	// In read-only mode: ONLY use user-provided tokens (DB tokens are for indexing only)
	// In normal mode: Use DB token if available, fall back to user-provided token
	var effectiveToken string

	connIDStr := strconv.FormatInt(conn.ID, 10)

	// Helper to get user token by connection name, ID, or wildcard fallback
	// Priority: connection name > connection ID > wildcard "*"
	getUserToken := func() string {
		// First, try connection name (user-friendly)
		if token, ok := opts.UserTokens[conn.Name]; ok && token != "" {
			return token
		}
		// Then, try connection ID (for API/programmatic use)
		if token, ok := opts.UserTokens[connIDStr]; ok && token != "" {
			return token
		}
		// Finally, fall back to wildcard
		if token, ok := opts.UserTokens["*"]; ok && token != "" {
			return token
		}

		return ""
	}

	s.logger.Debug("Token selection for replace operation",
		zap.Int64("connection_id", conn.ID),
		zap.String("connection_name", conn.Name),
		zap.Bool("repos_read_only", opts.ReposReadOnly),
		zap.Bool("has_db_token", conn.Token != ""),
		zap.Bool("has_user_token", getUserToken() != ""),
		zap.Int("user_tokens_count", len(opts.UserTokens)),
	)

	if opts.ReposReadOnly {
		// Read-only mode: only user-provided tokens allowed
		effectiveToken = getUserToken()
		s.logger.Info("Read-only mode: using user-provided token only",
			zap.Int64("connection_id", conn.ID),
			zap.Bool("token_found", effectiveToken != ""),
		)

		if effectiveToken == "" {
			return 0, nil, fmt.Errorf(
				"authentication required: in read-only mode, you must provide a personal access token for connection %q (id=%d)",
				conn.Name,
				conn.ID,
			)
		}
	} else {
		// Normal mode: prefer DB token, fall back to user-provided
		effectiveToken = conn.Token
		if effectiveToken == "" {
			effectiveToken = getUserToken()
		}

		if effectiveToken == "" {
			return 0, nil, fmt.Errorf("authentication required: connection %q (id=%d) has no token configured - cannot push changes or create merge requests without credentials", conn.Name, conn.ID)
		}
	}

	// Create a connection with the effective token for this operation
	if effectiveToken != conn.Token {
		conn = &repos.Connection{
			ID:              conn.ID,
			Name:            conn.Name,
			Type:            conn.Type,
			URL:             conn.URL,
			Token:           effectiveToken,
			ExcludeArchived: conn.ExcludeArchived,
			Repos:           conn.Repos,
			CreatedAt:       conn.CreatedAt,
			UpdatedAt:       conn.UpdatedAt,
		}
	}

	// Create unique work directory with random suffix to avoid collisions
	randomSuffix := generateRandomSuffix()

	repoDir := filepath.Join(
		s.workDir,
		fmt.Sprintf("%d-%d-%s", repo.ID, time.Now().Unix(), randomSuffix),
	)
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		return 0, nil, fmt.Errorf("create work dir: %w", err)
	}
	defer os.RemoveAll(repoDir)

	s.logger.Info("Processing repository for replace",
		zap.Int64("repo_id", repo.ID),
		zap.String("repo_name", repo.Name),
		zap.Int("files_count", len(files)),
		zap.String("work_dir", repoDir),
	)

	// Clone the repository with authentication
	if err := s.cloneRepo(ctx, repo.CloneURL, repoDir, conn); err != nil {
		return 0, nil, fmt.Errorf("clone: %w", err)
	}

	// Create branch for changes with sanitized name
	branchName := opts.BranchName
	if branchName == "" {
		branchName = fmt.Sprintf("codesearch/replace-%d-%s", time.Now().Unix(), randomSuffix)
	} else {
		branchName = sanitizeBranchName(branchName)
	}

	if err := s.createBranch(ctx, repoDir, branchName); err != nil {
		return 0, nil, fmt.Errorf("create branch: %w", err)
	}

	// Apply replacements
	modifiedCount := 0

	for _, file := range files {
		filePath := filepath.Join(repoDir, file.FilePath)

		modified, err := s.replaceInFile(filePath, opts)
		if err != nil {
			s.logger.Warn("Failed to replace in file",
				zap.String("file", file.FilePath),
				zap.Error(err),
			)

			continue
		}

		if modified {
			modifiedCount++

			s.logger.Debug("Modified file",
				zap.String("file", file.FilePath),
			)
		}
	}

	if modifiedCount == 0 {
		s.logger.Info("No files modified, skipping MR creation",
			zap.String("repo_name", repo.Name),
		)

		return 0, nil, nil
	}

	// Commit changes
	commitMsg := opts.MRTitle
	if commitMsg == "" {
		commitMsg = fmt.Sprintf("Replace '%s' with '%s'", opts.SearchPattern, opts.ReplaceWith)
	}

	if err := s.commitChanges(ctx, repoDir, commitMsg); err != nil {
		return modifiedCount, nil, fmt.Errorf("commit: %w", err)
	}

	// Push with authentication and retry logic
	if err := s.pushBranchWithRetry(ctx, repoDir, branchName, conn, 3); err != nil {
		return modifiedCount, nil, fmt.Errorf("push: %w", err)
	}

	// Create merge request using code host client
	client, err := s.getCodeHostClient(ctx, repo.ConnectionID)
	if err != nil {
		return modifiedCount, nil, fmt.Errorf("get code host client: %w", err)
	}

	description := opts.MRDescription
	if description == "" {
		description = fmt.Sprintf("Automated replacement of '%s' with '%s'\n\nModified %d files.",
			opts.SearchPattern, opts.ReplaceWith, modifiedCount)
	}

	// Use default branch, falling back to "main" if not set
	targetBranch := repo.DefaultBranch
	if targetBranch == "" {
		targetBranch = "main"

		s.logger.Warn("Repository has no default branch set, using 'main'",
			zap.String("repo_name", repo.Name),
		)
	}

	mr, err := client.CreateMergeRequest(
		ctx,
		repo.Name,
		commitMsg,
		description,
		branchName,
		targetBranch,
	)
	if err != nil {
		return modifiedCount, nil, fmt.Errorf("create MR: %w", err)
	}

	s.logger.Info("Created merge request",
		zap.String("repo_name", repo.Name),
		zap.String("mr_url", mr.URL),
		zap.Int("files_modified", modifiedCount),
	)

	return modifiedCount, mr, nil
}

// configureGitCmd sets environment variables on a git command to prevent
// credential prompts (keychain, terminal prompts, etc.)
func configureGitCmd(cmd *exec.Cmd) {
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0", // Disable terminal prompts
		"GIT_ASKPASS=",          // Disable askpass
		"SSH_ASKPASS=",          // Disable SSH askpass
		"GCM_INTERACTIVE=never", // Disable Git Credential Manager prompts
	)
}

// cloneRepo clones a repository with authentication.
func (s *Service) cloneRepo(
	ctx context.Context,
	cloneURL, destDir string,
	conn *repos.Connection,
) error {
	// Add authentication to URL
	authURL := gitutil.AddAuthToURL(cloneURL, conn)

	// Create a timeout context for clone operation (use configured timeout)
	cloneCtx, cancel := context.WithTimeout(ctx, s.cloneTimeout)
	defer cancel()

	// Shallow clone — we only need the latest commit to apply changes and push a new branch
	cmd := exec.CommandContext(cloneCtx, "git", "clone", "--depth", "1", "--single-branch", authURL, destDir)
	configureGitCmd(cmd)

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Sanitize output to remove any credentials
		sanitizedOutput := gitutil.SanitizeGitOutput(string(output), conn)
		if errors.Is(cloneCtx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("git clone timed out after %v: %s", s.cloneTimeout, sanitizedOutput)
		}

		return fmt.Errorf("git clone failed: %w\nOutput: %s", err, sanitizedOutput)
	}

	// Reset remote URL to remove token (security: don't store credentials in git config)
	resetCmd := exec.CommandContext(
		ctx,
		"git",
		"-C",
		destDir,
		"remote",
		"set-url",
		"origin",
		cloneURL,
	)
	_ = resetCmd.Run() // Ignore errors, not critical

	return nil
}

func (s *Service) createBranch(ctx context.Context, repoDir, branchName string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repoDir, "checkout", "-b", branchName)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git checkout -b failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

func (s *Service) commitChanges(ctx context.Context, repoDir, message string) error {
	// Configure git user for commit
	configEmailCmd := exec.CommandContext(
		ctx,
		"git",
		"-C",
		repoDir,
		"config",
		"user.email",
		"codesearch@automated.local",
	)
	if output, err := configEmailCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git config user.email failed: %w\nOutput: %s", err, string(output))
	}

	configNameCmd := exec.CommandContext(
		ctx,
		"git",
		"-C",
		repoDir,
		"config",
		"user.name",
		"Code Search Bot",
	)
	if output, err := configNameCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git config user.name failed: %w\nOutput: %s", err, string(output))
	}

	addCmd := exec.CommandContext(ctx, "git", "-C", repoDir, "add", "-A")
	if output, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add failed: %w\nOutput: %s", err, string(output))
	}

	commitCmd := exec.CommandContext(ctx, "git", "-C", repoDir, "commit", "-m", message)
	if output, err := commitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// pushBranchWithRetry pushes a branch with retry logic for transient failures.
func (s *Service) pushBranchWithRetry(
	ctx context.Context,
	repoDir, branchName string,
	conn *repos.Connection,
	maxRetries int,
) error {
	// Get the current remote URL
	getURLCmd := exec.CommandContext(ctx, "git", "-C", repoDir, "remote", "get-url", "origin")

	urlOutput, err := getURLCmd.Output()
	if err != nil {
		return fmt.Errorf("get remote URL: %w", err)
	}

	remoteURL := strings.TrimSpace(string(urlOutput))

	// Add auth to URL for push
	authURL := gitutil.AddAuthToURL(remoteURL, conn)

	// Temporarily set authenticated URL
	setURLCmd := exec.CommandContext(
		ctx,
		"git",
		"-C",
		repoDir,
		"remote",
		"set-url",
		"origin",
		authURL,
	)
	if output, err := setURLCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("set remote URL: %w\nOutput: %s", err, string(output))
	}

	// Ensure we reset the URL after push (security)
	defer func() {
		resetCmd := exec.CommandContext(
			context.Background(),
			"git",
			"-C",
			repoDir,
			"remote",
			"set-url",
			"origin",
			remoteURL,
		)
		_ = resetCmd.Run()
	}()

	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Create a timeout context for push operation (use configured timeout)
		pushCtx, cancel := context.WithTimeout(ctx, s.pushTimeout)

		cmd := exec.CommandContext(
			pushCtx,
			"git",
			"-C",
			repoDir,
			"push",
			"-u",
			"origin",
			branchName,
		)
		configureGitCmd(cmd)
		output, err := cmd.CombinedOutput()

		cancel()

		if err == nil {
			return nil
		}

		// Sanitize output to remove credentials
		sanitizedOutput := gitutil.SanitizeGitOutput(string(output), conn)

		// Check for timeout
		if errors.Is(pushCtx.Err(), context.DeadlineExceeded) {
			lastErr = fmt.Errorf(
				"git push timed out after %v (attempt %d/%d)",
				s.pushTimeout,
				attempt,
				maxRetries,
			)
		} else {
			lastErr = fmt.Errorf("git push failed (attempt %d/%d): %w\nOutput: %s", attempt, maxRetries, err, sanitizedOutput)
		}

		// Check if it's a non-retryable error
		if strings.Contains(string(output), "already exists") {
			return fmt.Errorf("branch '%s' already exists on remote: %w", branchName, lastErr)
		}

		if strings.Contains(string(output), "permission denied") ||
			strings.Contains(string(output), "403") {
			return fmt.Errorf("permission denied: %w", lastErr)
		}

		if attempt < maxRetries {
			s.logger.Warn("Push failed, retrying",
				zap.Int("attempt", attempt),
				zap.Int("max_retries", maxRetries),
				zap.String("error", sanitizedOutput),
			)
			time.Sleep(time.Duration(attempt) * 2 * time.Second) // Exponential backoff
		}
	}

	return lastErr
}

// replaceInFile performs the replacement in a single file.
func (s *Service) replaceInFile(filePath string, opts ReplaceOptions) (bool, error) {
	// Get file info for size check and permissions
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return false, err
	}

	// Skip files that are too large (use configured max size)
	if fileInfo.Size() > s.maxFileSize {
		s.logger.Warn("Skipping large file",
			zap.String("file", filePath),
			zap.Int64("size_bytes", fileInfo.Size()),
			zap.Int64("max_size_bytes", s.maxFileSize),
		)

		return false, nil
	}

	// Preserve original file permissions
	originalMode := fileInfo.Mode()

	content, err := os.ReadFile(filePath)
	if err != nil {
		return false, err
	}

	// Skip binary files
	if isBinaryContent(content) {
		s.logger.Debug("Skipping binary file",
			zap.String("file", filePath),
		)

		return false, nil
	}

	var newContent string

	originalContent := string(content)

	// Extract the actual content pattern from query syntax
	// e.g., "repo:foo content:bar file:*.go" -> "bar"
	searchPattern := extractContentPattern(opts.SearchPattern)
	if searchPattern == "" {
		s.logger.Warn("Could not extract search pattern from query",
			zap.String("query", opts.SearchPattern),
		)

		return false, nil
	}

	s.logger.Debug("Extracted search pattern",
		zap.String("original_query", opts.SearchPattern),
		zap.String("extracted_pattern", searchPattern),
	)

	if opts.IsRegex {
		flags := ""
		if !opts.CaseSensitive {
			flags = "(?i)"
		}

		// Use safe regex compilation to prevent ReDoS attacks
		re, err := regexutil.SafeCompile(flags + searchPattern)
		if err != nil {
			return false, fmt.Errorf("invalid regex pattern: %w", err)
		}

		newContent = re.ReplaceAllString(originalContent, opts.ReplaceWith)
	} else {
		if opts.CaseSensitive {
			newContent = strings.ReplaceAll(originalContent, searchPattern, opts.ReplaceWith)
		} else {
			// Case-insensitive string replacement - QuoteMeta makes this safe
			re, err := regexp.Compile("(?i)" + regexp.QuoteMeta(searchPattern))
			if err != nil {
				return false, err
			}

			newContent = re.ReplaceAllString(originalContent, opts.ReplaceWith)
		}
	}

	if newContent == originalContent {
		return false, nil
	}

	// Write with original permissions preserved
	if err := os.WriteFile(filePath, []byte(newContent), originalMode); err != nil {
		return false, err
	}

	return true, nil
}

// isBinaryContent checks if the content appears to be binary.
// Uses the same heuristic as Git: checks for NUL bytes in the first 8KB.
func isBinaryContent(content []byte) bool {
	// Check the first 8KB (same as Git)
	checkSize := min(len(content), 8000)

	// Look for NUL bytes, which indicate binary content
	return bytes.Contains(content[:checkSize], []byte{0})
}

// getCodeHostClient creates a code host client for the given connection.
func (s *Service) getCodeHostClient(
	ctx context.Context,
	connectionID int64,
) (codehost.Client, error) {
	// Get connection details
	conn, err := s.repoService.GetConnection(ctx, connectionID)
	if err != nil {
		return nil, fmt.Errorf("get connection: %w", err)
	}

	if conn == nil {
		return nil, fmt.Errorf("connection not found: %d", connectionID)
	}

	// Create appropriate client based on type
	return codehost.NewClient(conn.Type, conn.URL, conn.Token)
}

// countFileMatches counts total matches across files.
func countFileMatches(files []FileMatch) int {
	count := 0
	for _, f := range files {
		count += f.Matches
	}

	return count
}

// generateRandomSuffix generates a random 6-character hex string.
func generateRandomSuffix() string {
	bytes := make([]byte, 3)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based suffix
		return fmt.Sprintf("%x", time.Now().UnixNano()%0xFFFFFF)
	}

	return hex.EncodeToString(bytes)
}

// sanitizeBranchName removes or replaces invalid characters from a branch name.
func sanitizeBranchName(name string) string {
	var result strings.Builder

	prevWasInvalid := false

	for _, r := range name {
		// Valid characters: alphanumeric, /, -, _, .
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '/' || r == '-' || r == '_' ||
			r == '.' {
			result.WriteRune(r)

			prevWasInvalid = false
		} else if !prevWasInvalid {
			// Replace invalid chars with hyphen (avoid consecutive hyphens)
			result.WriteRune('-')

			prevWasInvalid = true
		}
	}

	cleaned := result.String()
	// Remove leading/trailing hyphens and dots
	cleaned = strings.Trim(cleaned, "-.")
	// Remove consecutive slashes
	for strings.Contains(cleaned, "//") {
		cleaned = strings.ReplaceAll(cleaned, "//", "/")
	}
	// Git doesn't allow branch names ending with .lock
	cleaned = strings.TrimSuffix(cleaned, ".lock")

	if cleaned == "" {
		return "codesearch-replace"
	}

	return cleaned
}

// extractContentPattern extracts the actual content to search for from a query string.
// It uses Zoekt's query parser to reliably separate content patterns from modifiers
// like repo:, file:, lang:, branch:, etc. Falls back to manual extraction when the
// parser cannot handle the input (e.g., glob patterns in file: filters).
//
// Examples:
//   - "destroyer" -> "destroyer"
//   - "content:destroyer" -> "destroyer"
//   - "repo:^foo$ content:destroyer file:*.md" -> "destroyer"
//   - "repo:foo bar" -> "bar" (words without prefix are content)
//   - "\"exact phrase\"" -> "exact phrase"
func extractContentPattern(queryStr string) string {
	queryStr = strings.TrimSpace(queryStr)
	if queryStr == "" {
		return ""
	}

	parsed, err := query.Parse(queryStr)
	if err != nil {
		// Zoekt parser failed (e.g., invalid regex in file: pattern).
		// Fall back to manual extraction.
		return extractContentPatternManual(queryStr)
	}

	patterns, hasFilters := extractContentFromQuery(parsed)

	// When the query contains filter nodes (repo:, file:, lang:, etc.),
	// words like "and", "not", "or" are likely boolean operators the user
	// typed between filters, not content to search for. Zoekt's parser
	// treats them as literal substrings, so we strip them here.
	if hasFilters {
		patterns = filterBooleanOperators(patterns)
	}

	if len(patterns) == 0 {
		return queryStr
	}

	return strings.Join(patterns, " ")
}

// booleanOperators are words stripped from extracted content when the query
// also contains filter nodes (repo:, file:, lang:, etc.).
var booleanOperators = map[string]bool{
	"and": true, "or": true, "not": true,
	"AND": true, "OR": true, "NOT": true,
}

// filterBooleanOperators removes common boolean operator words from content patterns.
func filterBooleanOperators(patterns []string) []string {
	var filtered []string

	for _, p := range patterns {
		if !booleanOperators[p] {
			filtered = append(filtered, p)
		}
	}

	return filtered
}

// extractContentFromQuery recursively walks a parsed Zoekt query tree and
// collects content patterns (i.e. substring/regexp nodes that are not file-only).
// It also returns whether filter nodes (repo, file, language, branch) were found.
func extractContentFromQuery(q query.Q) (patterns []string, hasFilters bool) {
	switch n := q.(type) {
	case *query.Substring:
		if n.FileName {
			return nil, true
		}

		if n.Pattern != "" {
			return []string{n.Pattern}, false
		}

		return nil, false
	case *query.Regexp:
		if n.FileName {
			return nil, true
		}

		if n.Regexp != nil {
			return []string{n.Regexp.String()}, false
		}

		return nil, false
	case *query.And:
		var all []string

		filters := false

		for _, child := range n.Children {
			p, f := extractContentFromQuery(child)
			all = append(all, p...)
			filters = filters || f
		}

		return all, filters
	case *query.Or:
		var all []string

		filters := false

		for _, child := range n.Children {
			p, f := extractContentFromQuery(child)
			all = append(all, p...)
			filters = filters || f
		}

		return all, filters
	case *query.Symbol:
		return extractContentFromQuery(n.Expr)
	case *query.Type:
		return extractContentFromQuery(n.Child)
	case *query.Boost:
		return extractContentFromQuery(n.Child)
	case *query.Not:
		// Skip negated patterns — they don't represent content to find.
		// But the presence of Not may indicate filter usage.
		_, f := extractContentFromQuery(n.Child)
		return nil, f
	case *query.Repo, *query.Language, *query.Branch,
		*query.RepoRegexp, *query.BranchesRepos, *query.RepoIDs, *query.RepoSet:
		return nil, true
	case *query.Const, *query.FileNameSet, query.RawConfig:
		return nil, false
	default:
		return nil, false
	}
}

// knownModifierPrefixes are query modifier prefixes to strip during manual extraction.
var knownModifierPrefixes = []string{
	"repo:", "file:", "lang:", "language:", "case:", "sym:",
	"branch:", "rev:", "path:", "content:", "-repo:", "-file:",
	"-lang:", "-branch:",
}

// extractContentPatternManual is the fallback used when zoekt's parser cannot
// handle the input (e.g., glob syntax in file: patterns).
func extractContentPatternManual(q string) string {
	// Check for explicit content: prefix first
	if idx := strings.Index(strings.ToLower(q), "content:"); idx != -1 {
		rest := q[idx+len("content:"):]
		// Handle quoted content
		if strings.HasPrefix(rest, "\"") {
			if end := strings.Index(rest[1:], "\""); end != -1 {
				return rest[1 : end+1]
			}
		}
		// Take until next space or modifier
		if before, _, ok := strings.Cut(rest, " "); ok {
			return before
		}

		return rest
	}

	// Split by spaces, skip modifiers and boolean operators
	var parts []string

	for word := range strings.FieldsSeq(q) {
		// Strip surrounding parens for prefix detection
		stripped := strings.Trim(word, "()")
		if stripped == "" {
			continue
		}

		if booleanOperators[stripped] {
			continue
		}

		isModifier := false

		lower := strings.ToLower(stripped)
		for _, prefix := range knownModifierPrefixes {
			if strings.HasPrefix(lower, prefix) {
				isModifier = true
				break
			}
		}

		if isModifier {
			continue
		}
		// Strip quotes and parens from actual content
		word = strings.Trim(word, "()\"")
		if word != "" {
			parts = append(parts, word)
		}
	}

	if len(parts) == 0 {
		return q
	}

	return strings.Join(parts, " ")
}
