package queue

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// JobType represents the type of job.
type JobType string

const (
	JobTypeIndex   JobType = "index"
	JobTypeReplace JobType = "replace"
	JobTypeSync    JobType = "sync"
	JobTypeCleanup JobType = "cleanup"
)

// JobStatus represents the status of a job.
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
)

// Retry configuration defaults.
const (
	// DefaultMaxAttempts is the default number of retry attempts for jobs.
	DefaultMaxAttempts = 3

	// DefaultRetryBaseDelay is the base delay for exponential backoff.
	DefaultRetryBaseDelay = 30 * time.Second

	// DefaultRetryMaxDelay is the maximum delay between retries.
	DefaultRetryMaxDelay = 30 * time.Minute

	// DefaultJobTTL is the default time-to-live for job data in Redis.
	// Jobs are automatically expired after this duration.
	DefaultJobTTL = 24 * time.Hour

	// DefaultJobClaimTTL is the default time a worker can hold a job claim.
	// If a worker doesn't heartbeat within this duration, the job can be reclaimed.
	DefaultJobClaimTTL = 5 * time.Minute
)

// Job represents a queued job.
type Job struct {
	ID          string          `json:"id"`
	Type        JobType         `json:"type"`
	Status      JobStatus       `json:"status"`
	Payload     json.RawMessage `json:"payload"`
	Error       string          `json:"error,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	StartedAt   *time.Time      `json:"started_at,omitempty"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
	Progress    *JobProgress    `json:"progress,omitempty"`
	// Retry fields
	Attempts    int        `json:"attempts"`               // Number of attempts so far
	MaxAttempts int        `json:"max_attempts,omitempty"` // Max attempts (0 = use default)
	NextRetryAt *time.Time `json:"next_retry_at,omitempty"`
	LastError   string     `json:"last_error,omitempty"` // Error from last attempt
}

// JobProgress tracks the progress of a running job.
type JobProgress struct {
	Current int    `json:"current"`
	Total   int    `json:"total"`
	Message string `json:"message,omitempty"`
}

// IndexPayload is the payload for index jobs.
type IndexPayload struct {
	RepositoryID int64    `json:"repository_id"`
	ConnectionID int64    `json:"connection_id"`
	RepoName     string   `json:"repo_name"`
	CloneURL     string   `json:"clone_url"`
	Branch       string   `json:"branch"`             // Default/primary branch (backward compat)
	Branches     []string `json:"branches,omitempty"` // Additional branches to index
}

// ReplaceMatch represents a file match for replacement (from preview).
type ReplaceMatch struct {
	RepositoryID   int64  `json:"repository_id"`
	RepositoryName string `json:"repository_name"`
	FilePath       string `json:"file_path"`
}

// ReplacePayload is the payload for replace jobs.
type ReplacePayload struct {
	// Search parameters (kept for reference/logging, not used for search)
	SearchPattern string   `json:"search_pattern"`
	ReplaceWith   string   `json:"replace_with"`
	IsRegex       bool     `json:"is_regex"`
	CaseSensitive bool     `json:"case_sensitive"`
	FilePatterns  []string `json:"file_patterns,omitempty"`

	// Matches from preview (required - Execute uses these directly)
	Matches []ReplaceMatch `json:"matches"`

	// MR/PR options (MR is always created)
	BranchName    string `json:"branch_name,omitempty"`
	MRTitle       string `json:"mr_title,omitempty"`
	MRDescription string `json:"mr_description,omitempty"`

	// User-provided tokens for repos without server-side authentication
	// Map of connection_id (as string) -> token
	UserTokens map[string]string `json:"user_tokens,omitempty"`

	// ReposReadOnly indicates if the system is in read-only mode
	// When true, only user-provided tokens can be used (DB tokens are for indexing only)
	ReposReadOnly bool `json:"repos_readonly,omitempty"`
}

// SyncPayload is the payload for sync jobs.
type SyncPayload struct {
	ConnectionID int64 `json:"connection_id"`
}

// CleanupPayload is the payload for cleanup jobs (deleting Zoekt shards and repo files).
type CleanupPayload struct {
	RepositoryID   int64  `json:"repository_id"`
	RepositoryName string `json:"repository_name"`
	DataDir        string `json:"data_dir"` // Base data directory for repos and index
}

// Queue handles job queue operations.
type Queue struct {
	client           *redis.Client
	queueKey         string // Normal priority queue (index, sync, cleanup)
	priorityQueueKey string // High priority queue (replace jobs)
	retryQueueKey    string // Sorted set for delayed retry jobs (score = retry timestamp)
	jobPrefix        string
	activeIndexKey   string // SET of repo IDs with active index jobs
	activeSyncKey    string // SET of connection IDs with active sync jobs
	activeCleanupKey string // SET of repo IDs with active cleanup jobs
	// Sorted set indexes for efficient queries (score = unix timestamp in milliseconds)
	jobIndexKey       string // All jobs sorted by creation time
	statusIndexPrefix string // Jobs by status: codesearch:jobs:status:{status}
	typeIndexPrefix   string // Jobs by type: codesearch:jobs:type:{type}
}

// NewQueue creates a new job queue.
func NewQueue(client *redis.Client) *Queue {
	return &Queue{
		client:            client,
		queueKey:          "codesearch:jobs:queue",
		priorityQueueKey:  "codesearch:jobs:priority",
		retryQueueKey:     "codesearch:jobs:retry",
		jobPrefix:         "codesearch:job:",
		activeIndexKey:    "codesearch:active:index",
		activeSyncKey:     "codesearch:active:sync",
		activeCleanupKey:  "codesearch:active:cleanup",
		jobIndexKey:       "codesearch:jobs:index",
		statusIndexPrefix: "codesearch:jobs:status:",
		typeIndexPrefix:   "codesearch:jobs:type:",
	}
}

