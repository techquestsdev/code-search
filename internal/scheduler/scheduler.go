package scheduler

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/aanogueira/code-search/internal/crypto"
	"github.com/aanogueira/code-search/internal/db"
	"github.com/aanogueira/code-search/internal/gitutil"
	"github.com/aanogueira/code-search/internal/metrics"
	"github.com/aanogueira/code-search/internal/queue"
	"github.com/aanogueira/code-search/internal/repos"
	"github.com/aanogueira/code-search/internal/scip"
)

// Config holds scheduler configuration.
type Config struct {
	// Enabled controls whether the scheduler runs
	Enabled bool

	// DefaultPollInterval is the default interval between sync checks
	DefaultPollInterval time.Duration

	// CheckInterval is how often the scheduler checks for repos needing sync
	CheckInterval time.Duration

	// StaleThreshold is how long a repo must be in 'indexing' or 'cloning' status
	// before it's considered stuck and reset to 'pending'. Default: 1 hour.
	StaleThreshold time.Duration

	// PendingJobTimeout is how long a repo can be in 'pending' status without
	// an active job before it's considered stuck and re-queued. Default: 5 minutes.
	// This should be short to quickly recover from job queue issues.
	PendingJobTimeout time.Duration

	// MaxConcurrentChecks limits parallel git fetch checks
	MaxConcurrentChecks int

	// ReposPath is the path where repositories are cloned
	ReposPath string

	// IndexPath is the path where Zoekt index shards are stored
	IndexPath string

	// JobRetentionPeriod is how long to keep completed/failed jobs
	JobRetentionPeriod time.Duration

	// OrphanCleanupInterval is how often to check for orphan shards (0 to disable)
	OrphanCleanupInterval time.Duration
}

const (
	leaderKey     = "codesearch:scheduler:leader"
	leaderLeaseTTL = 30 * time.Second
	leaderRefresh  = 10 * time.Second
)

// Scheduler handles automatic repository sync polling.
type Scheduler struct {
	cfg            Config
	pool           db.Pool
	queue          *queue.Queue
	redisClient    *redis.Client
	reposService   *repos.Service
	scipService    *scip.Service
	tokenEncryptor *crypto.TokenEncryptor
	logger         *zap.Logger
	instanceID     string

	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
}

// SetSCIPService sets the SCIP service for orphan SCIP database cleanup.
func (s *Scheduler) SetSCIPService(svc *scip.Service) {
	s.scipService = svc
}

// New creates a new scheduler.
func New(
	cfg Config,
	pool db.Pool,
	q *queue.Queue,
	redisClient *redis.Client,
	tokenEncryptor *crypto.TokenEncryptor,
	logger *zap.Logger,
) *Scheduler {
	// Generate a unique instance ID for leader election
	instanceID := os.Getenv("HOSTNAME")
	if instanceID == "" {
		instanceID = uuid.New().String()
	}

	return &Scheduler{
		cfg:            cfg,
		pool:           pool,
		queue:          q,
		redisClient:    redisClient,
		reposService:   repos.NewService(pool, tokenEncryptor),
		tokenEncryptor: tokenEncryptor,
		logger:         logger.With(zap.String("component", "scheduler"), zap.String("instance", instanceID)),
		instanceID:     instanceID,
	}
}

// markIndexJobInactive marks an index job as inactive, logging any errors at debug level.
// This is a best-effort cleanup operation.
func (s *Scheduler) markIndexJobInactive(ctx context.Context, repoID int64) {
	if err := s.queue.MarkIndexJobInactive(ctx, repoID); err != nil {
		s.logger.Debug("Failed to mark index job inactive",
			zap.Int64("repo_id", repoID),
			zap.Error(err),
		)
	}
}

// Start begins the scheduler loop.
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()

	if s.running {
		s.mu.Unlock()
		return errors.New("scheduler already running")
	}

	s.running = true
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	if !s.cfg.Enabled {
		s.logger.Info("Scheduler is disabled")
		return nil
	}

	s.logger.Info("Starting scheduler",
		zap.Duration("poll_interval", s.cfg.DefaultPollInterval),
		zap.Duration("check_interval", s.cfg.CheckInterval),
		zap.Duration("stale_threshold", s.cfg.StaleThreshold),
		zap.Int("max_concurrent_checks", s.cfg.MaxConcurrentChecks),
	)

	go s.run(ctx)

	return nil
}

