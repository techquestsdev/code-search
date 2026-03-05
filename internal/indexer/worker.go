package indexer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"

	"github.com/techquestsdev/code-search/internal/codehost"
	"github.com/techquestsdev/code-search/internal/config"
	"github.com/techquestsdev/code-search/internal/crypto"
	"github.com/techquestsdev/code-search/internal/db"
	"github.com/techquestsdev/code-search/internal/languages"
	scipkg "github.com/techquestsdev/code-search/internal/scip"
	"github.com/techquestsdev/code-search/internal/gitutil"
	"github.com/techquestsdev/code-search/internal/lock"
	"github.com/techquestsdev/code-search/internal/metrics"
	"github.com/techquestsdev/code-search/internal/queue"
	"github.com/techquestsdev/code-search/internal/replace"
	"github.com/techquestsdev/code-search/internal/repos"
	"github.com/techquestsdev/code-search/internal/search"
	"github.com/techquestsdev/code-search/internal/tracing"
)

// codeHostAdapter adapts codehost.Client to repos.CodeHostClient.
type codeHostAdapter struct {
	client codehost.Client
}

func (a *codeHostAdapter) ListRepositories(
	ctx context.Context,
) ([]repos.CodeHostRepository, error) {
	remoteRepos, err := a.client.ListRepositories(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]repos.CodeHostRepository, len(remoteRepos))
	for i, r := range remoteRepos {
		result[i] = repos.CodeHostRepository{
			Name:          r.Name,
			FullName:      r.FullName,
			CloneURL:      r.CloneURL,
			DefaultBranch: r.DefaultBranch,
			Private:       r.Private,
			Archived:      r.Archived,
		}
	}

	return result, nil
}

func (a *codeHostAdapter) GetRepository(
	ctx context.Context,
	name string,
) (*repos.CodeHostRepository, error) {
	repo, err := a.client.GetRepository(ctx, name)
	if err != nil {
		return nil, err
	}

	if repo == nil {
		return nil, nil
	}

	return &repos.CodeHostRepository{
		Name:          repo.Name,
		FullName:      repo.FullName,
		CloneURL:      repo.CloneURL,
		DefaultBranch: repo.DefaultBranch,
		Private:       repo.Private,
		Archived:      repo.Archived,
	}, nil
}

// Worker processes index jobs from the queue.
type Worker struct {
	cfg            *config.Config
	logger         *zap.Logger
	queue          *queue.Queue
	shardedQueue   *queue.ShardedQueue
	reposService   *repos.Service
	replaceService *replace.Service
	shardConfig    ShardConfig
	redisClient    *redis.Client
	workerID       string

	// Graceful shutdown support
	shutdownTimeout time.Duration

	// Optional SCIP service for code intelligence indexing (nil if disabled)
	scipService *scipkg.Service
}

// NewWorker creates a new indexer worker.
func NewWorker(
	cfg *config.Config,
	logger *zap.Logger,
	q *queue.Queue,
	pool db.Pool,
	tokenEncryptor *crypto.TokenEncryptor,
) *Worker {
	shardConfig := GetShardConfig()

	// Create sharded queue wrapper
	shardedQueue := queue.NewShardedQueue(q.Client())

	logger.Info("Worker initialized",
		zap.Bool("shard_enabled", shardConfig.Enabled),
		zap.Int("shard_index", shardConfig.ShardIndex),
		zap.Int("total_shards", shardConfig.TotalShards),
		zap.String("repos_path", cfg.Indexer.ReposPath),
		zap.String("index_path", cfg.Indexer.IndexPath),
		zap.String("zoekt_bin", cfg.Indexer.ZoektBin),
		zap.String("zoekt_url", cfg.Zoekt.URL),
	)

	// Log initial state of directories
	logDirectoryState(logger, "repos", cfg.Indexer.ReposPath)
	logDirectoryState(logger, "index", cfg.Indexer.IndexPath)

	// Create services for replace jobs
	repoService := repos.NewService(pool, tokenEncryptor)
	searchService := search.NewService(cfg.Zoekt.URL)

	// Create replace service with config
	replaceCfg := &replace.ServiceConfig{
		Concurrency:  cfg.Replace.Concurrency,
		CloneTimeout: cfg.Replace.CloneTimeout,
		PushTimeout:  cfg.Replace.PushTimeout,
		MaxFileSize:  cfg.Replace.MaxFileSize,
	}
	replaceService := replace.NewService(
		searchService,
		repoService,
		cfg.Indexer.ReposPath,
		logger,
		replaceCfg,
	)

	workerID := os.Getenv("HOSTNAME")
	if workerID == "" {
		workerID = fmt.Sprintf("indexer-%d-%d", shardConfig.ShardIndex, time.Now().UnixNano())
	}

	return &Worker{
		cfg:             cfg,
		logger:          logger,
		queue:           q,
		shardedQueue:    shardedQueue,
		reposService:    repoService,
		replaceService:  replaceService,
		shardConfig:     shardConfig,
		redisClient:     q.Client(),
		workerID:        workerID,
		shutdownTimeout: 5 * time.Minute, // Allow up to 5 minutes for job to complete
	}
}

// logDirectoryState logs the state of a directory.
func logDirectoryState(logger *zap.Logger, name, path string) {
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Warn(
				"Directory does not exist",
				zap.String("name", name),
				zap.String("path", path),
			)
		} else {
			logger.Error("Failed to read directory", zap.String("name", name), zap.String("path", path), zap.Error(err))
		}

		return
	}

	logger.Info("Directory state",
		zap.String("name", name),
		zap.String("path", path),
		zap.Int("entries", len(entries)),
	)
}

// SetSCIPService sets the optional SCIP service for code intelligence indexing.
func (w *Worker) SetSCIPService(svc *scipkg.Service) {
	w.scipService = svc
}

// standaloneLanguages are languages whose SCIP indexers work without project-specific
// build setup. They default to enabled when scip.enabled=true and the binary is found.
var standaloneLanguages = map[string]bool{
	"go":         true,
	"typescript": true,
	"javascript": true,
	"python":     true,
}

// detectLanguagesFromBareRepo detects all languages present in a bare git repo
// by checking for marker files via `git ls-tree` without a full checkout.
// It searches recursively so that projects in subdirectories (monorepos) are detected.
// Languages are returned in priority order.
func (w *Worker) detectLanguagesFromBareRepo(ctx context.Context, repoPath string) []string {
	cmd := exec.CommandContext(ctx, "git", "ls-tree", "-r", "--name-only", "HEAD")
	cmd.Dir = repoPath

	output, err := cmd.Output()
	if err != nil {
		w.logger.Debug("Failed to list files in bare repo for language detection",
			zap.String("repo_path", repoPath),
			zap.Error(err),
		)
		return nil
	}

	// Collect basenames of all files (supports subdirectory detection)
	basenames := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			basenames[filepath.Base(trimmed)] = true
		}
	}

	// Check in priority order (most specific first)
	var detected []string

	for _, lang := range languages.All {
		for _, marker := range lang.Markers {
			if basenames[marker] {
				detected = append(detected, lang.Name)
				break
			}
		}
	}

	return detected
}

// isSCIPLanguageEnabled checks whether SCIP indexing is enabled for a language.
// If the language has explicit config, that config is used.
// Otherwise, standalone languages default to enabled and build-dependent languages default to disabled.
func (w *Worker) isSCIPLanguageEnabled(language string) bool {
	if w.cfg.SCIP.Languages != nil {
		if langCfg, ok := w.cfg.SCIP.Languages[language]; ok {
			return langCfg.Enabled
		}
	}

	// Default: standalone languages are enabled, build-dependent are disabled
	return standaloneLanguages[language]
}