// Client returns the underlying Redis client.
func (q *Queue) Client() *redis.Client {
	return q.client
}

// statusIndexKey returns the sorted set key for a specific status.
func (q *Queue) statusIndexKey(status JobStatus) string {
	return q.statusIndexPrefix + string(status)
}

// typeIndexKey returns the sorted set key for a specific job type.
func (q *Queue) typeIndexKey(jobType JobType) string {
	return q.typeIndexPrefix + string(jobType)
}

// addToIndexes adds a job to all relevant sorted set indexes.
// Score is the creation timestamp in milliseconds for consistent ordering.
func (q *Queue) addToIndexes(ctx context.Context, job *Job) error {
	score := float64(job.CreatedAt.UnixMilli())
	pipe := q.client.Pipeline()

	// Add to main index
	pipe.ZAdd(ctx, q.jobIndexKey, redis.Z{Score: score, Member: job.ID})
	// Add to status index
	pipe.ZAdd(ctx, q.statusIndexKey(job.Status), redis.Z{Score: score, Member: job.ID})
	// Add to type index
	pipe.ZAdd(ctx, q.typeIndexKey(job.Type), redis.Z{Score: score, Member: job.ID})

	_, err := pipe.Exec(ctx)

	return err
}

// updateStatusIndex moves a job from one status index to another.
func (q *Queue) updateStatusIndex(
	ctx context.Context,
	jobID string,
	oldStatus, newStatus JobStatus,
	score float64,
) error {
	if oldStatus == newStatus {
		return nil
	}

	pipe := q.client.Pipeline()
	pipe.ZRem(ctx, q.statusIndexKey(oldStatus), jobID)
	pipe.ZAdd(ctx, q.statusIndexKey(newStatus), redis.Z{Score: score, Member: jobID})
	_, err := pipe.Exec(ctx)

	return err
}

// removeFromIndexes removes a job from all sorted set indexes.
func (q *Queue) removeFromIndexes(
	ctx context.Context,
	jobID string,
	jobType JobType,
	status JobStatus,
) error {
	pipe := q.client.Pipeline()
	pipe.ZRem(ctx, q.jobIndexKey, jobID)
	pipe.ZRem(ctx, q.statusIndexKey(status), jobID)
	pipe.ZRem(ctx, q.typeIndexKey(jobType), jobID)
	_, err := pipe.Exec(ctx)

	return err
}

// TLSConfig holds TLS configuration options for Redis connections.
type TLSConfig struct {
	Enabled    bool   // Enable TLS
	SkipVerify bool   // Skip certificate verification (insecure)
	CertFile   string // Path to client certificate file (for mTLS)
	KeyFile    string // Path to client key file (for mTLS)
	CACertFile string // Path to CA certificate file
	ServerName string // Override server name for TLS verification
}

// Connect creates a new Redis client.
func Connect(addr, password string, db int) (*redis.Client, error) {
	return ConnectWithTLS(addr, password, db, nil)
}

// ConnectWithTLS creates a new Redis client with optional TLS configuration.
func ConnectWithTLS(addr, password string, db int, tlsCfg *TLSConfig) (*redis.Client, error) {
	opts := &redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,

		// Connection pool settings for resilience under load
		PoolSize:     50,               // Maximum number of connections
		MinIdleConns: 5,                // Keep some connections ready
		PoolTimeout:  30 * time.Second, // Wait for a connection from pool

		// Timeout settings to handle slow Redis during fsync
		DialTimeout:  10 * time.Second, // Connection establishment timeout
		ReadTimeout:  30 * time.Second, // Read operation timeout (handles slow fsync)
		WriteTimeout: 30 * time.Second, // Write operation timeout

		// Retry settings for transient failures
		MaxRetries:      3,                      // Retry failed commands up to 3 times
		MinRetryBackoff: 100 * time.Millisecond, // Initial backoff
		MaxRetryBackoff: 2 * time.Second,        // Maximum backoff
	}

	// Configure TLS if enabled
	if tlsCfg != nil && tlsCfg.Enabled {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: tlsCfg.SkipVerify, //nolint:gosec // User-configurable for self-signed certs
		}

		// Set server name if provided
		if tlsCfg.ServerName != "" {
			tlsConfig.ServerName = tlsCfg.ServerName
		}

		// Load CA certificate if provided
		if tlsCfg.CACertFile != "" {
			caCert, err := os.ReadFile(tlsCfg.CACertFile)
			if err != nil {
				return nil, fmt.Errorf("read CA cert file: %w", err)
			}

			caCertPool := x509.NewCertPool()
			if !caCertPool.AppendCertsFromPEM(caCert) {
				return nil, errors.New("failed to parse CA certificate")
			}

			tlsConfig.RootCAs = caCertPool
		}

		// Load client certificate if provided (for mTLS)
		if tlsCfg.CertFile != "" && tlsCfg.KeyFile != "" {
			cert, err := tls.LoadX509KeyPair(tlsCfg.CertFile, tlsCfg.KeyFile)
			if err != nil {
				return nil, fmt.Errorf("load client certificate: %w", err)
			}

			tlsConfig.Certificates = []tls.Certificate{cert}
		}

		opts.TLSConfig = tlsConfig
	}

	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Ping(ctx).Err()
	if err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	return client, nil
}

// Enqueue adds a job to the queue.
// Replace jobs are added to a high-priority queue that is processed first.
func (q *Queue) Enqueue(ctx context.Context, jobType JobType, payload any) (*Job, error) {
	return q.EnqueueWithOptions(ctx, jobType, payload, 0)
}