// Stop halts the scheduler.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	close(s.stopCh)
	s.running = false
	s.logger.Info("Scheduler stopped")
}

// tryBecomeLeader attempts to acquire the scheduler leader lock.
// Returns true if this instance is now the leader.
func (s *Scheduler) tryBecomeLeader(ctx context.Context) bool {
	ok, err := s.redisClient.SetNX(ctx, leaderKey, s.instanceID, leaderLeaseTTL).Result()
	if err != nil {
		s.logger.Debug("Leader election Redis error", zap.Error(err))
		return false
	}

	return ok
}

// isLeader checks if this instance currently holds the leader lock.
func (s *Scheduler) isLeader(ctx context.Context) bool {
	val, err := s.redisClient.Get(ctx, leaderKey).Result()
	if err != nil {
		return false
	}

	return val == s.instanceID
}

// refreshLease extends the leader lease TTL.
func (s *Scheduler) refreshLease(ctx context.Context) bool {
	// Only refresh if we still own the lock (compare-and-extend via Lua)
	script := redis.NewScript(`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("PEXPIRE", KEYS[1], ARGV[2])
		else
			return 0
		end
	`)

	result, err := script.Run(ctx, s.redisClient, []string{leaderKey},
		s.instanceID, int(leaderLeaseTTL.Milliseconds())).Int()
	if err != nil {
		s.logger.Warn("Failed to refresh leader lease", zap.Error(err))
		return false
	}

	return result == 1
}

// releaseLeadership releases the leader lock if this instance holds it.
func (s *Scheduler) releaseLeadership(ctx context.Context) {
	// Only delete if we own the key (atomic compare-and-delete via Lua)
	script := redis.NewScript(`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		else
			return 0
		end
	`)

	_, _ = script.Run(ctx, s.redisClient, []string{leaderKey}, s.instanceID).Result()
}

// run is the main scheduler loop with leader election.
func (s *Scheduler) run(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.CheckInterval)
	defer ticker.Stop()

	// Leader election ticker - check/refresh every 10 seconds
	leaderTicker := time.NewTicker(leaderRefresh)
	defer leaderTicker.Stop()

	// Cleanup ticker - run every 10 minutes
	cleanupTicker := time.NewTicker(10 * time.Minute)
	defer cleanupTicker.Stop()

	// Orphan shard cleanup ticker - only if configured
	var orphanTicker *time.Ticker
	if s.cfg.OrphanCleanupInterval > 0 && s.cfg.IndexPath != "" {
		orphanTicker = time.NewTicker(s.cfg.OrphanCleanupInterval)
		defer orphanTicker.Stop()

		s.logger.Info("Orphan shard cleanup enabled",
			zap.Duration("interval", s.cfg.OrphanCleanupInterval),
			zap.String("index_path", s.cfg.IndexPath),
		)
	}

	// Try to become leader immediately
	isLeader := s.tryBecomeLeader(ctx)
	if isLeader {
		s.logger.Info("Acquired scheduler leadership")
		s.checkAndScheduleRepos(ctx)
		s.cleanupOldJobs(ctx)
	} else {
		s.logger.Info("Scheduler running in standby mode (another instance is leader)")
	}

	defer s.releaseLeadership(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-leaderTicker.C:
			if isLeader {
				// Refresh lease
				if !s.refreshLease(ctx) {
					s.logger.Warn("Lost scheduler leadership")
					isLeader = false
				}
			} else {
				// Try to acquire leadership
				if s.tryBecomeLeader(ctx) {
					s.logger.Info("Acquired scheduler leadership")
					isLeader = true
					// Run immediately after becoming leader
					s.checkAndScheduleRepos(ctx)
					s.cleanupOldJobs(ctx)
				}
			}
		case <-ticker.C:
			if isLeader {
				s.checkAndScheduleRepos(ctx)
			}
		case <-cleanupTicker.C:
			if isLeader {
				s.cleanupOldJobs(ctx)
			}
		case <-func() <-chan time.Time {
			if orphanTicker != nil {
				return orphanTicker.C
			}
			return make(chan time.Time)
		}():
			if isLeader {
				s.cleanupOrphanShards(ctx)
			}
		}
	}
}

