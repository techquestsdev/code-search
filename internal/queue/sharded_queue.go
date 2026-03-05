package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/techquestsdev/code-search/internal/log"
	"github.com/techquestsdev/code-search/internal/sharding"
)

// ShardedQueue extends Queue with shard-aware job processing.
type ShardedQueue struct {
	*Queue

	shardIndex  int
	totalShards int
	workerID    string
	enabled     bool
}

// NewShardedQueue creates a shard-aware queue.
func NewShardedQueue(client *redis.Client) *ShardedQueue {
	sq := &ShardedQueue{
		Queue:       NewQueue(client),
		shardIndex:  0,
		totalShards: 1,
		workerID:    fmt.Sprintf("worker-%d", time.Now().UnixNano()),
		enabled:     false,
	}

	// Read shard config from environment
	if totalStr := os.Getenv("TOTAL_SHARDS"); totalStr != "" {
		if total, err := strconv.Atoi(totalStr); err == nil && total > 1 {
			sq.totalShards = total
			sq.enabled = true
		}
	}

	if indexStr := os.Getenv("SHARD_INDEX"); indexStr != "" {
		if idx, err := strconv.Atoi(indexStr); err == nil {
			sq.shardIndex = idx
		}
	}

	if workerID := os.Getenv("HOSTNAME"); workerID != "" {
		sq.workerID = workerID
	}

	return sq
}

// processingKey returns the key for jobs being processed.
func (sq *ShardedQueue) processingKey() string {
	return "codesearch:jobs:processing"
}

// workerKey returns the key for tracking worker heartbeats.
func (sq *ShardedQueue) workerKey(jobID string) string {
	return fmt.Sprintf("codesearch:job:%s:worker", jobID)
}

// getShardForRepo returns which shard should handle a repo.
func getShardForRepo(repoName string, totalShards int) int {
	return sharding.GetShardForRepo(repoName, totalShards)
}

// DequeueForShard retrieves a job that belongs to this shard
// It will skip jobs that belong to other shards (re-queue them).
// Priority queue (replace jobs) is checked first, then normal queue.
func (sq *ShardedQueue) DequeueForShard(ctx context.Context, timeout time.Duration) (*Job, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		// First, try to get a job from the priority queue (non-blocking)
		// Replace jobs go to priority queue and should be processed first
		result, err := sq.client.RPop(ctx, sq.priorityQueueKey).Result()
		if err == nil && result != "" {
			job, err := sq.GetJob(ctx, result)
			if err == nil && job != nil {
				// Priority jobs (replace) don't need shard checks - any worker can handle them
				if err := sq.claimJob(ctx, job); err == nil {
					return job, nil
				}
				// Failed to claim, continue to try other jobs
			}
		}

		// Try to get a job from the normal queue
		remainingTimeout := max(time.Until(deadline), time.Second)

		// BRPop on both queues - priority first so it's preferred when both have jobs
		result2, err := sq.client.BRPop(ctx, remainingTimeout, sq.priorityQueueKey, sq.queueKey).
			Result()
		if errors.Is(err, redis.Nil) {
			return nil, nil // Timeout, no job
		}

		if err != nil {
			return nil, fmt.Errorf("dequeue job: %w", err)
		}

		if len(result2) < 2 {
			continue
		}

		jobID := result2[1]

		job, err := sq.GetJob(ctx, jobID)
		if err != nil || job == nil {
			continue
		}

		// Check if this job belongs to our shard (only for index jobs)
		if sq.enabled && job.Type == JobTypeIndex {
			var payload IndexPayload

			err := json.Unmarshal(job.Payload, &payload)
			if err == nil {
				targetShard := getShardForRepo(payload.RepoName, sq.totalShards)
				if targetShard != sq.shardIndex {
					// Not our shard - re-queue it for another worker
					sq.client.LPush(ctx, sq.queueKey, jobID)
					continue
				}
			}
		}

		// Claim this job - move to processing set with our worker ID
		if err := sq.claimJob(ctx, job); err != nil {
			// Failed to claim, someone else got it
			continue
		}

		return job, nil
	}

	return nil, nil
}

// claimJob atomically claims a job for this worker.
func (sq *ShardedQueue) claimJob(ctx context.Context, job *Job) error {
	workerKey := sq.workerKey(job.ID)

	// SET NX is atomic - only succeeds if key doesn't exist
	ok, err := sq.client.SetNX(ctx, workerKey, sq.workerID, DefaultJobClaimTTL).Result()
	if err != nil {
		return fmt.Errorf("claim job: %w", err)
	}

	if !ok {
		return errors.New("job already claimed")
	}

	// Add to processing set (idempotent, safe if we crash after SetNX)
	if err := sq.client.SAdd(ctx, sq.processingKey(), job.ID).Err(); err != nil {
		// Try to release the claim since we failed to track it
		sq.client.Del(ctx, workerKey)
		return fmt.Errorf("add to processing set: %w", err)
	}

	return nil
}

// Heartbeat updates the TTL for a job we're processing
// Call this periodically for long-running jobs.
func (sq *ShardedQueue) Heartbeat(ctx context.Context, jobID string) error {
	workerKey := sq.workerKey(jobID)
	return sq.client.Expire(ctx, workerKey, DefaultJobClaimTTL).Err()
}