// runSCIPIndexing runs SCIP code intelligence indexing after successful Zoekt indexing.
// It detects all languages in the repo and indexes each enabled/available one.
// This is non-fatal: errors are logged as warnings but never fail the index job.
func (w *Worker) runSCIPIndexing(ctx context.Context, repoID int64, repoPath, repoName string, conn *repos.Connection) {
	if w.scipService == nil || !w.cfg.SCIP.Enabled || !w.cfg.SCIP.AutoIndex {
		return
	}

	logger := w.logger.With(
		zap.Int64("repo_id", repoID),
		zap.String("repo_name", repoName),
		zap.String("component", "scip"),
	)

	// Detect all languages from the bare repo
	allLanguages := w.detectLanguagesFromBareRepo(ctx, repoPath)
	if len(allLanguages) == 0 {
		logger.Debug("No supported SCIP language detected, skipping")
		return
	}

	// Filter to enabled and available languages
	var enabledLangs []string
	for _, lang := range allLanguages {
		if !w.isSCIPLanguageEnabled(lang) {
			logger.Debug("SCIP indexing disabled for language",
				zap.String("language", lang),
			)
			continue
		}
		if !w.scipService.IsIndexerAvailable(lang) {
			logger.Debug("SCIP indexer binary not available for language",
				zap.String("language", lang),
			)
			continue
		}
		enabledLangs = append(enabledLangs, lang)
	}

	if len(enabledLangs) == 0 {
		logger.Debug("No enabled/available SCIP languages found")
		return
	}

	logger.Info("Starting SCIP indexing",
		zap.Strings("languages", enabledLangs),
	)

	// Apply aggregate timeout from config — covers all languages
	scipCtx := ctx
	var cancel context.CancelFunc
	if w.cfg.SCIP.Timeout > 0 {
		scipCtx, cancel = context.WithTimeout(ctx, w.cfg.SCIP.Timeout)
		defer cancel()
	}

	// Index each language: first clears, rest append
	for idx, lang := range enabledLangs {
		opts := &scipkg.IndexOptions{
			CodeHostURL:  conn.URL,
			CodeHostType: conn.Type,
			Token:        conn.Token,
			ClearFirst:   idx == 0, // Clear only before the first language
		}

		result, err := w.scipService.IndexWithOptions(scipCtx, repoID, repoPath, lang, opts)
		if err != nil {
			logger.Warn("SCIP indexing failed for language (non-fatal)",
				zap.String("language", lang),
				zap.Error(err),
			)
			continue
		}

		logger.Info("SCIP indexing completed for language",
			zap.String("language", lang),
			zap.Bool("success", result.Success),
			zap.Int("files", result.Files),
			zap.Int("symbols", result.Symbols),
			zap.Duration("duration", result.Duration),
		)
	}
}

// updateProgress updates job progress, logging any errors at debug level.
// Progress updates are best-effort and should not fail the job.
// Progress is always out of 100 (percentage).
func (w *Worker) updateProgress(ctx context.Context, jobID string, current int, message string) {
	if err := w.queue.UpdateProgress(ctx, jobID, current, 100, message); err != nil {
		w.logger.Debug("Failed to update job progress",
			zap.String("job_id", jobID),
			zap.Int("current", current),
			zap.Error(err),
		)
	}
}

// markIndexJobInactive marks an index job as inactive, logging any errors at debug level.
// This is a best-effort cleanup operation.
func (w *Worker) markIndexJobInactive(ctx context.Context, repoID int64) {
	if err := w.queue.MarkIndexJobInactive(ctx, repoID); err != nil {
		w.logger.Debug("Failed to mark index job inactive",
			zap.Int64("repo_id", repoID),
			zap.Error(err),
		)
	}
}

// updateIndexStatusBestEffort updates index status, logging any errors at debug level.
// Used when the primary operation has already failed and status update is best-effort.
func (w *Worker) updateIndexStatusBestEffort(
	ctx context.Context,
	repoID int64,
	status string,
	indexed bool,
) {
	if err := w.reposService.UpdateIndexStatus(ctx, repoID, status, indexed); err != nil {
		w.logger.Debug("Failed to update index status",
			zap.Int64("repo_id", repoID),
			zap.String("status", status),
			zap.Error(err),
		)
	}
}

// Run starts the worker loop.
// It supports graceful shutdown: when ctx is canceled, it will finish
// processing the current job before returning (up to shutdownTimeout).
func (w *Worker) Run(ctx context.Context) error {
	w.logger.Info("Worker starting",
		zap.Int("shard_index", w.shardConfig.ShardIndex),
		zap.Int("total_shards", w.shardConfig.TotalShards),
	)

	// Start job recovery loop in background.
	// Uses Redis lock to ensure only one indexer runs recovery, regardless of shard index.
	go w.runRecoveryWithLeaderElection(ctx)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Worker received shutdown signal, stopping...")
			return nil
		default:
			// Try to get a job for our shard
			job, err := w.shardedQueue.DequeueForShard(ctx, 5*time.Second)
			if err != nil {
				// Check if this is due to context cancellation (shutdown)
				if ctx.Err() != nil {
					w.logger.Info("Worker stopping due to context cancellation")
					return nil
				}

				w.logger.Error("Failed to dequeue job", zap.Error(err))
				time.Sleep(1 * time.Second)

				continue
			}

			if job == nil {
				// No job available, continue waiting
				continue
			}

			w.logger.Info(
				"Processing job",
				zap.String("id", job.ID),
				zap.String("type", string(job.Type)),
			)

			// Mark job as running
			if err := w.queue.MarkRunning(context.Background(), job.ID); err != nil {
				w.logger.Error("Failed to mark job as running", zap.Error(err))
			}

			// Create a job context that allows graceful completion even during shutdown.
			// If the main context is canceled, we give the job up to shutdownTimeout to complete.
			jobCtx, jobCancel := w.createJobContext(ctx)

			// Start heartbeat goroutine for long-running jobs
			heartbeatCtx, cancelHeartbeat := context.WithCancel(jobCtx)
			go w.sendHeartbeats(heartbeatCtx, job.ID)

			// Process the job based on type
			var processErr error

			switch job.Type {
			case queue.JobTypeIndex:
				processErr = w.processIndexJob(jobCtx, job)
			case queue.JobTypeSync:
				processErr = w.processSyncJob(jobCtx, job)
			case queue.JobTypeReplace:
				processErr = w.processReplaceJob(jobCtx, job)
			case queue.JobTypeCleanup:
				processErr = w.processCleanupJob(jobCtx, job)
			default:
				processErr = fmt.Errorf("unknown job type: %s", job.Type)
			}

			// Stop heartbeat and cleanup job context
			cancelHeartbeat()
			jobCancel()

			// Update job status and release (use background context to ensure this completes)
			statusCtx, statusCancel := context.WithTimeout(context.Background(), 30*time.Second)

			if processErr != nil {
				w.logger.Error("Job failed", zap.String("id", job.ID), zap.Error(processErr))

				err := w.shardedQueue.MarkFailedAndRelease(statusCtx, job.ID, processErr)
				if err != nil {
					w.logger.Error("Failed to mark job as failed", zap.Error(err))
				}
			} else {
				w.logger.Info("Job completed", zap.String("id", job.ID))

				err := w.shardedQueue.MarkCompletedAndRelease(statusCtx, job.ID)
				if err != nil {
					w.logger.Error("Failed to mark job as completed", zap.Error(err))
				}
			}

			statusCancel()

			// Check if we should stop after completing this job
			if ctx.Err() != nil {
				w.logger.Info("Worker completed current job during shutdown, stopping...")
				return nil
			}
		}
	}
}

// createJobContext creates a context for job processing that supports graceful shutdown.
// When the parent context is canceled, the job context will continue for up to shutdownTimeout
// to allow the current job to complete gracefully.
func (w *Worker) createJobContext(parentCtx context.Context) (context.Context, context.CancelFunc) {
	// Create a context that doesn't immediately inherit cancellation from parent
	jobCtx, jobCancel := context.WithCancel(context.Background())

	// Monitor parent context and propagate cancellation after timeout
	go func() {
		select {
		case <-parentCtx.Done():
			// Parent canceled - give job time to complete gracefully
			w.logger.Info("Shutdown signal received, allowing current job to complete",
				zap.Duration("timeout", w.shutdownTimeout))

			// Wait for shutdown timeout before canceling job
			select {
			case <-time.After(w.shutdownTimeout):
				w.logger.Warn("Shutdown timeout exceeded, forcefully canceling job")
				jobCancel()
			case <-jobCtx.Done():
				// Job completed before timeout
			}
		case <-jobCtx.Done():
			// Job completed normally
		}
	}()

	return jobCtx, jobCancel
}