// cleanupOldJobs removes old completed/failed jobs from the queue.
// It also cleans up repositories stuck in 'indexing' or 'cloning' status for too long.
func (s *Scheduler) cleanupOldJobs(ctx context.Context) {
	s.logger.Debug("Running job cleanup", zap.Duration("retention", s.cfg.JobRetentionPeriod))

	if s.cfg.JobRetentionPeriod <= 0 {
		s.logger.Debug("Job cleanup skipped - retention period is 0 or negative")
		return
	}

	result, err := s.queue.CleanupOldJobs(ctx, s.cfg.JobRetentionPeriod)
	if err != nil {
		s.logger.Error("Failed to cleanup old jobs", zap.Error(err))
		return
	}

	if result.DeletedCount > 0 {
		s.logger.Info("Cleaned up old jobs",
			zap.Int("deleted", result.DeletedCount),
			zap.Int("scanned", result.ScannedCount),
			zap.Duration("retention", s.cfg.JobRetentionPeriod),
		)
	} else {
		s.logger.Debug("Job cleanup complete",
			zap.Int("scanned", result.ScannedCount),
			zap.Int("deleted", 0),
			zap.Duration("retention", s.cfg.JobRetentionPeriod),
		)
	}

	// Cleanup repositories stuck in 'indexing' or 'cloning' status
	// Default threshold: 1 hour - repos stuck for more than 1 hour are considered stale
	staleThreshold := 1 * time.Hour
	if s.cfg.StaleThreshold > 0 {
		staleThreshold = s.cfg.StaleThreshold
	}

	count, err := s.reposService.CleanupStaleIndexing(ctx, staleThreshold)
	if err != nil {
		s.logger.Error("Failed to cleanup stale indexing", zap.Error(err))
	} else if count > 0 {
		s.logger.Info("Reset stale indexing repos",
			zap.Int64("count", count),
			zap.Duration("threshold", staleThreshold),
		)
	}

	// Recover orphaned active index markers
	s.recoverOrphanedActiveJobs(ctx)
}

// recoverOrphanedActiveJobs detects and removes orphaned entries in the active index set.
// This happens when the indexer crashes or restarts without properly cleaning up the active set.
func (s *Scheduler) recoverOrphanedActiveJobs(ctx context.Context) {
	s.logger.Debug("Running orphaned active jobs recovery")

	// Get all repo IDs marked as active
	activeRepos, err := s.queue.GetActiveIndexRepos(ctx)
	if err != nil {
		s.logger.Error("Failed to get active index repos", zap.Error(err))
		return
	}

	// Record metric for current active repos count
	metrics.SetActiveIndexRepos(len(activeRepos))

	if len(activeRepos) == 0 {
		s.logger.Debug("No active index jobs to check")
		return
	}

	s.logger.Debug("Checking active index jobs for orphans", zap.Int("active_count", len(activeRepos)))

	// Check each repo to see if it has an actual pending/running job
	orphanCount := 0
	for _, repoID := range activeRepos {
		hasJob, err := s.queue.HasActiveIndexJob(ctx, repoID)
		if err != nil {
			s.logger.Warn("Failed to check for active job",
				zap.Int64("repo_id", repoID),
				zap.Error(err),
			)
			continue
		}

		if !hasJob {
			// This repo is marked as active but has no pending/running job - it's orphaned
			if err := s.queue.MarkIndexJobInactive(ctx, repoID); err != nil {
				s.logger.Warn("Failed to remove orphaned active marker",
					zap.Int64("repo_id", repoID),
					zap.Error(err),
				)
			} else {
				s.logger.Info("Removed orphaned active index marker",
					zap.Int64("repo_id", repoID),
				)
				orphanCount++
				metrics.RecordOrphanedActiveMarkerRecovered()
			}
		}
	}

	if orphanCount > 0 {
		s.logger.Info("Orphaned active jobs recovery complete",
			zap.Int("orphans_removed", orphanCount),
			zap.Int("active_checked", len(activeRepos)),
		)
	} else {
		s.logger.Debug("No orphaned active jobs found",
			zap.Int("active_checked", len(activeRepos)),
		)
	}
}