// ReleaseJob releases a job from processing (on completion or failure).
func (sq *ShardedQueue) ReleaseJob(ctx context.Context, jobID string) error {
	// Remove from processing set and delete worker key
	pipe := sq.client.Pipeline()
	pipe.SRem(ctx, sq.processingKey(), jobID)
	pipe.Del(ctx, sq.workerKey(jobID))
	_, err := pipe.Exec(ctx)

	return err
}

// MarkCompletedAndRelease marks a job complete and releases it.
func (sq *ShardedQueue) MarkCompletedAndRelease(ctx context.Context, jobID string) error {
	err := sq.MarkCompleted(ctx, jobID)
	if err != nil {
		return err
	}

	return sq.ReleaseJob(ctx, jobID)
}

// MarkFailedAndRelease marks a job failed and releases it.
// If the job has retries remaining, it will be scheduled for retry instead of being marked as permanently failed.
func (sq *ShardedQueue) MarkFailedAndRelease(
	ctx context.Context,
	jobID string,
	jobErr error,
) error {
	// Try to schedule retry
	retried, err := sq.ScheduleRetry(ctx, jobID, jobErr)
	if err != nil {
		// If retry scheduling fails, fall back to marking as failed
		if markErr := sq.MarkFailed(ctx, jobID, jobErr); markErr != nil {
			return markErr
		}
	}

	// Release the job from processing regardless of retry status
	releaseErr := sq.ReleaseJob(ctx, jobID)

	// If job was not retried (max attempts exceeded), it's already marked as failed by ScheduleRetry
	// If job was retried, it's now in the retry queue waiting for its scheduled time
	_ = retried // Used implicitly by ScheduleRetry updating the job

	return releaseErr
}

// RecoverStaleJobs finds jobs that were being processed but worker died
// Call this periodically from a leader/coordinator.
func (sq *ShardedQueue) RecoverStaleJobs(ctx context.Context) (int, error) {
	// Get all jobs in processing set
	jobIDs, err := sq.client.SMembers(ctx, sq.processingKey()).Result()
	if err != nil {
		return 0, fmt.Errorf("get processing jobs: %w", err)
	}

	// Lua script for fully atomic recovery
	recoverScript := redis.NewScript(`
		local processingKey = KEYS[1]
		local queueKey = KEYS[2]
		local jobKey = KEYS[3]
		local workerKey = KEYS[4]
		local jobID = ARGV[1]
		local now = ARGV[2]
		
		-- Check if worker key still exists
		if redis.call('EXISTS', workerKey) == 1 then
			return 0  -- Worker still alive
		end
		
		-- Check if still in processing set (another recovery might have handled it)
		if redis.call('SISMEMBER', processingKey, jobID) == 0 then
			return 0  -- Already recovered
		end
		
		-- Get job data
		local jobData = redis.call('GET', jobKey)
		if not jobData then
			-- Job doesn't exist, just clean up
			redis.call('SREM', processingKey, jobID)
			return 0
		end
		
		-- Parse and update job status
		local job = cjson.decode(jobData)
		if job.status ~= 'running' then
			-- Job already completed/failed, just clean up
			redis.call('SREM', processingKey, jobID)
			return 0
		end
		
		-- Update job to pending status
		job.status = 'pending'
		job.updated_at = now
		redis.call('SET', jobKey, cjson.encode(job), 'EX', 86400)
		
		-- Remove from processing and re-queue
		redis.call('SREM', processingKey, jobID)
		redis.call('LPUSH', queueKey, jobID)
		
		return 1
	`)

	recovered := 0
	now := time.Now().Format(time.RFC3339)

	for _, jobID := range jobIDs {
		result, err := recoverScript.Run(ctx, sq.client, []string{
			sq.processingKey(),
			sq.queueKey,
			sq.jobPrefix + jobID,
			sq.workerKey(jobID),
		}, jobID, now).Int()
		if err != nil {
			continue
		}

		if result == 1 {
			recovered++
		}
	}

	return recovered, nil
}

// RecoveryLoop runs periodic recovery of stale jobs.
func (sq *ShardedQueue) RecoveryLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Also process retry queue more frequently
	retryTicker := time.NewTicker(10 * time.Second)
	defer retryTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if recovered, err := sq.RecoverStaleJobs(ctx); err != nil {
				log.Warn("Job recovery error", log.Err(err))
			} else if recovered > 0 {
				log.Info("Recovered stale jobs", log.Int("count", recovered))
			}
		case <-retryTicker.C:
			if processed, err := sq.ProcessRetryQueue(ctx); err != nil {
				log.Warn("Retry queue processing error", log.Err(err))
			} else if processed > 0 {
				log.Info("Processed retry jobs", log.Int("count", processed))
			}
		}
	}
}

// GetShardIndex returns this worker's shard index.
func (sq *ShardedQueue) GetShardIndex() int {
	return sq.shardIndex
}

// GetTotalShards returns total number of shards.
func (sq *ShardedQueue) GetTotalShards() int {
	return sq.totalShards
}

// IsShardingEnabled returns whether sharding is enabled.
func (sq *ShardedQueue) IsShardingEnabled() bool {
	return sq.enabled
}