// sendHeartbeats sends periodic heartbeats for a job.
func (w *Worker) sendHeartbeats(ctx context.Context, jobID string) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := w.shardedQueue.Heartbeat(ctx, jobID)
			if err != nil {
				w.logger.Warn(
					"Failed to send heartbeat",
					zap.String("job_id", jobID),
					zap.Error(err),
				)
			}
		}
	}
}

// runRecoveryWithLeaderElection runs the recovery loop only when this instance
// holds the recovery leader lock. Any indexer shard can become recovery leader.
func (w *Worker) runRecoveryWithLeaderElection(ctx context.Context) {
	const (
		recoveryLeaderKey = "codesearch:indexer:recovery-leader"
		leaseTTL          = 60 * time.Second
		checkInterval     = 20 * time.Second
		recoveryInterval  = 30 * time.Second
	)

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	isLeader := false

	for {
		select {
		case <-ctx.Done():
			// Release leadership on shutdown
			if isLeader {
				script := `if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) else return 0 end`
				_, _ = w.redisClient.Eval(ctx, script, []string{recoveryLeaderKey}, w.workerID).Result()
			}

			return
		case <-ticker.C:
			if isLeader {
				// Refresh lease
				script := `if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("PEXPIRE", KEYS[1], ARGV[2]) else return 0 end`
				result, err := w.redisClient.Eval(ctx, script, []string{recoveryLeaderKey},
					w.workerID, int(leaseTTL.Milliseconds())).Int()
				if err != nil || result == 0 {
					w.logger.Info("Lost recovery leadership")
					isLeader = false
				} else {
					// Run recovery
					if recovered, err := w.shardedQueue.RecoverStaleJobs(ctx); err != nil {
						w.logger.Warn("Job recovery error", zap.Error(err))
					} else if recovered > 0 {
						w.logger.Info("Recovered stale jobs", zap.Int("count", recovered))
					}

					if processed, err := w.shardedQueue.ProcessRetryQueue(ctx); err != nil {
						w.logger.Warn("Retry queue processing error", zap.Error(err))
					} else if processed > 0 {
						w.logger.Info("Processed retry jobs", zap.Int("count", processed))
					}
				}
			} else {
				// Try to become leader
				ok, err := w.redisClient.SetNX(ctx, recoveryLeaderKey, w.workerID, leaseTTL).Result()
				if err == nil && ok {
					w.logger.Info("Acquired recovery leadership")
					isLeader = true
				}
			}
		}
	}
}

// extendLock periodically extends a distributed lock's TTL.
// Call this in a goroutine for long-running operations.
func (w *Worker) extendLock(ctx context.Context, repoLock *lock.DistributedLock, repoName string) {
	ticker := time.NewTicker(5 * time.Minute) // Extend every 5 minutes (lock TTL is 30 min)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := repoLock.Extend(ctx)
			if err != nil {
				w.logger.Warn("Failed to extend repo lock",
					zap.String("repo", repoName),
					zap.Error(err),
				)
			}
		}
	}
}

// sendRepoHeartbeats periodically touches the repository's updated_at timestamp.
// This prevents CleanupStaleIndexing from resetting long-running indexing jobs.
// Call this in a goroutine for long-running indexing operations.
func (w *Worker) sendRepoHeartbeats(ctx context.Context, repoID int64, repoName string) {
	ticker := time.NewTicker(10 * time.Minute) // Touch every 10 minutes (stale threshold is 1+ hours)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := w.reposService.TouchRepository(ctx, repoID)
			if err != nil {
				w.logger.Warn("Failed to send repo heartbeat",
					zap.String("repo", repoName),
					zap.Int64("repo_id", repoID),
					zap.Error(err),
				)
			} else {
				w.logger.Debug("Sent repo heartbeat",
					zap.String("repo", repoName),
					zap.Int64("repo_id", repoID),
				)
			}
		}
	}
}