// cleanupOrphanShards finds and removes Zoekt shard files that don't have
// matching repositories in the database. This can happen when:
// 1. Repositories are deleted directly from the database
// 2. Cleanup jobs fail to complete
// 3. Manual intervention leaves stale shards.
func (s *Scheduler) cleanupOrphanShards(ctx context.Context) {
	if s.cfg.IndexPath == "" {
		s.logger.Debug("Orphan shard cleanup skipped - no index path configured")
		return
	}

	s.logger.Debug("Running orphan shard cleanup", zap.String("index_path", s.cfg.IndexPath))

	// Count and record orphaned .tmp files
	tmpFiles, err := filepath.Glob(filepath.Join(s.cfg.IndexPath, "*.tmp"))
	if err != nil {
		s.logger.Warn("Failed to glob temp files", zap.Error(err))
	} else {
		metrics.SetOrphanedTempFiles(len(tmpFiles))
		if len(tmpFiles) > 0 {
			s.logger.Info("Found orphaned temp index files",
				zap.Int("count", len(tmpFiles)),
				zap.String("index_path", s.cfg.IndexPath),
			)
		}
	}

	// Get all shard files
	shardFiles, err := filepath.Glob(filepath.Join(s.cfg.IndexPath, "*.zoekt"))
	if err != nil {
		s.logger.Error("Failed to glob shard files", zap.Error(err))
		return
	}

	if len(shardFiles) == 0 {
		s.logger.Debug("No shard files found")
		return
	}

	// Get all repo names from the database (non-excluded only)
	allRepos, err := s.reposService.ListRepositories(ctx, nil)
	if err != nil {
		s.logger.Error("Failed to list repositories", zap.Error(err))
		return
	}

	// Build a set of valid repo names in multiple formats (non-excluded repos)
	// Zoekt can use different naming conventions:
	// 1. URL-encoded: gitlab.molops.io%2Fapps%2Frepo_v16.00000.zoekt
	// 2. Underscore-replaced: owner_repo_v16.00000.zoekt
	validRepoNames := make(map[string]bool)

	for _, repo := range allRepos {
		if !repo.Excluded {
			// Add the original name
			validRepoNames[repo.Name] = true
			// Add URL-encoded version (slashes become %2F)
			validRepoNames[url.PathEscape(repo.Name)] = true
			// Add underscore-replaced version (legacy format)
			validRepoNames[strings.ReplaceAll(repo.Name, "/", "_")] = true
		}
	}

	s.logger.Debug("Checking for orphan shards",
		zap.Int("shard_files", len(shardFiles)),
		zap.Int("valid_repos", len(validRepoNames)),
	)

	// Check each shard file
	orphanCount := 0
	removedCount := 0

	for _, shardFile := range shardFiles {
		baseName := filepath.Base(shardFile)

		// Skip compound shards - they may contain multiple repos
		if strings.HasPrefix(baseName, "compound_") {
			continue
		}

		// Extract repo name from shard file name
		// Shard files are named like: "owner_repo_v16.00000.zoekt"
		// We need to match the prefix before "_v" followed by version number
		repoName := extractRepoNameFromShard(baseName)
		if repoName == "" {
			s.logger.Debug("Could not extract repo name from shard", zap.String("file", baseName))
			continue
		}

		// Check if this repo exists in the database
		if !validRepoNames[repoName] {
			orphanCount++

			s.logger.Info("Found orphan shard",
				zap.String("file", baseName),
				zap.String("repo_name", repoName),
			)

			// Remove the orphan shard
			if err := os.Remove(shardFile); err != nil && !os.IsNotExist(err) {
				s.logger.Warn("Failed to remove orphan shard",
					zap.String("file", baseName),
					zap.Error(err),
				)
			} else {
				removedCount++

				s.logger.Info("Removed orphan shard", zap.String("file", baseName))
			}
		}
	}

	if orphanCount > 0 {
		s.logger.Info("Orphan shard cleanup complete",
			zap.Int("orphans_found", orphanCount),
			zap.Int("orphans_removed", removedCount),
		)
	} else {
		s.logger.Debug("No orphan shards found")
	}

	// Clean up orphan SCIP databases (repos deleted but SCIP files remain)
	if s.scipService != nil {
		s.cleanupOrphanSCIPDatabases(ctx, validRepoNames, allRepos)
	}
}

