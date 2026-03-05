package db

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Default pool configuration.
const (
	defaultMaxConns        = 25
	defaultMinConns        = 5
	defaultMaxConnLifetime = 5 * time.Minute
)

// getPoolConfig reads pool configuration from environment variables.
func getPoolConfig() (maxConns, minConns int32, lifetime time.Duration) {
	maxConns = defaultMaxConns
	minConns = defaultMinConns
	lifetime = defaultMaxConnLifetime

	// DB_MAX_CONNS - maximum number of connections in the pool
	if v := os.Getenv("DB_MAX_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxConns = int32(n)
		}
	}

	// DB_MIN_CONNS - minimum number of connections to keep open
	if v := os.Getenv("DB_MIN_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			minConns = int32(n)
		}
	}

	// DB_CONN_LIFETIME - max lifetime of a connection in seconds
	if v := os.Getenv("DB_CONN_LIFETIME_SECS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			lifetime = time.Duration(n) * time.Second
		}
	}

	// Ensure min <= max
	if minConns > maxConns {
		minConns = maxConns
	}

	return maxConns, minConns, lifetime
}

// DB wraps the database connection pool.
type DB struct {
	pool *pgxpool.Pool
}

// New creates a new database connection.
func New(ctx context.Context, databaseURL string) (*DB, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}

	maxConns, minConns, lifetime := getPoolConfig()
	config.MaxConns = maxConns
	config.MinConns = minConns
	config.MaxConnLifetime = lifetime

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}

	return &DB{pool: pool}, nil
}

// Connect creates a new database connection pool (convenience function).
// DEPRECATED: Use ConnectWithDriver for multi-database support.
func Connect(databaseURL string) (*pgxpool.Pool, error) {
	ctx := context.Background()

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}

	maxConns, minConns, lifetime := getPoolConfig()
	config.MaxConns = maxConns
	config.MinConns = minConns
	config.MaxConnLifetime = lifetime

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}

	return pool, nil
}

// ConnectWithDriver creates a new database connection pool with driver detection.
// The driver is auto-detected from the URL if not specified.
// Supported drivers: postgres, mysql.
func ConnectWithDriver(ctx context.Context, databaseURL, driver string) (Pool, error) {
	maxConns, minConns, lifetime := getPoolConfig()
	cfg := PoolConfig{
		MaxOpenConns:    int(maxConns),
		MaxIdleConns:    int(minConns),
		ConnMaxLifetime: lifetime,
	}

	// Auto-detect driver if not specified
	var driverType DriverType
	if driver != "" {
		driverType = DriverType(driver)
	} else {
		driverType = DetectDriver(databaseURL)
	}

	switch driverType {
	case DriverPostgres:
		return openPostgres(ctx, databaseURL, cfg)
	case DriverMySQL:
		return openMySQL(ctx, databaseURL, cfg)
	default:
		return nil, &UnsupportedDriverError{Driver: string(driverType)}
	}
}

// UnsupportedDriverError is returned when an unsupported database driver is specified.
type UnsupportedDriverError struct {
	Driver string
}

func (e *UnsupportedDriverError) Error() string {
	return "unsupported database driver: " + e.Driver
}

// Close closes the database connection pool.
func (db *DB) Close() {
	db.pool.Close()
}

// Pool returns the underlying connection pool.
func (db *DB) Pool() *pgxpool.Pool {
	return db.pool
}