// processIndexJob handles indexing a repository.
func (w *Worker) processIndexJob(ctx context.Context, job *queue.Job) error {
	ctx, span := tracing.StartSpan(ctx, "job.index")
	defer span.End()

	start := time.Now()

	var payload queue.IndexPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	// Always mark job as inactive when done (success or failure)
	defer func() {
		if err := w.queue.MarkIndexJobInactive(context.Background(), payload.RepositoryID); err != nil {
			w.logger.Warn(
				"Failed to mark index job inactive",
				zap.Int64("repo_id", payload.RepositoryID),
				zap.Error(err),
			)
		}
	}()

	// Acquire distributed lock for this repository to prevent concurrent operations
	// TTL of 30 minutes should cover even large repos; we extend it during long operations
	repoLock := lock.NewDistributedLock(w.redisClient, "repo:"+payload.RepoName, 30*time.Minute)

	acquired, err := repoLock.TryAcquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire repo lock: %w", err)
	}

	if !acquired {
		w.logger.Info("Repository locked by another worker, skipping job",
			zap.String("repo_name", payload.RepoName),
			zap.String("job_id", job.ID),
		)
		// Don't re-queue - the scheduler will create a new job if needed.
		// Re-queuing here causes job explosion when multiple workers compete for locks.
		return nil // Not an error, just couldn't acquire lock
	}

	defer func() { _ = repoLock.Release(ctx) }()

	// Start lock extension goroutine for long-running operations
	lockCtx, cancelLock := context.WithCancel(ctx)
	defer cancelLock()

	go w.extendLock(lockCtx, repoLock, payload.RepoName)

	// Start repo heartbeat goroutine to prevent stale cleanup from resetting status
	// This is especially important for large repos with millions of files
	repoHeartbeatCtx, cancelRepoHeartbeat := context.WithCancel(ctx)
	defer cancelRepoHeartbeat()

	go w.sendRepoHeartbeats(repoHeartbeatCtx, payload.RepositoryID, payload.RepoName)

	tracing.SetAttributes(ctx,
		tracing.AttrJobID.String(job.ID),
		tracing.AttrJobType.String(string(job.Type)),
		tracing.AttrRepoID.Int64(payload.RepositoryID),
		tracing.AttrRepoName.String(payload.RepoName),
		tracing.AttrConnectionID.Int64(payload.ConnectionID),
	)

	// Check if this shard should handle this repo
	if !w.shardConfig.ShouldHandleRepo(payload.RepoName) {
		w.logger.Info("Skipping repo (not assigned to this shard)",
			zap.String("repo_name", payload.RepoName),
			zap.Int("shard_index", w.shardConfig.ShardIndex),
			zap.Int("assigned_shard", GetShardForRepo(payload.RepoName, w.shardConfig.TotalShards)),
		)

		return nil // Not an error, just skip
	}

	// Create logger with repo context for structured logging
	logger := w.logger.With(
		zap.Int64("repo_id", payload.RepositoryID),
		zap.String("repo_name", payload.RepoName),
	)

	logger.Info("Processing index job",
		zap.Int64("connection_id", payload.ConnectionID),
		zap.String("clone_url", payload.CloneURL),
		zap.Int("shard_index", w.shardConfig.ShardIndex),
	)

	// Fetch connection to get the token
	conn, err := w.reposService.GetConnection(ctx, payload.ConnectionID)
	if err != nil {
		metrics.RecordJob("index", time.Since(start), false)
		tracing.RecordError(ctx, err)

		return fmt.Errorf("get connection: %w", err)
	}

	// Determine repo path - use safe name (replace slashes)
	safeName := strings.ReplaceAll(payload.RepoName, "/", "_")
	repoPath := filepath.Join(w.cfg.Indexer.ReposPath, safeName+".git")

	// Check if repo exists, clone or fetch
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		// Update status to cloning for new repos
		err := w.reposService.UpdateIndexStatus(ctx, payload.RepositoryID, "cloning", false)
		if err != nil {
			w.logger.Warn("Failed to update status to cloning", zap.Error(err))
		}
		// Clone the repository
		if _, err := w.CloneRepository(ctx, payload.CloneURL, payload.RepoName, conn); err != nil {
			w.updateIndexStatusBestEffort(ctx, payload.RepositoryID, "failed", false)

			metrics.RecordJob("index", time.Since(start), false)
			tracing.RecordError(ctx, err)

			return fmt.Errorf("clone repository: %w", err)
		}
	} else {
		// Update status to indexing for existing repos
		err := w.reposService.UpdateIndexStatus(ctx, payload.RepositoryID, "indexing", false)
		if err != nil {
			w.logger.Warn("Failed to update status to indexing", zap.Error(err))
		}
		// Fetch updates
		err = w.FetchRepository(ctx, repoPath, conn)
		if err != nil {
			w.logger.Warn("Failed to fetch, will try to re-clone", zap.Error(err))
			// Try removing and re-cloning
			_ = os.RemoveAll(repoPath)

			if _, err := w.CloneRepository(ctx, payload.CloneURL, payload.RepoName, conn); err != nil {
				w.updateIndexStatusBestEffort(ctx, payload.RepositoryID, "failed", false)

				metrics.RecordJob("index", time.Since(start), false)
				tracing.RecordError(ctx, err)

				return fmt.Errorf("re-clone repository: %w", err)
			}
		}
	}

	// Update status to indexing before running zoekt
	if err := w.reposService.UpdateIndexStatus(ctx, payload.RepositoryID, "indexing", false); err != nil {
		w.logger.Warn("Failed to update status to indexing", zap.Error(err))
	}

	// Build list of branches to index
	// Priority:
	// 1. If repo has specific branches configured (via config or UI), use those
	// 2. Else if index_all_branches is globally enabled, discover all branches
	// 3. Else use default branch only
	var branches []string

	// Check if branches were explicitly configured (not just default branch)
	// - Multiple branches = explicit config
	// - Single branch matching default = use it
	// - Single branch differing from default = check if it exists (could be stale or intentional)
	var hasConfiguredBranches bool

	switch {
	case len(payload.Branches) > 1:
		// Multiple branches = explicit config
		hasConfiguredBranches = true
	case len(payload.Branches) == 1 && payload.Branches[0] == payload.Branch:
		// Single branch matches default = use it
		hasConfiguredBranches = true
	case len(payload.Branches) == 1:
		// Single branch differs from default - check if it actually exists
		// If it exists, it's intentional config. If not, it's stale data.
		if w.branchExists(ctx, repoPath, payload.Branches[0]) {
			hasConfiguredBranches = true

			w.logger.Info("Custom branch exists, using it",
				zap.String("repo", payload.RepoName),
				zap.String("branch", payload.Branches[0]),
			)
		} else {
			hasConfiguredBranches = false

			w.logger.Warn("Configured branch does not exist, falling back to default",
				zap.String("repo", payload.RepoName),
				zap.String("configured_branch", payload.Branches[0]),
				zap.String("default_branch", payload.Branch),
			)
		}
	default:
		// No branches configured
		hasConfiguredBranches = false
	}

	if hasConfiguredBranches {
		// Use explicitly configured branches (from config or UI)
		branches = payload.Branches
		w.logger.Info("Using configured branches",
			zap.String("repo", payload.RepoName),
			zap.Strings("branches", branches),
		)
	} else if w.cfg.Indexer.IndexAllBranches {
		// Discover all branches from the repo
		discoveredBranches, err := w.discoverBranches(ctx, repoPath)
		if err != nil {
			w.logger.Warn("Failed to discover branches, falling back to default",
				zap.String("repo", payload.RepoName),
				zap.Error(err),
			)
			branches = []string{payload.Branch}
		} else {
			branches = discoveredBranches
			w.logger.Info("Discovered branches for indexing",
				zap.String("repo", payload.RepoName),
				zap.Int("count", len(branches)),
				zap.Strings("branches", branches),
			)
		}
	} else {
		// Default: use only the primary branch
		branches = []string{payload.Branch}
	}

	// Get repository size for observability
	repoSizeMB, err := w.getRepoSizeMB(repoPath)
	if err != nil {
		logger.Warn("Failed to check repo size, proceeding with indexing",
			zap.Error(err),
		)
		repoSizeMB = 0 // Default to 0 if we can't get size
	} else {
		logger.Info("Repository size",
			zap.Int64("size_mb", repoSizeMB),
		)
	}
	repoSizeBytes := repoSizeMB * 1024 * 1024

	// Check repo size if configured
	if w.cfg.Indexer.MaxRepoSizeMB > 0 && repoSizeMB > w.cfg.Indexer.MaxRepoSizeMB {
		msg := fmt.Sprintf("repository size (%d MB) exceeds max_repo_size_mb limit (%d MB), skipping indexing",
			repoSizeMB, w.cfg.Indexer.MaxRepoSizeMB)

		logger.Warn(msg,
			zap.Int64("repo_size_mb", repoSizeMB),
			zap.Int64("max_size_mb", w.cfg.Indexer.MaxRepoSizeMB),
		)

		// Mark as failed with a clear message
		if err := w.reposService.UpdateIndexStatus(ctx, payload.RepositoryID, "failed: repo too large", false); err != nil {
			logger.Warn("Failed to update status", zap.Error(err))
		}

		metrics.RecordJob("index", time.Since(start), false)
		metrics.RecordRepoSize(repoSizeBytes, false)
		metrics.RecordIndexFailure("repo_too_large")

		return fmt.Errorf("%s", msg)
	} else if repoSizeMB > 0 {
		logger.Info("Repository size check passed",
			zap.Int64("repo_size_mb", repoSizeMB),
			zap.Int64("max_size_mb", w.cfg.Indexer.MaxRepoSizeMB),
		)
	}

	// Filter out branches that don't actually exist in the repo
	// (e.g. empty repos that have no commits, or stale branch names)
	var validBranches []string
	for _, b := range branches {
		if w.branchExists(ctx, repoPath, b) {
			validBranches = append(validBranches, b)
		} else {
			logger.Warn("Branch does not exist in repo, skipping",
				zap.String("branch", b),
			)
		}
	}

	if len(validBranches) == 0 {
		logger.Warn("No valid branches found, repository may be empty",
			zap.Strings("requested_branches", branches),
		)

		w.updateIndexStatusBestEffort(ctx, payload.RepositoryID, "empty", false)
		metrics.RecordJob("index", time.Since(start), true)

		return nil
	}

	branches = validBranches

	logger.Info("Indexing branches",
		zap.Strings("branches", branches),
		zap.Int64("repo_size_mb", repoSizeMB),
	)

	// Index the repository with all branches
	if err := w.IndexRepository(ctx, repoPath, branches); err != nil {
		// Record repo size with failure
		metrics.RecordRepoSize(repoSizeBytes, false)

		// Categorize failure reason for better observability
		failureReason := categorizeIndexFailure(err)
		metrics.RecordIndexFailure(failureReason)

		logger.Error("Indexing failed",
			zap.Error(err),
			zap.String("failure_reason", failureReason),
			zap.Int64("repo_size_mb", repoSizeMB),
			zap.Duration("elapsed", time.Since(start)),
		)

		w.updateIndexStatusBestEffort(ctx, payload.RepositoryID, "failed", false)

		metrics.RecordJob("index", time.Since(start), false)
		tracing.RecordError(ctx, err)

		return fmt.Errorf("index repository: %w", err)
	}

	// Record successful indexing with repo size
	metrics.RecordRepoSize(repoSizeBytes, true)

	logger.Info("Indexing completed successfully",
		zap.Int64("repo_size_mb", repoSizeMB),
		zap.Duration("duration", time.Since(start)),
	)

	// Mark as indexed
	if err := w.reposService.UpdateIndexStatus(ctx, payload.RepositoryID, "indexed", true); err != nil {
		logger.Warn("Failed to update status to indexed", zap.Error(err))
	}

	metrics.RecordJob("index", time.Since(start), true)
	tracing.SetOK(ctx)

	// Publish reindex event for cache invalidation across API server instances
	if err := w.redisClient.Publish(ctx, "codesearch:repo:reindexed",
		fmt.Sprintf("%d", payload.RepositoryID)).Err(); err != nil {
		logger.Debug("Failed to publish reindex event", zap.Error(err))
	}

	// Run SCIP code intelligence indexing (non-fatal)
	w.runSCIPIndexing(ctx, payload.RepositoryID, repoPath, payload.RepoName, conn)

	return nil
}