// cleanupOrphanSCIPDatabases removes SCIP database files whose repo IDs no
// longer map to a valid (non-excluded) repository.
func (s *Scheduler) cleanupOrphanSCIPDatabases(ctx context.Context, _ map[string]bool, allRepos []repos.Repository) {
	scipIDs, err := s.scipService.ListIndexedRepoIDs()
	if err != nil {
		s.logger.Warn("Failed to list SCIP indexed repo IDs", zap.Error(err))
		return
	}

	if len(scipIDs) == 0 {
		return
	}

	// Build set of valid repo IDs (non-excluded)
	validIDs := make(map[int64]bool, len(allRepos))
	for _, r := range allRepos {
		if !r.Excluded {
			validIDs[r.ID] = true
		}
	}

	removed := 0
	for _, id := range scipIDs {
		if !validIDs[id] {
			if err := s.scipService.DeleteIndex(ctx, id); err != nil {
				s.logger.Warn("Failed to delete orphan SCIP index",
					zap.Int64("repo_id", id),
					zap.Error(err),
				)
			} else {
				removed++
				s.logger.Info("Removed orphan SCIP index", zap.Int64("repo_id", id))
			}
		}
	}

	if removed > 0 {
		s.logger.Info("Orphan SCIP cleanup complete", zap.Int("removed", removed))
	}
}

// extractRepoNameFromShard extracts the repository name from a shard file name.
// Shard files can be named in different formats:
// 1. URL-encoded: "gitlab.molops.io%2Fapps%2Frepo_v16.00000.zoekt"
// 2. Underscore: "owner_repo_v16.00000.zoekt"
// Returns the part before the version suffix.
func extractRepoNameFromShard(shardName string) string {
	// Remove .zoekt extension
	name := strings.TrimSuffix(shardName, ".zoekt")

	// Find the version pattern: _v followed by digits
	// The pattern is _v<version>.<shard_number>
	// Example: gitlab.molops.io%2Fapps%2Frepo_v16.00000 -> gitlab.molops.io%2Fapps%2Frepo

	// Split by "_v" and check if the part after looks like a version
	parts := strings.Split(name, "_v")
	if len(parts) < 2 {
		return ""
	}

	// The last part should be the version (e.g., "16.00000")
	lastPart := parts[len(parts)-1]
	if !isVersionSuffix(lastPart) {
		// Not a version pattern, might be part of the repo name
		return ""
	}

	// Join all parts except the last one (the version)
	return strings.Join(parts[:len(parts)-1], "_v")
}

// isVersionSuffix checks if a string looks like a version suffix (e.g., "16.00000").
func isVersionSuffix(s string) bool {
	// Version format: digits.digits (e.g., "16.00000")
	dotIdx := strings.Index(s, ".")
	if dotIdx <= 0 || dotIdx >= len(s)-1 {
		return false
	}

	// Check that both parts are numeric
	for i, part := range []string{s[:dotIdx], s[dotIdx+1:]} {
		for _, c := range part {
			if c < '0' || c > '9' {
				// Allow only digits, but the second part might have additional suffix
				if i == 1 && c == '.' {
					// Handle cases like "16.00000.zoekt" where we already stripped .zoekt
					continue
				}

				return false
			}
		}
	}

	return true
}

// checkAndScheduleRepos finds repos that need syncing and schedules them.
func (s *Scheduler) checkAndScheduleRepos(ctx context.Context) {
	s.logger.Debug("Checking for repositories needing sync")

	// Find repos that need sync based on:
	// 1. Poll interval elapsed since last_indexed
	// 2. Never indexed (pending status for too long)
	repos, err := s.getReposNeedingSync(ctx)
	if err != nil {
		s.logger.Error("Failed to get repos needing sync", zap.Error(err))
		return
	}

	if len(repos) == 0 {
		s.logger.Debug("No repositories need syncing")
		return
	}

	s.logger.Info("Found repositories needing sync", zap.Int("count", len(repos)))

	// Process repos with limited concurrency
	sem := make(chan struct{}, s.cfg.MaxConcurrentChecks)

	var wg sync.WaitGroup

	for _, repo := range repos {
		// capture for goroutine
		wg.Add(1)

		go func() {
			defer wg.Done()

			sem <- struct{}{}

			defer func() { <-sem }()

			s.processRepo(ctx, repo)
		}()
	}

	wg.Wait()
}

// RepoNeedingSync contains info about a repo that may need syncing.
type RepoNeedingSync struct {
	ID            int64
	ConnectionID  int64
	Name          string
	CloneURL      string
	DefaultBranch string
	Branches      []string // Additional branches to index
	IndexStatus   string
	LastIndexed   *time.Time
	PollInterval  *time.Duration // nil means use default
	LocalPath     string
}

