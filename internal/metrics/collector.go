package metrics

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/redis/go-redis/v9"
)

// DBPool is an interface for database pools that provide stats and query capabilities.
// This matches the db.Pool interface from internal/db.
type DBPool interface {
	Stats() PoolStats
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
}

// PoolStats contains connection pool statistics.
// This matches db.PoolStats from internal/db.
type PoolStats struct {
	MaxOpenConnections int
	OpenConnections    int
	InUse              int
	Idle               int
}

// Rows is an interface for query results.
// This matches db.Rows from internal/db.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Close()
	Err() error
}

// Collector periodically collects infrastructure metrics.
type Collector struct {
	db        DBPool
	redis     *redis.Client
	indexPath string
	interval  time.Duration
	stopCh    chan struct{}
	stoppedCh chan struct{}
}

// NewCollector creates a new metrics collector.
// Pass nil for any component that's not available.
func NewCollector(
	db DBPool,
	redis *redis.Client,
	indexPath string,
	interval time.Duration,
) *Collector {
	if interval == 0 {
		interval = 30 * time.Second
	}

	return &Collector{
		db:        db,
		redis:     redis,
		indexPath: indexPath,
		interval:  interval,
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
	}
}

// DBPoolAdapter wraps a database pool to satisfy the DBPool interface.
type DBPoolAdapter struct {
	StatsFunc func() PoolStats
	QueryFunc func(ctx context.Context, sql string, args ...any) (Rows, error)
}

// Stats returns pool statistics.
func (a *DBPoolAdapter) Stats() PoolStats {
	if a.StatsFunc != nil {
		return a.StatsFunc()
	}

	return PoolStats{}
}

// Query executes a query.
func (a *DBPoolAdapter) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	return a.QueryFunc(ctx, sql, args...)
}

// Start begins collecting metrics at the configured interval.
func (c *Collector) Start(ctx context.Context) {
	go c.run(ctx)
}

// Stop stops the collector.
func (c *Collector) Stop() {
	close(c.stopCh)
	<-c.stoppedCh
}

func (c *Collector) run(ctx context.Context) {
	defer close(c.stoppedCh)

	// Collect immediately on start
	c.collect(ctx)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.collect(ctx)
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (c *Collector) collect(ctx context.Context) {
	// Collect database connection stats
	if c.db != nil {
		stats := c.db.Stats()
		SetDBConnections(stats.OpenConnections, stats.InUse)
	}

	// Collect Redis connection stats
	if c.redis != nil {
		poolStats := c.redis.PoolStats()
		SetRedisConnections(int(poolStats.TotalConns))
	}

	// Collect Zoekt index stats
	if c.indexPath != "" {
		shards, size := c.collectZoektStats()
		SetZoektStats(shards, size)
	}

	// Collect repository and job counts from database
	if c.db != nil {
		c.collectRepoCounts(ctx)
		c.collectConnectionCounts(ctx)
		c.collectJobQueueCounts(ctx)
	}
}

func (c *Collector) collectZoektStats() (shards int, totalSize int64) {
	entries, err := os.ReadDir(c.indexPath)
	if err != nil {
		return 0, 0
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Count .zoekt shard files
		if ext := filepath.Ext(entry.Name()); ext == ".zoekt" {
			shards++

			if info, err := entry.Info(); err == nil {
				totalSize += info.Size()
			}
		}
	}

	return shards, totalSize
}

func (c *Collector) collectRepoCounts(ctx context.Context) {
	counts := make(map[string]int)

	// Count repositories by index_status (only non-deleted, non-excluded)
	rows, err := c.db.Query(ctx, `
		SELECT index_status, COUNT(*) 
		FROM repositories 
		WHERE deleted = false AND excluded = false
		GROUP BY index_status
	`)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var (
			status string
			count  int
		)

		err := rows.Scan(&status, &count)
		if err == nil {
			counts[status] = count
		}
	}

	// Count excluded repositories (not deleted)
	excludedRows, err := c.db.Query(ctx, `
		SELECT COUNT(*) FROM repositories WHERE excluded = true AND deleted = false
	`)
	if err == nil {
		defer excludedRows.Close()
		if excludedRows.Next() {
			var count int
			if excludedRows.Scan(&count) == nil {
				counts["excluded"] = count
			}
		}
	}

	// Count deleted repositories
	deletedRows, err := c.db.Query(ctx, `
		SELECT COUNT(*) FROM repositories WHERE deleted = true
	`)
	if err == nil {
		defer deletedRows.Close()
		if deletedRows.Next() {
			var count int
			if deletedRows.Scan(&count) == nil {
				counts["deleted"] = count
			}
		}
	}

	if len(counts) > 0 {
		SetRepositoryCounts(counts)
	}
}

func (c *Collector) collectConnectionCounts(ctx context.Context) {
	counts := make(map[string]int)

	// Count connections by type
	rows, err := c.db.Query(ctx, `
		SELECT type, COUNT(*) 
		FROM connections 
		GROUP BY type
	`)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var (
			connType string
			count    int
		)

		err := rows.Scan(&connType, &count)
		if err == nil {
			counts[connType] = count
		}
	}

	if len(counts) > 0 {
		SetConnectionCounts(counts)
	}
}

func (c *Collector) collectJobQueueCounts(ctx context.Context) {
	counts := make(map[string]int)

	// Count pending jobs by type
	rows, err := c.db.Query(ctx, `
		SELECT type, COUNT(*) 
		FROM jobs 
		WHERE status = 'pending'
		GROUP BY type
	`)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var (
			jobType string
			count   int
		)

		err := rows.Scan(&jobType, &count)
		if err == nil {
			counts[jobType] = count
		}
	}

	if len(counts) > 0 {
		SetJobQueueCounts(counts)
	}
}