// processSyncJob handles syncing repositories from a connection
// This fetches repos from the code host and adds them to the database.
func (w *Worker) processSyncJob(ctx context.Context, job *queue.Job) error {
	ctx, span := tracing.StartSpan(ctx, "job.sync")
	defer span.End()

	start := time.Now()

	var payload queue.SyncPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	// Always mark job as inactive when done (success or failure)
	defer func() {
		if err := w.queue.MarkSyncJobInactive(context.Background(), payload.ConnectionID); err != nil {
			w.logger.Warn(
				"Failed to mark sync job inactive",
				zap.Int64("connection_id", payload.ConnectionID),
				zap.Error(err),
			)
		}
	}()

	tracing.SetAttributes(ctx,
		tracing.AttrJobID.String(job.ID),
		tracing.AttrJobType.String(string(job.Type)),
		tracing.AttrConnectionID.Int64(payload.ConnectionID),
	)

	w.logger.Info("Processing sync job", zap.Int64("connection_id", payload.ConnectionID))

	// Update progress: starting
	w.updateProgress(ctx, job.ID, 0, "Connecting to code host...")

	// Get connection details
	conn, err := w.reposService.GetConnection(ctx, payload.ConnectionID)
	if err != nil {
		metrics.RecordJob("sync", time.Since(start), false)
		tracing.RecordError(ctx, err)

		return fmt.Errorf("get connection: %w", err)
	}

	if conn == nil {
		err := fmt.Errorf("connection not found: %d", payload.ConnectionID)

		metrics.RecordJob("sync", time.Since(start), false)
		tracing.RecordError(ctx, err)

		return err
	}

	tracing.SetAttributes(ctx, tracing.AttrHostType.String(conn.Type))

	// Create client for this connection
	client, err := codehost.NewClient(conn.Type, conn.URL, conn.Token)
	if err != nil {
		metrics.RecordJob("sync", time.Since(start), false)
		tracing.RecordError(ctx, err)

		return fmt.Errorf("create codehost client: %w", err)
	}

	// Sync repositories using the service method
	w.updateProgress(ctx, job.ID, 10, "Syncing repositories from code host...")
	w.logger.Info("Syncing repositories from code host",
		zap.Int64("connection_id", payload.ConnectionID),
		zap.String("type", conn.Type),
		zap.Bool("exclude_archived", conn.ExcludeArchived),
		zap.Int("repos_filter_count", len(conn.Repos)),
	)

	// Get repo configs from config file (if any)
	var repoConfigs []repos.RepoConfigEntry

	if codeHost, ok := w.cfg.CodeHosts[conn.Name]; ok {
		for _, rc := range codeHost.RepoConfigs {
			repoConfigs = append(repoConfigs, repos.RepoConfigEntry{
				Name:     rc.Name,
				Branches: rc.Branches,
				Exclude:  rc.Exclude,
				Delete:   rc.Delete,
			})
		}
	}

	// Check if cleanup_archived is enabled (from config file or connection setting)
	cleanupArchived := conn.CleanupArchived
	if codeHost, ok := w.cfg.CodeHosts[conn.Name]; ok {
		cleanupArchived = cleanupArchived || codeHost.CleanupArchived
	}

	adapter := &codeHostAdapter{client: client}
	archivedRepos, err := w.reposService.SyncRepositories(ctx, payload.ConnectionID, adapter, conn.ExcludeArchived, cleanupArchived, conn.Repos, repoConfigs)
	if err != nil {
		metrics.RecordJob("sync", time.Since(start), false)
		tracing.RecordError(ctx, err)

		return fmt.Errorf("sync repositories: %w", err)
	}

	// Queue cleanup jobs for repos that were newly archived and excluded
	if len(archivedRepos) > 0 {
		w.logger.Info("Queueing cleanup jobs for newly archived repos",
			zap.Int("count", len(archivedRepos)),
			zap.Int64("connection_id", payload.ConnectionID),
		)

		for _, archived := range archivedRepos {
			cleanupPayload := queue.CleanupPayload{
				RepositoryID:   archived.ID,
				RepositoryName: archived.Name,
				DataDir:        w.cfg.Indexer.IndexPath,
			}
			if _, err := w.queue.Enqueue(ctx, queue.JobTypeCleanup, cleanupPayload); err != nil {
				w.logger.Warn("Failed to queue cleanup job for archived repo",
					zap.Int64("repo_id", archived.ID),
					zap.String("repo_name", archived.Name),
					zap.Error(err),
				)
			}
		}
	}

	// Update progress: queueing index jobs
	w.updateProgress(ctx, job.ID, 75, "Queueing index jobs...")

	// Now queue index jobs for all repos in this connection
	repos, err := w.reposService.ListRepositories(ctx, &payload.ConnectionID)
	if err != nil {
		metrics.RecordJob("sync", time.Since(start), false)
		tracing.RecordError(ctx, err)

		return fmt.Errorf("list repositories: %w", err)
	}

	indexQueued := 0
	indexSkipped := 0

	totalToQueue := len(repos)
	for i, repo := range repos {
		// Update progress periodically
		if i%10 == 0 || i == totalToQueue-1 {
			progress := 75 + int(float64(i)/float64(totalToQueue)*25) // 75-100%
			w.updateProgress(
				ctx,
				job.ID,
				progress,
				fmt.Sprintf("Queueing index jobs (%d/%d)...", i+1, totalToQueue),
			)
		}

		// Atomically try to acquire the job slot - this eliminates race conditions
		acquired, err := w.queue.TryAcquireIndexJob(ctx, repo.ID)
		if err != nil {
			w.logger.Warn(
				"Failed to acquire index job slot",
				zap.String("repo", repo.Name),
				zap.Error(err),
			)
			// Continue anyway - better to have duplicate than miss
		} else if !acquired {
			indexSkipped++
			continue // Skip - already has a pending job
		}

		// Update repo status to pending before queueing
		// This ensures the scheduler can pick up stuck repos if the job fails to complete
		if err := w.reposService.UpdateIndexStatus(ctx, repo.ID, "pending", false); err != nil {
			w.logger.Warn("Failed to update status to pending",
				zap.String("repo", repo.Name),
				zap.Error(err),
			)
			// Continue anyway - the job will still be queued
		}

		indexPayload := queue.IndexPayload{
			RepositoryID: repo.ID,
			ConnectionID: repo.ConnectionID,
			RepoName:     repo.Name,
			CloneURL:     repo.CloneURL,
			Branch:       repo.DefaultBranch,
			Branches:     repo.Branches, // Include all branches for multi-branch indexing
		}

		if _, err := w.queue.Enqueue(ctx, queue.JobTypeIndex, indexPayload); err != nil {
			w.markIndexJobInactive(ctx, repo.ID)
			w.logger.Error(
				"Failed to queue index job",
				zap.String("repo", repo.Name),
				zap.Error(err),
			)
		} else {
			indexQueued++
		}
	}

	w.logger.Info("Queued index jobs for repositories",
		zap.Int("queued", indexQueued),
		zap.Int("skipped", indexSkipped),
	)

	metrics.RecordJob("sync", time.Since(start), true)
	metrics.RecordRepoSync(time.Since(start), true)
	tracing.AddEvent(ctx, "sync.completed",
		attribute.Int("repos_synced", len(repos)),
		attribute.Int("index_jobs_queued", indexQueued),
	)
	tracing.SetOK(ctx)

	return nil
}