// getReposNeedingSync queries for repos that need to be checked for updates.
func (s *Scheduler) getReposNeedingSync(ctx context.Context) ([]RepoNeedingSync, error) {
	// Calculate thresholds
	pollThreshold := time.Now().Add(-s.cfg.DefaultPollInterval)

	// Use dedicated timeout for stuck pending repos (default: 5 minutes)
	// This is separate from StaleThreshold which is for indexing/cloning cleanup
	pendingJobTimeout := 5 * time.Minute
	if s.cfg.PendingJobTimeout > 0 {
		pendingJobTimeout = s.cfg.PendingJobTimeout
	}
	pendingThreshold := time.Now().Add(-pendingJobTimeout)

	// Build database-agnostic timestamp literal
	sqlBuilder := db.NewSQLBuilder(s.pool.Driver())
	epochTimestamp := sqlBuilder.TimestampLiteral("1970-01-01")

	// Find repos where:
	// 1. last_indexed is older than poll_interval (or never indexed)
	// 2. Status is 'indexed' (not currently being processed)
	// 3. OR status is 'pending' for too long (stuck without a job)
	// 4. AND not excluded or deleted
	query := fmt.Sprintf(`
		SELECT
			r.id, r.connection_id, r.name, r.clone_url, r.default_branch, r.branches,
			r.index_status, r.last_indexed,
			COALESCE(r.poll_interval_seconds, $1) as poll_interval_secs
		FROM repositories r
		WHERE
			r.excluded = false
			AND r.deleted = false
			AND (
				(
					-- Repos that need periodic re-sync
					r.index_status = 'indexed'
					AND (r.last_indexed IS NULL OR r.last_indexed < $2)
				)
				OR (
					-- Stuck pending repos (older than pending job timeout)
					-- These repos are in 'pending' but likely don't have a job in the queue
					r.index_status = 'pending'
					AND r.updated_at < $3
				)
			)
		ORDER BY COALESCE(r.last_indexed, %s)
		LIMIT 100
	`, epochTimestamp)

	rows, err := s.pool.Query(
		ctx,
		query,
		int(s.cfg.DefaultPollInterval.Seconds()),
		pollThreshold,
		pendingThreshold,
	)
	if err != nil {
		return nil, fmt.Errorf("query repos: %w", err)
	}
	defer rows.Close()

	var repos []RepoNeedingSync

	for rows.Next() {
		var (
			r                RepoNeedingSync
			pollIntervalSecs int
			branches         db.StringArray
		)

		err := rows.Scan(
			&r.ID, &r.ConnectionID, &r.Name, &r.CloneURL, &r.DefaultBranch, &branches,
			&r.IndexStatus, &r.LastIndexed, &pollIntervalSecs,
		)
		if err != nil {
			return nil, fmt.Errorf("scan repo: %w", err)
		}

		r.Branches = []string(branches)
		pollInterval := time.Duration(pollIntervalSecs) * time.Second
		r.PollInterval = &pollInterval

		// Calculate local path
		safeName := strings.ReplaceAll(r.Name, "/", "_")
		r.LocalPath = filepath.Join(s.cfg.ReposPath, safeName+".git")

		repos = append(repos, r)
	}

	return repos, rows.Err()
}

// processRepo checks if a repo has updates and schedules re-indexing if needed.
func (s *Scheduler) processRepo(ctx context.Context, repo RepoNeedingSync) {
	logger := s.logger.With(zap.Int64("repo_id", repo.ID), zap.String("repo_name", repo.Name))

	// Check if repo has new commits
	hasUpdates, err := s.checkForUpdates(ctx, repo)
	if err != nil {
		logger.Warn("Failed to check for updates", zap.Error(err))
		// Schedule anyway if we can't check
		hasUpdates = true
	}

	if !hasUpdates {
		logger.Debug("No updates found, skipping re-index")
		// Update last_indexed to current time to reset the poll interval
		// This prevents checking the same repo repeatedly when there are no changes
		err := s.touchLastIndexed(ctx, repo.ID)
		if err != nil {
			logger.Warn("Failed to update last_indexed", zap.Error(err))
		}

		return
	}

	logger.Info("Updates found, scheduling re-index")

	// Schedule for re-indexing
	if err := s.scheduleIndex(ctx, repo); err != nil {
		logger.Error("Failed to schedule index", zap.Error(err))
		return
	}
}