// EnqueueWithOptions adds a job to the queue with custom retry settings.
// maxAttempts of 0 uses the default (DefaultMaxAttempts).
func (q *Queue) EnqueueWithOptions(
	ctx context.Context,
	jobType JobType,
	payload any,
	maxAttempts int,
) (*Job, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	job := &Job{
		ID:          strconv.FormatInt(time.Now().UnixNano(), 10),
		Type:        jobType,
		Status:      JobStatusPending,
		Payload:     payloadJSON,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Attempts:    0,
		MaxAttempts: maxAttempts,
	}

	jobJSON, err := json.Marshal(job)
	if err != nil {
		return nil, fmt.Errorf("marshal job: %w", err)
	}

	// Store job data
	jobKey := q.jobPrefix + job.ID
	if err := q.client.Set(ctx, jobKey, jobJSON, DefaultJobTTL).Err(); err != nil {
		return nil, fmt.Errorf("store job: %w", err)
	}

	// Add to sorted set indexes for efficient queries
	// Non-fatal: job is still in queue, just not indexed
	// This maintains backward compatibility during migration
	_ = q.addToIndexes(ctx, job)

	// Add to appropriate queue based on job type
	// Replace jobs go to priority queue, others to normal queue
	queueKey := q.queueKey
	if jobType == JobTypeReplace {
		queueKey = q.priorityQueueKey
	}

	if err := q.client.LPush(ctx, queueKey, job.ID).Err(); err != nil {
		return nil, fmt.Errorf("enqueue job: %w", err)
	}

	return job, nil
}

// ErrJobAlreadyExists is returned when trying to enqueue a duplicate job.
var ErrJobAlreadyExists = errors.New("job already exists")

// FindPendingJob searches for an existing pending or running job of the given type
// that matches the provided key extractor function.
// The keyExtractor is called for each job payload and should return a comparable key.
func (q *Queue) FindPendingJob(
	ctx context.Context,
	jobType JobType,
	keyExtractor func(payload json.RawMessage) (string, bool),
) (*Job, error) {
	// Use sorted set indexes for efficient lookup instead of SCAN
	// Check both pending and running status indexes
	for _, status := range []JobStatus{JobStatusPending, JobStatusRunning} {
		jobIDs, err := q.client.ZRange(ctx, q.statusIndexKey(status), 0, -1).Result()
		if err != nil {
			continue
		}

		for _, jobID := range jobIDs {
			job, err := q.GetJob(ctx, jobID)
			if err != nil || job == nil {
				continue
			}

			// Check type
			if job.Type != jobType {
				continue
			}

			// Double-check status (in case of race condition)
			if job.Status != JobStatusPending && job.Status != JobStatusRunning {
				continue
			}

			// Check if key matches
			if key, ok := keyExtractor(job.Payload); ok && key != "" {
				return job, nil
			}
		}
	}

	return nil, nil
}

// HasPendingIndexJob checks if there's already a pending/running index job for the given repository.
// Uses O(1) Redis SET lookup instead of scanning all jobs.
func (q *Queue) HasPendingIndexJob(ctx context.Context, repoID int64) (bool, error) {
	return q.client.SIsMember(ctx, q.activeIndexKey, strconv.FormatInt(repoID, 10)).Result()
}

// TryAcquireIndexJob atomically checks and marks a repo as having an active index job.
// Returns true if the job was acquired (repo was not already active), false if already active.
// This eliminates race conditions between HasPendingIndexJob and MarkIndexJobActive.
func (q *Queue) TryAcquireIndexJob(ctx context.Context, repoID int64) (bool, error) {
	// SADD returns 1 if the member was added (didn't exist), 0 if it already existed
	added, err := q.client.SAdd(ctx, q.activeIndexKey, strconv.FormatInt(repoID, 10)).Result()
	if err != nil {
		return false, err
	}

	return added == 1, nil
}

// MarkIndexJobActive adds a repo ID to the active index jobs set.
// Deprecated: Use TryAcquireIndexJob for race-free job acquisition.
func (q *Queue) MarkIndexJobActive(ctx context.Context, repoID int64) error {
	return q.client.SAdd(ctx, q.activeIndexKey, strconv.FormatInt(repoID, 10)).Err()
}

// MarkIndexJobInactive removes a repo ID from the active index jobs set.
func (q *Queue) MarkIndexJobInactive(ctx context.Context, repoID int64) error {
	return q.client.SRem(ctx, q.activeIndexKey, strconv.FormatInt(repoID, 10)).Err()
}

// GetActiveIndexRepos returns all repo IDs currently in the active index jobs set.
func (q *Queue) GetActiveIndexRepos(ctx context.Context) ([]int64, error) {
	members, err := q.client.SMembers(ctx, q.activeIndexKey).Result()
	if err != nil {
		return nil, fmt.Errorf("get active index repos: %w", err)
	}

	repoIDs := make([]int64, 0, len(members))
	for _, member := range members {
		repoID, err := strconv.ParseInt(member, 10, 64)
		if err != nil {
			// Skip invalid entries
			continue
		}
		repoIDs = append(repoIDs, repoID)
	}

	return repoIDs, nil
}

// HasActiveIndexJob checks if a repo has any pending or running index job.
// This is used for recovery to detect orphaned active index markers.
func (q *Queue) HasActiveIndexJob(ctx context.Context, repoID int64) (bool, error) {
	// Get all index jobs from the type index
	indexKey := q.typeIndexKey(JobTypeIndex)
	jobIDs, err := q.client.ZRange(ctx, indexKey, 0, -1).Result()
	if err != nil {
		return false, fmt.Errorf("get index jobs: %w", err)
	}

	// Check each job to see if it matches this repo and is pending/running
	for _, jobID := range jobIDs {
		jobKey := q.jobPrefix + jobID
		jobJSON, err := q.client.Get(ctx, jobKey).Result()
		if err != nil {
			continue
		}

		var job Job
		if err := json.Unmarshal([]byte(jobJSON), &job); err != nil {
			continue
		}

		// Only check pending and running jobs
		if job.Status != JobStatusPending && job.Status != JobStatusRunning {
			continue
		}

		// Parse the payload to get repository ID
		var payload IndexPayload
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			continue
		}

		if payload.RepositoryID == repoID {
			return true, nil
		}
	}

	return false, nil
}