// processReplaceJob handles search and replace operations across repositories.
func (w *Worker) processReplaceJob(ctx context.Context, job *queue.Job) error {
	ctx, span := tracing.StartSpan(ctx, "job.replace")
	defer span.End()

	start := time.Now()

	var payload queue.ReplacePayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	tracing.SetAttributes(ctx,
		tracing.AttrJobID.String(job.ID),
		tracing.AttrJobType.String(string(job.Type)),
		attribute.String("replace.search_pattern", payload.SearchPattern),
		attribute.Bool("replace.is_regex", payload.IsRegex),
		attribute.Int("replace.matches_count", len(payload.Matches)),
	)

	w.logger.Info("Processing replace job",
		zap.String("job_id", job.ID),
		zap.String("search_pattern", payload.SearchPattern),
		zap.String("replace_with", payload.ReplaceWith),
		zap.Bool("is_regex", payload.IsRegex),
		zap.Int("matches_count", len(payload.Matches)),
		zap.Bool("repos_read_only", payload.ReposReadOnly),
		zap.Int("user_tokens_count", len(payload.UserTokens)),
	)

	// Update progress: starting
	w.updateProgress(ctx, job.ID, 0, "Starting search and replace operation...")

	// Convert precomputed matches from payload
	var precomputedMatches []replace.PrecomputedMatch
	for _, m := range payload.Matches {
		precomputedMatches = append(precomputedMatches, replace.PrecomputedMatch{
			RepositoryID:   m.RepositoryID,
			RepositoryName: m.RepositoryName,
			FilePath:       m.FilePath,
		})
	}

	// Build replace options from payload (MR is always created)
	opts := replace.ReplaceOptions{
		SearchPattern:      payload.SearchPattern,
		ReplaceWith:        payload.ReplaceWith,
		IsRegex:            payload.IsRegex,
		CaseSensitive:      payload.CaseSensitive,
		FilePatterns:       payload.FilePatterns,
		MRTitle:            payload.MRTitle,
		MRDescription:      payload.MRDescription,
		BranchName:         payload.BranchName,
		PrecomputedMatches: precomputedMatches,
		DryRun:             false,
		UserTokens:         payload.UserTokens,
		ReposReadOnly:      payload.ReposReadOnly,
	}

	// Ensure we have matches (required after removing search fallback)
	if len(precomputedMatches) == 0 {
		err := errors.New("no matches provided in job payload: preview must be called first")

		metrics.RecordJob("replace", time.Since(start), false)
		metrics.RecordReplace(0, false)
		tracing.RecordError(ctx, err)

		return err
	}

	w.updateProgress(ctx, job.ID, 10, "Processing matches...")

	// Execute the replacement
	results, err := w.replaceService.Execute(ctx, opts)
	if err != nil {
		metrics.RecordJob("replace", time.Since(start), false)
		metrics.RecordReplace(0, false)
		tracing.RecordError(ctx, err)

		return fmt.Errorf("execute replace: %w", err)
	}

	// Calculate totals for logging
	totalRepos := len(results)
	successCount := 0
	errorCount := 0
	totalFilesModified := 0
	totalMRsCreated := 0

	for i, result := range results {
		// Update progress for each repository
		progress := 10 + int(float64(i+1)/float64(totalRepos)*80) // 10-90%
		w.updateProgress(ctx, job.ID, progress,
			fmt.Sprintf("Processing repository %d/%d: %s", i+1, totalRepos, result.RepositoryName))

		if result.Error != "" {
			errorCount++

			w.logger.Warn("Replace failed for repository",
				zap.String("repository", result.RepositoryName),
				zap.String("error", result.Error),
			)
		} else {
			successCount++

			totalFilesModified += result.FilesModified
			if result.MergeRequest != nil {
				totalMRsCreated++
			}
		}
	}

	// Update progress: completed
	w.updateProgress(ctx, job.ID, 100, "Replacement operation completed")

	w.logger.Info("Replace job completed",
		zap.String("job_id", job.ID),
		zap.Int("total_repos", totalRepos),
		zap.Int("success_count", successCount),
		zap.Int("error_count", errorCount),
		zap.Int("total_files_modified", totalFilesModified),
		zap.Int("total_mrs_created", totalMRsCreated),
	)

	// Record metrics
	metrics.RecordJob("replace", time.Since(start), errorCount == 0 || successCount > 0)
	metrics.RecordReplace(totalFilesModified, successCount > 0)
	tracing.AddEvent(ctx, "replace.completed",
		attribute.Int("repos_processed", totalRepos),
		attribute.Int("success_count", successCount),
		attribute.Int("error_count", errorCount),
		attribute.Int("files_modified", totalFilesModified),
		attribute.Int("mrs_created", totalMRsCreated),
	)

	// If all repos had errors, return an error with details
	if errorCount > 0 && successCount == 0 {
		// Collect unique error messages for better diagnostics
		errorMessages := make(map[string]int)

		for _, result := range results {
			if result.Error != "" {
				// Simplify common error patterns for grouping
				errMsg := result.Error
				if strings.Contains(errMsg, "authentication") || strings.Contains(errMsg, "401") ||
					strings.Contains(errMsg, "403") {
					errMsg = "authentication failed (invalid or insufficient token permissions)"
				} else if strings.Contains(errMsg, "clone") {
					errMsg = "git clone failed"
				} else if strings.Contains(errMsg, "push") {
					errMsg = "git push failed"
				}

				errorMessages[errMsg]++
			}
		}

		// Build error summary
		var errParts []string

		for msg, count := range errorMessages {
			if count == 1 {
				errParts = append(errParts, msg)
			} else {
				errParts = append(errParts, fmt.Sprintf("%s (%d repos)", msg, count))
			}
		}

		err := fmt.Errorf(
			"all %d repositories failed: %s",
			errorCount,
			strings.Join(errParts, "; "),
		)
		tracing.RecordError(ctx, err)

		return err
	}

	tracing.SetOK(ctx)

	return nil
}