// checkForUpdates does a git fetch to see if there are new commits.
func (s *Scheduler) checkForUpdates(ctx context.Context, repo RepoNeedingSync) (bool, error) {
	// If repo doesn't exist locally, it definitely needs indexing
	if repo.LocalPath == "" {
		return true, nil
	}

	// Get the connection for auth
	conn, err := s.reposService.GetConnection(ctx, repo.ConnectionID)
	if err != nil {
		return false, fmt.Errorf("get connection: %w", err)
	}

	// Get current HEAD before fetch
	headBefore, err := s.getGitHead(ctx, repo.LocalPath, repo.DefaultBranch)
	if err != nil {
		// Repo might not exist yet
		return true, nil
	}

	// Do a git fetch (dry-run style - we just want to see if there are updates)
	if err := s.gitFetch(ctx, repo.LocalPath, conn); err != nil {
		return false, fmt.Errorf("git fetch: %w", err)
	}

	// Get HEAD after fetch
	headAfter, err := s.getGitHead(ctx, repo.LocalPath, repo.DefaultBranch)
	if err != nil {
		return true, nil // Assume updates if we can't check
	}

	// Compare
	return headBefore != headAfter, nil
}

// getGitHead returns the commit hash for a branch.
func (s *Scheduler) getGitHead(ctx context.Context, repoPath, branch string) (string, error) {
	// Try remote ref first (after fetch)
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "rev-parse", "origin/"+branch)

	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output)), nil
	}

	// Fall back to local ref
	cmd = exec.CommandContext(ctx, "git", "-C", repoPath, "rev-parse", branch)

	output, err = cmd.Output()
	if err != nil {
		return "", fmt.Errorf("rev-parse %s: %w", branch, err)
	}

	return strings.TrimSpace(string(output)), nil
}

// gitFetch fetches updates from the remote.
func (s *Scheduler) gitFetch(ctx context.Context, repoPath string, conn *repos.Connection) error {
	// Get current remote URL
	getURLCmd := exec.CommandContext(ctx, "git", "-C", repoPath, "remote", "get-url", "origin")

	urlOutput, err := getURLCmd.Output()
	if err != nil {
		return fmt.Errorf("get remote url: %w", err)
	}

	remoteURL := strings.TrimSpace(string(urlOutput))
	authURL := gitutil.AddAuthToURL(remoteURL, conn)

	// Temporarily set authenticated URL
	if authURL != remoteURL {
		setURLCmd := exec.CommandContext(
			ctx,
			"git",
			"-C",
			repoPath,
			"remote",
			"set-url",
			"origin",
			authURL,
		)

		err := setURLCmd.Run()
		if err != nil {
			return fmt.Errorf("set auth url: %w", err)
		}

		defer func() {
			// Reset to original URL (without token in it)
			resetCmd := exec.CommandContext(
				ctx,
				"git",
				"-C",
				repoPath,
				"remote",
				"set-url",
				"origin",
				remoteURL,
			)
			if err := resetCmd.Run(); err != nil {
				s.logger.Debug("Failed to reset remote URL",
					zap.String("repo_path", repoPath),
					zap.Error(err),
				)
			}
		}()
	}

	// Fetch with timeout
	fetchCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(fetchCtx, "git", "-C", repoPath, "fetch", "--all", "--prune")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git fetch: %w, output: %s", err, string(output))
	}

	return nil
}