// HasPendingSyncJob checks if there's already a pending/running sync job for the given connection.
// Uses O(1) Redis SET lookup instead of scanning all jobs.
func (q *Queue) HasPendingSyncJob(ctx context.Context, connectionID int64) (bool, error) {
	return q.client.SIsMember(ctx, q.activeSyncKey, strconv.FormatInt(connectionID, 10)).Result()
}

// TryAcquireSyncJob atomically checks and marks a connection as having an active sync job.
// Returns true if the job was acquired (connection was not already active), false if already active.
// This eliminates race conditions between HasPendingSyncJob and MarkSyncJobActive.
func (q *Queue) TryAcquireSyncJob(ctx context.Context, connectionID int64) (bool, error) {
	added, err := q.client.SAdd(ctx, q.activeSyncKey, strconv.FormatInt(connectionID, 10)).Result()
	if err != nil {
		return false, err
	}

	return added == 1, nil
}

// MarkSyncJobActive adds a connection ID to the active sync jobs set.
// Deprecated: Use TryAcquireSyncJob for race-free job acquisition.
func (q *Queue) MarkSyncJobActive(ctx context.Context, connectionID int64) error {
	return q.client.SAdd(ctx, q.activeSyncKey, strconv.FormatInt(connectionID, 10)).Err()
}

// MarkSyncJobInactive removes a connection ID from the active sync jobs set.
func (q *Queue) MarkSyncJobInactive(ctx context.Context, connectionID int64) error {
	return q.client.SRem(ctx, q.activeSyncKey, strconv.FormatInt(connectionID, 10)).Err()
}

// HasPendingCleanupJob checks if there's already a pending/running cleanup job for the given repository.
func (q *Queue) HasPendingCleanupJob(ctx context.Context, repoID int64) (bool, error) {
	return q.client.SIsMember(ctx, q.activeCleanupKey, strconv.FormatInt(repoID, 10)).Result()
}

// TryAcquireCleanupJob atomically checks and marks a repo as having an active cleanup job.
// Returns true if the job was acquired (repo was not already active), false if already active.
// This eliminates race conditions between HasPendingCleanupJob and MarkCleanupJobActive.
func (q *Queue) TryAcquireCleanupJob(ctx context.Context, repoID int64) (bool, error) {
	added, err := q.client.SAdd(ctx, q.activeCleanupKey, strconv.FormatInt(repoID, 10)).Result()
	if err != nil {
		return false, err
	}

	return added == 1, nil
}

// MarkCleanupJobActive adds a repo ID to the active cleanup jobs set.
// Deprecated: Use TryAcquireCleanupJob for race-free job acquisition.
func (q *Queue) MarkCleanupJobActive(ctx context.Context, repoID int64) error {
	return q.client.SAdd(ctx, q.activeCleanupKey, strconv.FormatInt(repoID, 10)).Err()
}

// MarkCleanupJobInactive removes a repo ID from the active cleanup jobs set.
func (q *Queue) MarkCleanupJobInactive(ctx context.Context, repoID int64) error {
	return q.client.SRem(ctx, q.activeCleanupKey, strconv.FormatInt(repoID, 10)).Err()
}

// Dequeue retrieves the next job from the queue.
// Priority queue (replace jobs) is checked first, then normal queue.
func (q *Queue) Dequeue(ctx context.Context, timeout time.Duration) (*Job, error) {
	// First, try to get a job from the priority queue (non-blocking)
	result, err := q.client.RPop(ctx, q.priorityQueueKey).Result()
	if err == nil && result != "" {
		return q.GetJob(ctx, result)
	}

	// If no priority job, block on both queues
	// BRPop will return from whichever queue has a job first
	// Priority queue is listed first so it's preferred when both have jobs
	result2, err := q.client.BRPop(ctx, timeout, q.priorityQueueKey, q.queueKey).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil // No job available
	}

	if err != nil {
		return nil, fmt.Errorf("dequeue job: %w", err)
	}

	if len(result2) < 2 {
		return nil, nil
	}

	jobID := result2[1]

	return q.GetJob(ctx, jobID)
}

// GetJob retrieves a job by ID.
func (q *Queue) GetJob(ctx context.Context, jobID string) (*Job, error) {
	jobKey := q.jobPrefix + jobID

	jobJSON, err := q.client.Get(ctx, jobKey).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("get job: %w", err)
	}

	var job Job
	if err := json.Unmarshal([]byte(jobJSON), &job); err != nil {
		return nil, fmt.Errorf("unmarshal job: %w", err)
	}

	return &job, nil
}

// UpdateStatus updates the status of a job.
func (q *Queue) UpdateStatus(
	ctx context.Context,
	jobID string,
	status JobStatus,
	errorMsg string,
) error {
	job, err := q.GetJob(ctx, jobID)
	if err != nil {
		return err
	}

	if job == nil {
		return fmt.Errorf("job not found: %s", jobID)
	}

	oldStatus := job.Status
	job.Status = status
	job.Error = errorMsg
	job.UpdatedAt = time.Now()

	// Set timestamps based on status
	now := time.Now()
	if status == JobStatusRunning && job.StartedAt == nil {
		job.StartedAt = &now
	}

	if status == JobStatusCompleted || status == JobStatusFailed {
		job.CompletedAt = &now
	}

	jobJSON, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}

	jobKey := q.jobPrefix + jobID
	if err := q.client.Set(ctx, jobKey, jobJSON, DefaultJobTTL).Err(); err != nil {
		return fmt.Errorf("update job: %w", err)
	}

	// Update status index (move from old status to new status)
	if oldStatus != status {
		score := float64(job.CreatedAt.UnixMilli())
		_ = q.updateStatusIndex(ctx, jobID, oldStatus, status, score)
	}

	return nil
}