// processCleanupJob handles cleaning up Zoekt index shards and cloned repo files
// when a repository is excluded or deleted.
func (w *Worker) processCleanupJob(ctx context.Context, job *queue.Job) error {
	var payload queue.CleanupPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	// Always mark cleanup job as inactive when done (success or failure)
	defer func() {
		if err := w.queue.MarkCleanupJobInactive(context.Background(), payload.RepositoryID); err != nil {
			w.logger.Warn(
				"Failed to mark cleanup job inactive",
				zap.Int64("repo_id", payload.RepositoryID),
				zap.Error(err),
			)
		}
	}()

	// Acquire distributed lock for this repository
	repoLock := lock.NewDistributedLock(
		w.redisClient,
		"repo:"+payload.RepositoryName,
		10*time.Minute,
	)

	acquired, err := repoLock.TryAcquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire repo lock: %w", err)
	}

	if !acquired {
		w.logger.Info("Repository locked by another worker, skipping cleanup job",
			zap.String("repo_name", payload.RepositoryName),
			zap.String("job_id", job.ID),
		)
		// Don't re-queue - cleanup will be retried on next exclusion if needed.
		return nil
	}

	defer func() { _ = repoLock.Release(ctx) }()

	w.logger.Info("Processing cleanup job",
		zap.Int64("repository_id", payload.RepositoryID),
		zap.String("repository_name", payload.RepositoryName),
	)

	// Update progress: starting
	w.updateProgress(ctx, job.ID, 0, "Starting cleanup...")

	// Clean up cloned repository files
	// Repository names like "owner/repo" are stored as "owner_repo.git"
	safeName := strings.ReplaceAll(payload.RepositoryName, "/", "_")
	repoPath := filepath.Join(w.cfg.Indexer.ReposPath, safeName+".git")

	w.updateProgress(ctx, job.ID, 25, "Removing cloned repository...")

	if err := os.RemoveAll(repoPath); err != nil && !os.IsNotExist(err) {
		w.logger.Warn("Failed to remove cloned repository",
			zap.String("path", repoPath),
			zap.Error(err),
		)
	} else {
		w.logger.Info("Removed cloned repository", zap.String("path", repoPath))
	}

	// Clean up Zoekt index shards
	// Zoekt index files can be named in different formats:
	// 1. URL-encoded with host: "gitlab.example.com%2Fowner%2Frepo_v16.00000.zoekt"
	// 2. Underscore-replaced: "owner_repo_v16.00000.zoekt"
	// We need to scan all shards and check if they match the repository name
	w.updateProgress(ctx, job.ID, 50, "Removing Zoekt index shards...")

	// Build multiple patterns to match the repo name in shard files
	// The repo name in the shard could be in different formats
	repoNamePatterns := []string{
		payload.RepositoryName,                                 // Original: owner/repo
		strings.ReplaceAll(payload.RepositoryName, "/", "_"),   // Underscore: owner_repo
		url.PathEscape(payload.RepositoryName),                 // URL-encoded: owner%2Frepo
		strings.ReplaceAll(payload.RepositoryName, "/", "%2F"), // Explicit %2F: owner%2Frepo
	}

	// Get all .zoekt files
	allZoektPattern := filepath.Join(w.cfg.Indexer.IndexPath, "*.zoekt")
	allShardFiles, err := filepath.Glob(allZoektPattern)
	if err != nil {
		w.logger.Warn("Failed to glob index files",
			zap.String("pattern", allZoektPattern),
			zap.Error(err),
		)
	}

	var shardFiles []string
	for _, shardFile := range allShardFiles {
		baseName := filepath.Base(shardFile)

		// Skip compound shards - they're handled separately
		if strings.HasPrefix(baseName, "compound_") {
			continue
		}

		// Check if this shard matches any of our patterns
		// The shard name format is: <repo_name_pattern>_v<version>.<shard>.zoekt
		// We check if the filename contains the repo name pattern followed by _v
		for _, pattern := range repoNamePatterns {
			// Check if the shard name starts with the pattern followed by _v
			// or contains the pattern (for host-prefixed names)
			if strings.HasPrefix(baseName, pattern+"_v") ||
				strings.Contains(baseName, "%2F"+pattern+"_v") ||
				strings.Contains(baseName, "/"+pattern+"_v") ||
				strings.HasSuffix(strings.TrimSuffix(baseName, "_v"+extractVersionSuffix(baseName)), pattern) {
				shardFiles = append(shardFiles, shardFile)
				break
			}
		}
	}

	for _, shardFile := range shardFiles {
		err := os.Remove(shardFile)
		if err != nil && !os.IsNotExist(err) {
			w.logger.Warn("Failed to remove index shard",
				zap.String("file", shardFile),
				zap.Error(err),
			)
		} else {
			w.logger.Info("Removed index shard", zap.String("file", shardFile))
		}
	}

	// Remove compound shards that may contain the deleted repo's data.
	// Remaining repos in those shards still have individual shards on disk,
	// so search continues to work. Zoekt's auto-merge recreates compound
	// shards on the next cycle without the deleted repo.
	compoundPattern := filepath.Join(w.cfg.Indexer.IndexPath, "compound_*.zoekt")
	compoundFiles, _ := filepath.Glob(compoundPattern)

	compoundRemoved := 0
	for _, cf := range compoundFiles {
		if err := os.Remove(cf); err != nil && !os.IsNotExist(err) {
			w.logger.Warn("Failed to remove compound shard",
				zap.String("file", filepath.Base(cf)), zap.Error(err))
		} else if err == nil {
			compoundRemoved++
			w.logger.Info("Removed compound shard containing deleted repo data",
				zap.String("file", filepath.Base(cf)))
		}
	}

	// Clean up SCIP database files
	if w.scipService != nil {
		if err := w.scipService.DeleteIndex(ctx, payload.RepositoryID); err != nil {
			w.logger.Warn("Failed to delete SCIP index",
				zap.Int64("repo_id", payload.RepositoryID),
				zap.Error(err),
			)
		} else {
			w.logger.Info("Deleted SCIP index", zap.Int64("repo_id", payload.RepositoryID))
		}
	}

	w.updateProgress(ctx, job.ID, 100, "Cleanup completed")

	w.logger.Info("Cleanup job completed",
		zap.Int64("repository_id", payload.RepositoryID),
		zap.String("repository_name", payload.RepositoryName),
		zap.Int("shards_removed", len(shardFiles)),
		zap.Int("compound_shards_removed", compoundRemoved),
	)

	return nil
}

// getRepoSizeMB calculates the size of a repository in megabytes.
func (w *Worker) getRepoSizeMB(repoPath string) (int64, error) {
	var totalSize int64

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			totalSize += info.Size()
		}

		return nil
	})

	if err != nil {
		return 0, err
	}

	return totalSize / (1024 * 1024), nil // Convert bytes to MB
}

// IndexRepository indexes a single repository.
func (w *Worker) IndexRepository(ctx context.Context, repoPath string, branches []string) error {
	ctx, span := tracing.StartSpan(ctx, "zoekt.index")
	defer span.End()

	tracing.SetAttributes(ctx,
		attribute.String("zoekt.repo_path", repoPath),
		tracing.AttrBranchCount.Int(len(branches)),
		tracing.AttrBranches.StringSlice(branches),
	)

	w.logger.Info("Starting zoekt-git-index",
		zap.String("path", repoPath),
		zap.Strings("branches", branches),
		zap.String("index_path", w.cfg.Indexer.IndexPath),
		zap.String("zoekt_bin", w.cfg.Indexer.ZoektBin),
		zap.String("ctags_bin", w.cfg.Indexer.CtagsBin),
		zap.Bool("require_ctags", w.cfg.Indexer.RequireCtags),
		zap.Duration("index_timeout", w.cfg.Indexer.IndexTimeout),
	)

	args := []string{
		"-index", w.cfg.Indexer.IndexPath,
	}

	// Enable ctags for symbol indexing
	if w.cfg.Indexer.RequireCtags {
		args = append(args, "-require_ctags")
	}

	if len(branches) > 0 {
		args = append(args, "-branches", strings.Join(branches, ","))
	}

	args = append(args, repoPath)

	w.logger.Debug("Executing zoekt-git-index",
		zap.String("command", w.cfg.Indexer.ZoektBin),
		zap.Strings("args", args),
	)

	// Create context with timeout if configured (0 = no timeout/infinite)
	indexCtx := ctx
	var cancelTimeout context.CancelFunc
	if w.cfg.Indexer.IndexTimeout > 0 {
		indexCtx, cancelTimeout = context.WithTimeout(ctx, w.cfg.Indexer.IndexTimeout)
		defer cancelTimeout()

		w.logger.Info("Indexing with timeout",
			zap.Duration("timeout", w.cfg.Indexer.IndexTimeout),
		)
	}

	start := time.Now()
	cmd := exec.CommandContext(indexCtx, w.cfg.Indexer.ZoektBin, args...)

	// Set CTAGS_COMMAND environment variable to ensure zoekt finds the ctags binary
	// Zoekt uses CTAGS_COMMAND for universal-ctags
	if w.cfg.Indexer.CtagsBin != "" {
		cmd.Env = append(os.Environ(), "CTAGS_COMMAND="+w.cfg.Indexer.CtagsBin)
	}

	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	if err != nil {
		// Record metrics for failure
		metrics.RecordBranchesIndexed(len(branches), false)
		metrics.RecordRepoIndex(duration, false)
		metrics.RecordError("indexer", "zoekt_failed")
		tracing.RecordError(ctx, err)

		w.logger.Error("zoekt-git-index failed",
			zap.String("path", repoPath),
			zap.Error(err),
			zap.String("output", string(output)),
		)

		return fmt.Errorf("zoekt-git-index failed: %w\nOutput: %s", err, string(output))
	}

	// Record metrics for success
	metrics.RecordBranchesIndexed(len(branches), true)
	metrics.RecordRepoIndex(duration, true)
	tracing.SetOK(ctx)
	tracing.AddEvent(ctx, "index.completed",
		attribute.Float64("duration_seconds", duration.Seconds()),
		attribute.Int("branch_count", len(branches)),
	)

	w.logger.Info("Indexing completed successfully",
		zap.String("path", repoPath),
		zap.Int("output_length", len(output)),
	)

	// Log the full output at debug level to aid troubleshooting when index
	// files are not updating as expected. This keeps success logs concise but
	// preserves the detailed output for debugging when debug level is enabled.
	w.logger.Debug("zoekt-git-index output",
		zap.String("output", string(output)),
	)

	// Log index file creation
	w.logIndexFiles()

	return nil
}

