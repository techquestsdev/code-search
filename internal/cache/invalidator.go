// Package cache provides cross-instance cache invalidation via Redis pub/sub.
package cache

import (
	"context"
	"strconv"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const reindexedChannel = "codesearch:repo:reindexed"

// SCIPEvictor evicts cached SCIP data for a repo.
type SCIPEvictor interface {
	EvictCache(repoID int64)
}

// SymbolsInvalidator invalidates cached symbols for a repo.
type SymbolsInvalidator interface {
	InvalidateRepo(ctx context.Context, repoID int64) error
}

// Invalidator subscribes to Redis pub/sub and invalidates local caches.
type Invalidator struct {
	redisClient *redis.Client
	scip        SCIPEvictor
	symbols     SymbolsInvalidator
	logger      *zap.Logger
}

// NewInvalidator creates a new cache invalidator.
func NewInvalidator(
	redisClient *redis.Client,
	scip SCIPEvictor,
	symbols SymbolsInvalidator,
	logger *zap.Logger,
) *Invalidator {
	return &Invalidator{
		redisClient: redisClient,
		scip:        scip,
		symbols:     symbols,
		logger:      logger.With(zap.String("component", "cache-invalidator")),
	}
}

// Start begins listening for reindex events and invalidating caches.
// Runs until ctx is canceled. Should be called in a goroutine.
func (inv *Invalidator) Start(ctx context.Context) {
	sub := inv.redisClient.Subscribe(ctx, reindexedChannel)

	defer func() { _ = sub.Close() }()

	inv.logger.Info("Cache invalidation subscriber started")

	ch := sub.Channel()

	for {
		select {
		case <-ctx.Done():
			inv.logger.Info("Cache invalidation subscriber stopped")
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}

			repoID, err := strconv.ParseInt(msg.Payload, 10, 64)
			if err != nil {
				inv.logger.Debug(
					"Invalid reindex event payload",
					zap.String("payload", msg.Payload),
				)

				continue
			}

			inv.invalidate(ctx, repoID)
		}
	}
}

func (inv *Invalidator) invalidate(ctx context.Context, repoID int64) {
	if inv.scip != nil {
		inv.scip.EvictCache(repoID)
	}

	if inv.symbols != nil {
		if err := inv.symbols.InvalidateRepo(ctx, repoID); err != nil {
			inv.logger.Debug("Failed to invalidate symbols cache",
				zap.Int64("repo_id", repoID),
				zap.Error(err),
			)
		}
	}

	inv.logger.Debug("Cache invalidated for repo", zap.Int64("repo_id", repoID))
}