// MarkRunning marks a job as running.
func (q *Queue) MarkRunning(ctx context.Context, jobID string) error {
	return q.UpdateStatus(ctx, jobID, JobStatusRunning, "")
}

// MarkCompleted marks a job as completed.
func (q *Queue) MarkCompleted(ctx context.Context, jobID string) error {
	return q.UpdateStatus(ctx, jobID, JobStatusCompleted, "")
}

// MarkFailed marks a job as failed.
func (q *Queue) MarkFailed(ctx context.Context, jobID string, err error) error {
	return q.UpdateStatus(ctx, jobID, JobStatusFailed, err.Error())
}

// CalculateRetryDelay calculates the delay before the next retry using exponential backoff.
// Formula: min(baseDelay * 2^(attempt-1), maxDelay).
func CalculateRetryDelay(attempt int) time.Duration {
	if attempt <= 0 {
		attempt = 1
	}

	// Cap the exponent to prevent overflow (2^10 = 1024 is plenty)
	exponent := min(attempt-1, 10)

	// Exponential backoff: 30s, 60s, 120s, 240s, etc.
	delay := min(DefaultRetryBaseDelay*time.Duration(1<<uint(exponent)), DefaultRetryMaxDelay)

	return delay
}

// ShouldRetry determines if a job should be retried based on its attempt count.
func (j *Job) ShouldRetry() bool {
	maxAttempts := j.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = DefaultMaxAttempts
	}

	return j.Attempts < maxAttempts
}

// ScheduleRetry schedules a job for retry with exponential backoff.
// Returns true if the job was scheduled for retry, false if max attempts exceeded.
func (q *Queue) ScheduleRetry(ctx context.Context, jobID string, jobErr error) (bool, error) {
	job, err := q.GetJob(ctx, jobID)
	if err != nil {
		return false, err
	}

	if job == nil {
		return false, fmt.Errorf("job not found: %s", jobID)
	}

	// Increment attempts
	job.Attempts++
	job.LastError = jobErr.Error()
	job.UpdatedAt = time.Now()

	// Track old status for index update
	oldStatus := job.Status

	// Check if we should retry
	if !job.ShouldRetry() {
		// No more retries - mark as permanently failed
		job.Status = JobStatusFailed
		job.Error = fmt.Sprintf("max attempts (%d) exceeded: %s", job.Attempts, jobErr.Error())
		now := time.Now()
		job.CompletedAt = &now

		jobJSON, err := json.Marshal(job)
		if err != nil {
			return false, fmt.Errorf("marshal job: %w", err)
		}

		jobKey := q.jobPrefix + jobID
		if err := q.client.Set(ctx, jobKey, jobJSON, DefaultJobTTL).Err(); err != nil {
			return false, fmt.Errorf("update job: %w", err)
		}

		// Update status index (move from old status to failed)
		score := float64(job.CreatedAt.UnixMilli())
		_ = q.updateStatusIndex(ctx, jobID, oldStatus, JobStatusFailed, score)

		return false, nil
	}

	// Calculate retry time
	delay := CalculateRetryDelay(job.Attempts)
	retryAt := time.Now().Add(delay)
	job.NextRetryAt = &retryAt
	job.Status = JobStatusPending // Reset to pending for retry
	job.StartedAt = nil           // Clear start time
	job.Progress = nil            // Clear progress

	jobJSON, err := json.Marshal(job)
	if err != nil {
		return false, fmt.Errorf("marshal job: %w", err)
	}

	jobKey := q.jobPrefix + jobID

	// Use pipeline for atomic operations
	pipe := q.client.Pipeline()
	pipe.Set(ctx, jobKey, jobJSON, DefaultJobTTL)
	// Add to retry sorted set with score = retry timestamp
	pipe.ZAdd(ctx, q.retryQueueKey, redis.Z{
		Score:  float64(retryAt.Unix()),
		Member: jobID,
	})

	_, err = pipe.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("schedule retry: %w", err)
	}

	// Update status index (move from old status to pending)
	score := float64(job.CreatedAt.UnixMilli())
	_ = q.updateStatusIndex(ctx, jobID, oldStatus, JobStatusPending, score)

	return true, nil
}

// ProcessRetryQueue moves jobs from the retry queue to the main queue if their retry time has passed.
// This should be called periodically (e.g., every 10 seconds).
func (q *Queue) ProcessRetryQueue(ctx context.Context) (int, error) {
	now := time.Now().Unix()

	// Get jobs ready for retry (score <= now)
	jobIDs, err := q.client.ZRangeByScore(ctx, q.retryQueueKey, &redis.ZRangeBy{
		Min:   "-inf",
		Max:   strconv.FormatInt(now, 10),
		Count: 100, // Process up to 100 jobs per call
	}).Result()
	if err != nil {
		return 0, fmt.Errorf("get retry jobs: %w", err)
	}

	if len(jobIDs) == 0 {
		return 0, nil
	}

	processed := 0

	for _, jobID := range jobIDs {
		// Get job to determine which queue it should go to
		job, err := q.GetJob(ctx, jobID)
		if err != nil || job == nil {
			// Job doesn't exist, remove from retry queue
			q.client.ZRem(ctx, q.retryQueueKey, jobID)
			continue
		}

		// Clear retry time
		job.NextRetryAt = nil
		job.UpdatedAt = time.Now()

		jobJSON, err := json.Marshal(job)
		if err != nil {
			continue
		}

		// Determine target queue
		queueKey := q.queueKey
		if job.Type == JobTypeReplace {
			queueKey = q.priorityQueueKey
		}

		// Atomic: remove from retry queue, update job, push to main queue
		pipe := q.client.Pipeline()
		pipe.ZRem(ctx, q.retryQueueKey, jobID)
		pipe.Set(ctx, q.jobPrefix+jobID, jobJSON, DefaultJobTTL)
		pipe.LPush(ctx, queueKey, jobID)

		if _, err := pipe.Exec(ctx); err != nil {
			continue
		}

		processed++
	}

	return processed, nil
}