// touchLastIndexed updates the last_indexed timestamp without changing status.
func (s *Scheduler) touchLastIndexed(ctx context.Context, repoID int64) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE repositories
		SET last_indexed = NOW(), updated_at = NOW()
		WHERE id = $1 AND index_status = 'indexed'
	`, repoID)

	return err
}

// scheduleIndex adds a repo to the indexing queue.
func (s *Scheduler) scheduleIndex(ctx context.Context, repo RepoNeedingSync) error {
	// Atomically try to acquire the job slot - this eliminates race conditions
	acquired, err := s.queue.TryAcquireIndexJob(ctx, repo.ID)
	if err != nil {
		s.logger.Warn(
			"Failed to acquire index job slot",
			zap.Int64("repo_id", repo.ID),
			zap.Error(err),
		)
		// Continue anyway - worst case we create a duplicate
	} else if !acquired {
		s.logger.Debug("Skipping - job already pending", zap.Int64("repo_id", repo.ID), zap.String("repo", repo.Name))
		return nil
	}

	// Update status to pending
	_, err = s.pool.Exec(ctx, `
		UPDATE repositories
		SET index_status = 'pending', updated_at = NOW()
		WHERE id = $1
	`, repo.ID)
	if err != nil {
		s.markIndexJobInactive(ctx, repo.ID)
		return fmt.Errorf("update status: %w", err)
	}

	// Create index job payload
	payload := queue.IndexPayload{
		RepositoryID: repo.ID,
		ConnectionID: repo.ConnectionID,
		RepoName:     repo.Name,
		CloneURL:     repo.CloneURL,
		Branch:       repo.DefaultBranch,
		Branches:     repo.Branches, // Include configured branches for multi-branch indexing
	}

	// Enqueue the job
	_, err = s.queue.Enqueue(ctx, queue.JobTypeIndex, payload)
	if err != nil {
		// Rollback active marker on failure
		s.markIndexJobInactive(ctx, repo.ID)
		return fmt.Errorf("enqueue job: %w", err)
	}

	return nil
}

// ManualSync triggers an immediate sync check for a specific repo.
func (s *Scheduler) ManualSync(ctx context.Context, repoID int64) error {
	var repo RepoNeedingSync

	err := s.pool.QueryRow(ctx, `
		SELECT
			r.id, r.connection_id, r.name, r.clone_url, r.default_branch,
			r.index_status, r.last_indexed
		FROM repositories r
		WHERE r.id = $1
	`, repoID).Scan(
		&repo.ID, &repo.ConnectionID, &repo.Name, &repo.CloneURL, &repo.DefaultBranch,
		&repo.IndexStatus, &repo.LastIndexed,
	)
	if err != nil {
		return fmt.Errorf("get repo: %w", err)
	}

	// Calculate local path
	safeName := strings.ReplaceAll(repo.Name, "/", "_")
	repo.LocalPath = filepath.Join(s.cfg.ReposPath, safeName+".git")

	// Force schedule without checking for updates
	s.logger.Info(
		"Manual sync requested",
		zap.Int64("repo_id", repoID),
		zap.String("repo_name", repo.Name),
	)

	return s.scheduleIndex(ctx, repo)
}

// SetPollInterval sets a custom poll interval for a repository.
func (s *Scheduler) SetPollInterval(
	ctx context.Context,
	repoID int64,
	interval time.Duration,
) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE repositories
		SET poll_interval_seconds = $2, updated_at = NOW()
		WHERE id = $1
	`, repoID, int(interval.Seconds()))

	return err
}

// GetStats returns scheduler statistics.
func (s *Scheduler) GetStats(ctx context.Context) (*Stats, error) {
	var stats Stats

	// Get counts by status
	rows, err := s.pool.Query(ctx, `
		SELECT index_status, COUNT(*)
		FROM repositories
		GROUP BY index_status
	`)
	if err != nil {
		return nil, fmt.Errorf("query stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			status string
			count  int
		)

		err := rows.Scan(&status, &count)
		if err != nil {
			return nil, fmt.Errorf("scan stats: %w", err)
		}

		switch status {
		case "indexed":
			stats.IndexedCount = count
		case "pending":
			stats.PendingCount = count
		case "indexing":
			stats.IndexingCount = count
		case "failed":
			stats.FailedCount = count
		}
	}

	// Get stale repo count
	staleThreshold := time.Now().Add(-s.cfg.DefaultPollInterval)

	err = s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM repositories
		WHERE index_status = 'indexed'
		AND (last_indexed IS NULL OR last_indexed < $1)
	`, staleThreshold).Scan(&stats.StaleCount)
	if err != nil {
		return nil, fmt.Errorf("query stale count: %w", err)
	}

	stats.TotalCount = stats.IndexedCount + stats.PendingCount + stats.IndexingCount + stats.FailedCount
	stats.NextCheckAt = time.Now().Add(s.cfg.CheckInterval)

	return &stats, nil
}

// Stats holds scheduler statistics.
type Stats struct {
	TotalCount    int       `json:"total_count"`
	IndexedCount  int       `json:"indexed_count"`
	PendingCount  int       `json:"pending_count"`
	IndexingCount int       `json:"indexing_count"`
	FailedCount   int       `json:"failed_count"`
	StaleCount    int       `json:"stale_count"`
	NextCheckAt   time.Time `json:"next_check_at"`
}