// logIndexFiles logs the current state of the index directory.
func (w *Worker) logIndexFiles() {
	files, err := filepath.Glob(filepath.Join(w.cfg.Indexer.IndexPath, "*.zoekt"))
	if err != nil {
		w.logger.Warn("Failed to list index files", zap.Error(err))
		return
	}

	w.logger.Info("Index directory state",
		zap.String("index_path", w.cfg.Indexer.IndexPath),
		zap.Int("zoekt_files_count", len(files)),
	)

	for _, f := range files {
		info, err := os.Stat(f)
		if err == nil {
			w.logger.Debug("Index file",
				zap.String("file", filepath.Base(f)),
				zap.Int64("size_mb", info.Size()/(1024*1024)),
				zap.Time("modified", info.ModTime()),
			)
		}
	}
}

// extractVersionSuffix extracts the version suffix from a Zoekt shard filename.
// For example, "owner_repo_v16.00000.zoekt" returns "16.00000.zoekt".
func extractVersionSuffix(filename string) string {
	// Find "_v" followed by version
	idx := strings.LastIndex(filename, "_v")
	if idx == -1 {
		return ""
	}
	return filename[idx+2:] // Skip the "_v" prefix
}

// discoverBranches discovers all branches in a git repository.
func (w *Worker) discoverBranches(ctx context.Context, repoPath string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "branch", "-a", "--format=%(refname:short)")
	cmd.Dir = repoPath

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git branch failed: %w", err)
	}

	var branches []string

	seen := make(map[string]bool)

	for line := range strings.SplitSeq(strings.TrimSpace(string(output)), "\n") {
		branch := strings.TrimSpace(line)
		if branch == "" {
			continue
		}

		// Skip remote tracking branches like "origin/HEAD -> origin/main"
		if strings.Contains(branch, "->") {
			continue
		}

		// For remote branches (origin/branch), extract just the branch name
		if after, ok := strings.CutPrefix(branch, "origin/"); ok {
			branch = after
		}

		// Skip if we've already seen this branch
		if seen[branch] {
			continue
		}

		seen[branch] = true

		branches = append(branches, branch)
	}

	// If no branches found, return HEAD as fallback
	if len(branches) == 0 {
		return []string{"HEAD"}, nil
	}

	return branches, nil
}

// branchExists checks if a branch exists in the local git repository.
// It checks both local and remote tracking branches.
func (w *Worker) branchExists(ctx context.Context, repoPath, branch string) bool {
	// Try to resolve the branch ref - this works for both local and remote branches
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	cmd.Dir = repoPath

	if err := cmd.Run(); err == nil {
		return true
	}

	// Also check remote tracking branch
	cmd = exec.CommandContext(ctx, "git", "rev-parse", "--verify", "--quiet", "refs/remotes/origin/"+branch)
	cmd.Dir = repoPath

	return cmd.Run() == nil
}

// CloneRepository clones a repository to the local filesystem.
func (w *Worker) CloneRepository(
	ctx context.Context,
	url string,
	name string,
	conn *repos.Connection,
) (string, error) {
	ctx, span := tracing.StartSpan(ctx, "git.clone")
	defer span.End()

	// Replace slashes with underscores to create a flat directory structure
	safeName := strings.ReplaceAll(name, "/", "_")
	destPath := filepath.Join(w.cfg.Indexer.ReposPath, safeName+".git")

	tracing.SetAttributes(ctx,
		tracing.AttrRepoName.String(name),
		attribute.String("git.clone_url", url),
		attribute.String("git.dest_path", destPath),
	)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		tracing.RecordError(ctx, err)
		return "", fmt.Errorf("create parent dir: %w", err)
	}

	w.logger.Info("Cloning repository",
		zap.String("url", url),
		zap.String("dest", destPath),
	)

	// Add authentication token to URL if available
	cloneURL := gitutil.AddAuthToURL(url, conn)

	start := time.Now()
	cmd := exec.CommandContext(ctx, "git", "clone", "--bare", cloneURL, destPath)

	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	if err != nil {
		// Record metrics and tracing for failure
		metrics.RecordGitClone(duration, false)
		metrics.RecordError("git", "clone_failed")
		tracing.RecordError(ctx, err)

		// Sanitize output to remove any credentials
		sanitizedOutput := gitutil.SanitizeGitOutput(string(output), conn)

		return "", fmt.Errorf("git clone failed: %w\nOutput: %s", err, sanitizedOutput)
	}

	// Record metrics for success
	metrics.RecordGitClone(duration, true)
	tracing.SetOK(ctx)
	tracing.AddEvent(
		ctx,
		"clone.completed",
		attribute.Float64("duration_seconds", duration.Seconds()),
	)

	// Reset remote URL to remove token (security: don't store credentials in git config)
	resetCmd := exec.CommandContext(ctx, "git", "-C", destPath, "remote", "set-url", "origin", url)
	_ = resetCmd.Run() // Ignore errors, not critical

	return destPath, nil
}

// FetchRepository fetches updates for a repository.
func (w *Worker) FetchRepository(
	ctx context.Context,
	repoPath string,
	conn *repos.Connection,
) error {
	ctx, span := tracing.StartSpan(ctx, "git.fetch")
	defer span.End()

	tracing.SetAttributes(ctx, attribute.String("git.repo_path", repoPath))

	w.logger.Info("Fetching repository", zap.String("path", repoPath))

	// For fetch, we need to update the remote URL with auth
	// Get current remote URL
	getURLCmd := exec.CommandContext(ctx, "git", "-C", repoPath, "remote", "get-url", "origin")

	urlOutput, err := getURLCmd.Output()
	if err == nil {
		remoteURL := strings.TrimSpace(string(urlOutput))

		authURL := gitutil.AddAuthToURL(remoteURL, conn)
		if authURL != remoteURL {
			// Temporarily set authenticated URL
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
			if err := setURLCmd.Run(); err != nil {
				w.logger.Warn("Failed to set authenticated URL", zap.Error(err))
			}

			defer func() {
				// Reset to original URL (without token)
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
				_ = resetCmd.Run()
			}()
		}
	}

	start := time.Now()
	// For bare repos, we need to explicitly update refs/heads/* to get branch updates
	// Using '+refs/heads/*:refs/heads/*' forces update of all branches
	cmd := exec.CommandContext(
		ctx,
		"git",
		"-C",
		repoPath,
		"fetch",
		"origin",
		"+refs/heads/*:refs/heads/*",
		"--prune",
	)

	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	if err != nil {
		// Record metrics and tracing for failure
		metrics.RecordGitFetch(duration, false)
		metrics.RecordError("git", "fetch_failed")
		tracing.RecordError(ctx, err)

		// Sanitize output to remove any credentials
		sanitizedOutput := gitutil.SanitizeGitOutput(string(output), conn)

		return fmt.Errorf("git fetch failed: %w\nOutput: %s", err, sanitizedOutput)
	}

	// Record metrics for success
	metrics.RecordGitFetch(duration, true)
	tracing.SetOK(ctx)
	tracing.AddEvent(
		ctx,
		"fetch.completed",
		attribute.Float64("duration_seconds", duration.Seconds()),
	)

	return nil
}

// categorizeIndexFailure categorizes indexing failures for better observability.
// Returns one of: oom_killed, timeout, git_error, zoekt_error, unknown
func categorizeIndexFailure(err error) string {
	if err == nil {
		return "unknown"
	}

	errStr := strings.ToLower(err.Error())

	// Check for OOM kill signals
	if strings.Contains(errStr, "killed") ||
		strings.Contains(errStr, "oom") ||
		strings.Contains(errStr, "out of memory") ||
		strings.Contains(errStr, "signal: killed") {
		return "oom_killed"
	}

	// Check for timeout
	if strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "context deadline exceeded") ||
		strings.Contains(errStr, "deadline") {
		return "timeout"
	}

	// Check for git errors
	if strings.Contains(errStr, "git") ||
		strings.Contains(errStr, "clone") ||
		strings.Contains(errStr, "fetch") ||
		strings.Contains(errStr, "repository") {
		return "git_error"
	}

	// Check for zoekt-specific errors
	if strings.Contains(errStr, "zoekt") ||
		strings.Contains(errStr, "index") {
		return "zoekt_error"
	}

	return "unknown"
}