// GetRetryQueueLength returns the number of jobs waiting for retry.
func (q *Queue) GetRetryQueueLength(ctx context.Context) (int64, error) {
	return q.client.ZCard(ctx, q.retryQueueKey).Result()
}

// UpdateProgress updates the progress of a running job.
func (q *Queue) UpdateProgress(
	ctx context.Context,
	jobID string,
	current, total int,
	message string,
) error {
	job, err := q.GetJob(ctx, jobID)
	if err != nil {
		return err
	}

	if job == nil {
		return fmt.Errorf("job not found: %s", jobID)
	}

	job.Progress = &JobProgress{
		Current: current,
		Total:   total,
		Message: message,
	}
	job.UpdatedAt = time.Now()

	jobJSON, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}

	jobKey := q.jobPrefix + jobID
	if err := q.client.Set(ctx, jobKey, jobJSON, DefaultJobTTL).Err(); err != nil {
		return fmt.Errorf("update job: %w", err)
	}

	return nil
}

// JobListOptions represents options for listing jobs.
type JobListOptions struct {
	Type          JobType    // Filter by job type (empty = all)
	Status        JobStatus  // Filter by status (empty = all)
	ExcludeStatus JobStatus  // Exclude jobs with this status (empty = none)
	RepoName      string     // Filter by repo name (partial match, empty = all)
	CreatedAfter  *time.Time // Filter to jobs created after this time (nil = no filter)
	Limit         int        // Max results (default 50)
	Offset        int        // Pagination offset
}

// JobListResult represents the result of listing jobs.
type JobListResult struct {
	Jobs       []*Job `json:"jobs"`
	TotalCount int    `json:"total_count"`
	Limit      int    `json:"limit"`
	Offset     int    `json:"offset"`
	HasMore    bool   `json:"has_more"`
}

// ListJobsWithOptions returns jobs with filtering and pagination.
// Uses sorted set indexes for efficient queries instead of SCAN.
func (q *Queue) ListJobsWithOptions(
	ctx context.Context,
	opts JobListOptions,
) (*JobListResult, error) {
	if opts.Limit <= 0 {
		opts.Limit = 50
	}

	if opts.Limit > 10000 {
		opts.Limit = 10000
	}

	// Determine which index to use based on filters
	var indexKey string
	if opts.Status != "" {
		// Use status-specific index
		indexKey = q.statusIndexKey(opts.Status)
	} else if opts.Type != "" {
		// Use type-specific index
		indexKey = q.typeIndexKey(opts.Type)
	} else {
		// Use main index
		indexKey = q.jobIndexKey
	}

	// Get all job IDs from the index (sorted by creation time, newest first)
	// ZREVRANGE returns in reverse order (highest score first = newest)
	jobIDs, err := q.client.ZRevRange(ctx, indexKey, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("get job index: %w", err)
	}

	if len(jobIDs) == 0 {
		return &JobListResult{
			Jobs:       []*Job{},
			TotalCount: 0,
			Limit:      opts.Limit,
			Offset:     opts.Offset,
			HasMore:    false,
		}, nil
	}

	// Batch fetch job data using MGET
	const mgetBatchSize = 500

	var allJobs []*Job

	for i := 0; i < len(jobIDs); i += mgetBatchSize {
		end := min(i+mgetBatchSize, len(jobIDs))
		batchIDs := jobIDs[i:end]

		// Build keys for MGET
		keys := make([]string, len(batchIDs))
		for j, id := range batchIDs {
			keys[j] = q.jobPrefix + id
		}

		values, err := q.client.MGet(ctx, keys...).Result()
		if err != nil {
			return nil, fmt.Errorf("mget jobs: %w", err)
		}

		for _, val := range values {
			if val == nil {
				continue
			}

			jobJSON, ok := val.(string)
			if !ok {
				continue
			}

			var job Job
			if err := json.Unmarshal([]byte(jobJSON), &job); err != nil {
				continue
			}

			// Always validate filters against actual job data
			// This handles stale index entries where job status changed but index wasn't updated
			if opts.Type != "" && job.Type != opts.Type {
				continue
			}

			if opts.Status != "" && job.Status != opts.Status {
				continue
			}

			// Exclude status filter
			if opts.ExcludeStatus != "" && job.Status == opts.ExcludeStatus {
				continue
			}

			// Time filter - only include jobs created after the specified time
			if opts.CreatedAfter != nil && job.CreatedAt.Before(*opts.CreatedAfter) {
				continue
			}

			// Repo name filter (search in payload)
			if opts.RepoName != "" {
				repoName := ""

				// Extract repo name based on job type
				switch job.Type {
				case JobTypeIndex:
					var payload IndexPayload
					if err := json.Unmarshal(job.Payload, &payload); err == nil {
						repoName = payload.RepoName
					}
				case JobTypeCleanup:
					var payload CleanupPayload
					if err := json.Unmarshal(job.Payload, &payload); err == nil {
						repoName = payload.RepositoryName
					}
				case JobTypeSync:
					// Sync jobs don't have a repo name, skip filter
					continue
				case JobTypeReplace:
					// Replace jobs operate on multiple repos, skip filter for now
					continue
				}

				// Check if repo name matches filter (case-insensitive partial match)
				if repoName == "" || !strings.Contains(
					strings.ToLower(repoName),
					strings.ToLower(opts.RepoName),
				) {
					continue
				}
			}

			allJobs = append(allJobs, &job)
		}
	}

	// Jobs are already sorted by creation time (newest first) from ZREVRANGE
	totalCount := len(allJobs)

	// Apply pagination
	start := opts.Offset
	end := opts.Offset + opts.Limit

	if start >= len(allJobs) {
		return &JobListResult{
			Jobs:       []*Job{},
			TotalCount: totalCount,
			Limit:      opts.Limit,
			Offset:     opts.Offset,
			HasMore:    false,
		}, nil
	}

	if end > len(allJobs) {
		end = len(allJobs)
	}

	return &JobListResult{
		Jobs:       allJobs[start:end],
		TotalCount: totalCount,
		Limit:      opts.Limit,
		Offset:     opts.Offset,
		HasMore:    end < len(allJobs),
	}, nil
}

