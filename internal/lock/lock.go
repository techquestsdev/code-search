package lock

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

// DistributedLock provides distributed locking using Redis.
type DistributedLock struct {
	client   *redis.Client
	key      string
	workerID string
	ttl      time.Duration
}

// NewDistributedLock creates a new distributed lock.
func NewDistributedLock(client *redis.Client, key string, ttl time.Duration) *DistributedLock {
	workerID := os.Getenv("HOSTNAME")
	if workerID == "" {
		workerID = fmt.Sprintf("worker-%d", time.Now().UnixNano())
	}

	return &DistributedLock{
		client:   client,
		key:      "codesearch:lock:" + key,
		workerID: workerID,
		ttl:      ttl,
	}
}

// TryAcquire attempts to acquire the lock without blocking
// Returns true if lock was acquired, false otherwise.
func (l *DistributedLock) TryAcquire(ctx context.Context) (bool, error) {
	ok, err := l.client.SetNX(ctx, l.key, l.workerID, l.ttl).Result()
	if err != nil {
		return false, fmt.Errorf("acquire lock: %w", err)
	}

	return ok, nil
}

// Acquire blocks until the lock is acquired or context is canceled.
func (l *DistributedLock) Acquire(ctx context.Context) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			acquired, err := l.TryAcquire(ctx)
			if err != nil {
				return err
			}

			if acquired {
				return nil
			}
		}
	}
}

// Release releases the lock if we own it
// Uses Lua script for atomic check-and-delete.
func (l *DistributedLock) Release(ctx context.Context) error {
	script := redis.NewScript(`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		end
		return 0
	`)

	_, err := script.Run(ctx, l.client, []string{l.key}, l.workerID).Result()
	if err != nil {
		return fmt.Errorf("release lock: %w", err)
	}

	return nil
}

// Extend extends the lock TTL if we still own it.
func (l *DistributedLock) Extend(ctx context.Context) error {
	script := redis.NewScript(`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("PEXPIRE", KEYS[1], ARGV[2])
		end
		return 0
	`)

	result, err := script.Run(ctx, l.client, []string{l.key}, l.workerID, l.ttl.Milliseconds()).
		Int()
	if err != nil {
		return fmt.Errorf("extend lock: %w", err)
	}

	if result == 0 {
		return errors.New("lock not owned by us")
	}

	return nil
}

// IsHeld checks if we currently hold the lock.
func (l *DistributedLock) IsHeld(ctx context.Context) (bool, error) {
	val, err := l.client.Get(ctx, l.key).Result()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("check lock: %w", err)
	}

	return val == l.workerID, nil
}

// WithLock executes a function while holding the lock
// Automatically acquires before and releases after.
func (l *DistributedLock) WithLock(ctx context.Context, fn func(ctx context.Context) error) error {
	err := l.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}

	defer func() { _ = l.Release(ctx) }()

	return fn(ctx)
}

// TryWithLock executes a function if the lock can be acquired
// Returns false if lock was not acquired.
func (l *DistributedLock) TryWithLock(
	ctx context.Context,
	fn func(ctx context.Context) error,
) (bool, error) {
	acquired, err := l.TryAcquire(ctx)
	if err != nil {
		return false, err
	}

	if !acquired {
		return false, nil
	}

	defer func() { _ = l.Release(ctx) }()

	return true, fn(ctx)
}