// QueueLength returns the number of pending jobs.
func (q *Queue) QueueLength(ctx context.Context) (int64, error) {
	return q.client.LLen(ctx, q.queueKey).Result()
}

// CleanupResult contains the result of a cleanup operation.
type CleanupResult struct {
	DeletedCount int `json:"deleted_count"`
	ScannedCount int `json:"scanned_count"`
}

// CleanupOldJobs removes completed and failed jobs older than the specified duration.
// Uses sorted set indexes for efficient range queries instead of SCAN.
func (q *Queue) CleanupOldJobs(ctx context.Context, maxAge time.Duration) (*CleanupResult, error) {
	cutoff := time.Now().Add(-maxAge)
	cutoffScore := float64(cutoff.UnixMilli())
	result := &CleanupResult{}

	// Process completed and failed jobs using their status indexes
	for _, status := range []JobStatus{JobStatusCompleted, JobStatusFailed} {
		indexKey := q.statusIndexKey(status)

		// Get jobs older than cutoff (score < cutoffScore)
		// ZRANGEBYSCORE with -inf to cutoffScore gets all old jobs
		jobIDs, err := q.client.ZRangeByScore(ctx, indexKey, &redis.ZRangeBy{
			Min: "-inf",
			Max: fmt.Sprintf("%f", cutoffScore),
		}).Result()
		if err != nil {
			continue
		}

		for _, jobID := range jobIDs {
			result.ScannedCount++

			// Get job to verify and get type for index cleanup
			job, err := q.GetJob(ctx, jobID)
			if err != nil || job == nil {
				// Job doesn't exist, clean up index entry
				q.client.ZRem(ctx, indexKey, jobID)
				continue
			}

			// Verify job is old enough (double-check with actual timestamp)
			jobTime := job.UpdatedAt
			if job.CompletedAt != nil {
				jobTime = *job.CompletedAt
			}

			if jobTime.Before(cutoff) {
				// Delete job data and remove from all indexes
				jobKey := q.jobPrefix + jobID
				if err := q.client.Del(ctx, jobKey).Err(); err == nil {
					_ = q.removeFromIndexes(ctx, jobID, job.Type, job.Status)

					// Clean up active job markers based on job type
					// This prevents repos/connections from getting stuck with stale active markers
					switch job.Type {
					case JobTypeIndex:
						var payload IndexPayload
						if err := json.Unmarshal(job.Payload, &payload); err == nil && payload.RepositoryID > 0 {
							_ = q.MarkIndexJobInactive(ctx, payload.RepositoryID)
						}
					case JobTypeSync:
						var payload SyncPayload
						if err := json.Unmarshal(job.Payload, &payload); err == nil && payload.ConnectionID > 0 {
							_ = q.MarkSyncJobInactive(ctx, payload.ConnectionID)
						}
					case JobTypeCleanup:
						var payload CleanupPayload
						if err := json.Unmarshal(job.Payload, &payload); err == nil && payload.RepositoryID > 0 {
							_ = q.MarkCleanupJobInactive(ctx, payload.RepositoryID)
						}
					}

					result.DeletedCount++
				}
			}
		}
	}

	return result, nil
}

// Clear removes all jobs from the queue.
func (q *Queue) Clear(ctx context.Context) error {
	err := q.client.Del(ctx, q.queueKey).Err()
	if err != nil {
		return fmt.Errorf("clear queue: %w", err)
	}

	return nil
}

// BulkActionResult represents the result of a bulk job operation.
type BulkActionResult struct {
	Processed int `json:"processed"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
}

// BulkCancelJobs cancels all jobs matching the given filters
// Only pending and running jobs can be canceled.
// Uses sorted set indexes for efficient queries instead of SCAN.
func (q *Queue) BulkCancelJobs(
	ctx context.Context,
	jobType JobType,
	status JobStatus,
) (*BulkActionResult, error) {
	result := &BulkActionResult{}

	// Determine which statuses to process
	statuses := []JobStatus{JobStatusPending, JobStatusRunning}
	if status != "" {
		// Only process the specified status if it's cancelable
		if status != JobStatusPending && status != JobStatusRunning {
			return result, nil // Can't cancel completed/failed jobs
		}

		statuses = []JobStatus{status}
	}

	for _, s := range statuses {
		indexKey := q.statusIndexKey(s)

		jobIDs, err := q.client.ZRange(ctx, indexKey, 0, -1).Result()
		if err != nil {
			continue
		}

		for _, jobID := range jobIDs {
			job, err := q.GetJob(ctx, jobID)
			if err != nil || job == nil {
				continue
			}

			// Apply type filter
			if jobType != "" && job.Type != jobType {
				continue
			}

			// Double-check status (in case of race)
			if job.Status != JobStatusPending && job.Status != JobStatusRunning {
				continue
			}

			result.Processed++

			// Cancel the job
			oldStatus := job.Status
			job.Status = JobStatusFailed
			job.Error = "Canceled by bulk operation"
			job.UpdatedAt = time.Now()
			now := time.Now()
			job.CompletedAt = &now

			updatedJSON, err := json.Marshal(job)
			if err != nil {
				result.Failed++
				continue
			}

			jobKey := q.jobPrefix + jobID
			if err := q.client.Set(ctx, jobKey, updatedJSON, DefaultJobTTL).Err(); err != nil {
				result.Failed++
				continue
			}

			// Update status index
			score := float64(job.CreatedAt.UnixMilli())
			_ = q.updateStatusIndex(ctx, jobID, oldStatus, JobStatusFailed, score)

			result.Succeeded++
		}
	}

	return result, nil
}

// BulkDeleteJobs deletes all jobs matching the given filters
// Only completed and failed jobs can be deleted.
// Uses sorted set indexes for efficient queries instead of SCAN.
func (q *Queue) BulkDeleteJobs(
	ctx context.Context,
	jobType JobType,
	status JobStatus,
) (*BulkActionResult, error) {
	result := &BulkActionResult{}

	// Determine which statuses to process
	var statuses []JobStatus
	if status != "" {
		statuses = []JobStatus{status}
	} else {
		// By default, only delete completed or failed jobs
		statuses = []JobStatus{JobStatusCompleted, JobStatusFailed}
	}

	for _, s := range statuses {
		indexKey := q.statusIndexKey(s)

		jobIDs, err := q.client.ZRange(ctx, indexKey, 0, -1).Result()
		if err != nil {
			continue
		}

		for _, jobID := range jobIDs {
			job, err := q.GetJob(ctx, jobID)
			if err != nil || job == nil {
				// Clean up orphaned index entry
				q.client.ZRem(ctx, indexKey, jobID)
				continue
			}

			// Apply type filter
			if jobType != "" && job.Type != jobType {
				continue
			}

			// Verify status matches (in case of race)
			if status != "" && job.Status != status {
				continue
			}

			// By default, only delete completed or failed jobs
			if status == "" && job.Status != JobStatusCompleted && job.Status != JobStatusFailed {
				continue
			}

			result.Processed++

			jobKey := q.jobPrefix + jobID
			if err := q.client.Del(ctx, jobKey).Err(); err != nil {
				result.Failed++
				continue
			}

			// Remove from all indexes
			_ = q.removeFromIndexes(ctx, jobID, job.Type, job.Status)

			result.Succeeded++
		}
	}

	return result, nil
}

// DeleteAllJobs deletes ALL jobs regardless of status (use with caution).
// Uses the main job index for efficient iteration instead of SCAN.
func (q *Queue) DeleteAllJobs(ctx context.Context) (*BulkActionResult, error) {
	result := &BulkActionResult{}

	// Get all job IDs from the main index
	jobIDs, err := q.client.ZRange(ctx, q.jobIndexKey, 0, -1).Result()
	if err != nil {
		return result, fmt.Errorf("get job index: %w", err)
	}

	for _, jobID := range jobIDs {
		result.Processed++

		// Get job to find type and status for index cleanup
		job, _ := q.GetJob(ctx, jobID)

		jobKey := q.jobPrefix + jobID
		if err := q.client.Del(ctx, jobKey).Err(); err != nil {
			result.Failed++
			continue
		}

		// Remove from all indexes
		if job != nil {
			_ = q.removeFromIndexes(ctx, jobID, job.Type, job.Status)
		} else {
			// Job data already gone, just remove from main index
			q.client.ZRem(ctx, q.jobIndexKey, jobID)
		}

		result.Succeeded++
	}

	// Clear all indexes completely to ensure clean state
	pipe := q.client.Pipeline()
	pipe.Del(ctx, q.jobIndexKey)
	pipe.Del(ctx, q.queueKey)
	pipe.Del(ctx, q.priorityQueueKey)
	// Clear all status indexes
	for _, status := range []JobStatus{JobStatusPending, JobStatusRunning, JobStatusCompleted, JobStatusFailed} {
		pipe.Del(ctx, q.statusIndexKey(status))
	}
	// Clear all type indexes
	for _, jobType := range []JobType{JobTypeIndex, JobTypeReplace, JobTypeSync, JobTypeCleanup} {
		pipe.Del(ctx, q.typeIndexKey(jobType))
	}

	_, _ = pipe.Exec(ctx)

	return result, nil
}

// RebuildJobIndexes scans all existing jobs and rebuilds the sorted set indexes.
// This is useful for fixing stale data from jobs created before the index feature
// or when indexes get out of sync due to bugs.
// Returns the number of jobs processed and any errors encountered.
func (q *Queue) RebuildJobIndexes(ctx context.Context) (int, error) {
	// First, clear all existing indexes
	pipe := q.client.Pipeline()
	pipe.Del(ctx, q.jobIndexKey)

	for _, status := range []JobStatus{JobStatusPending, JobStatusRunning, JobStatusCompleted, JobStatusFailed} {
		pipe.Del(ctx, q.statusIndexKey(status))
	}

	for _, jobType := range []JobType{JobTypeIndex, JobTypeReplace, JobTypeSync, JobTypeCleanup} {
		pipe.Del(ctx, q.typeIndexKey(jobType))
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return 0, fmt.Errorf("clear indexes: %w", err)
	}

	// Scan all job keys using SCAN
	processed := 0
	cursor := uint64(0)
	pattern := q.jobPrefix + "*"

	for {
		var (
			keys []string
			err  error
		)

		keys, cursor, err = q.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return processed, fmt.Errorf("scan jobs: %w", err)
		}

		if len(keys) == 0 && cursor == 0 {
			break
		}

		// Fetch all jobs in this batch
		for _, key := range keys {
			jobJSON, err := q.client.Get(ctx, key).Result()
			if err != nil {
				continue
			}

			var job Job
			if err := json.Unmarshal([]byte(jobJSON), &job); err != nil {
				continue
			}

			// Add to indexes
			if err := q.addToIndexes(ctx, &job); err != nil {
				continue
			}

			processed++
		}

		if cursor == 0 {
			break
		}
	}

	return processed, nil
}
