package db

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// DriverType represents the database driver type.
type DriverType string

const (
	DriverPostgres DriverType = "postgres"
	DriverMySQL    DriverType = "mysql"
)

// Pool is the common interface for database connection pools.
// It abstracts the differences between pgxpool.Pool and sql.DB.
type Pool interface {
	// Query executes a query that returns rows.
	Query(ctx context.Context, sql string, args ...any) (Rows, error)

	// QueryRow executes a query that returns at most one row.
	QueryRow(ctx context.Context, sql string, args ...any) Row

	// Exec executes a query that doesn't return rows.
	Exec(ctx context.Context, sql string, args ...any) (Result, error)

	// Begin starts a new transaction.
	Begin(ctx context.Context) (Tx, error)

	// Ping verifies the connection is alive.
	Ping(ctx context.Context) error

	// Close closes the connection pool.
	Close() error

	// Driver returns the driver type.
	Driver() DriverType

	// Stats returns connection pool statistics.
	Stats() PoolStats
}

// Tx is the common interface for database transactions.
type Tx interface {
	// Query executes a query that returns rows.
	Query(ctx context.Context, sql string, args ...any) (Rows, error)

	// QueryRow executes a query that returns at most one row.
	QueryRow(ctx context.Context, sql string, args ...any) Row

	// Exec executes a query that doesn't return rows.
	Exec(ctx context.Context, sql string, args ...any) (Result, error)

	// Commit commits the transaction.
	Commit(ctx context.Context) error

	// Rollback aborts the transaction.
	Rollback(ctx context.Context) error
}

// Rows is the common interface for query results.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Close()
	Err() error
}

// Row is the common interface for single row results.
type Row interface {
	Scan(dest ...any) error
}

// Result is the common interface for exec results.
type Result interface {
	RowsAffected() (int64, error)
	LastInsertId() (int64, error)
}

// PoolStats contains connection pool statistics.
type PoolStats struct {
	MaxOpenConnections int
	OpenConnections    int
	InUse              int
	Idle               int
}

// PoolConfig contains common pool configuration.
type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// DefaultPoolConfig returns default pool configuration.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
	}
}

// DetectDriver detects the database driver from the URL.
func DetectDriver(url string) DriverType {
	if strings.HasPrefix(url, "postgres://") || strings.HasPrefix(url, "postgresql://") {
		return DriverPostgres
	}

	if strings.HasPrefix(url, "mysql://") || strings.Contains(url, "@tcp(") {
		return DriverMySQL
	}

	// Default to postgres
	return DriverPostgres
}

// Open creates a new database connection pool based on the URL.
func Open(ctx context.Context, url string, cfg PoolConfig) (Pool, error) {
	driver := DetectDriver(url)

	switch driver {
	case DriverPostgres:
		return openPostgres(ctx, url, cfg)
	case DriverMySQL:
		return openMySQL(ctx, url, cfg)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", driver)
	}
}

// PlaceholderStyle returns the placeholder style for a driver.
// PostgreSQL uses $1, $2, etc. MySQL uses ?
func PlaceholderStyle(driver DriverType) string {
	switch driver {
	case DriverMySQL:
		return "?"
	default:
		return "$"
	}
}

// placeholderRegex matches PostgreSQL-style placeholders ($1, $2, etc.)
var placeholderRegex = regexp.MustCompile(`\$\d+`)

// ConvertPlaceholders converts PostgreSQL-style placeholders ($1, $2) to MySQL-style (?)
// when using MySQL driver.
func ConvertPlaceholders(query string, driver DriverType) string {
	if driver == DriverPostgres {
		return query
	}

	// For MySQL, convert all $N placeholders to ?
	return placeholderRegex.ReplaceAllString(query, "?")
}

// stdRow wraps sql.Row to implement Row interface.
type stdRow struct {
	row *sql.Row
}

func (r *stdRow) Scan(dest ...any) error {
	return r.row.Scan(dest...)
}

// stdRows wraps sql.Rows to implement Rows interface.
type stdRows struct {
	rows *sql.Rows
}

func (r *stdRows) Next() bool {
	return r.rows.Next()
}

func (r *stdRows) Scan(dest ...any) error {
	return r.rows.Scan(dest...)
}

func (r *stdRows) Close() {
	r.rows.Close()
}

func (r *stdRows) Err() error {
	return r.rows.Err()
}

// stdResult wraps sql.Result to implement Result interface.
type stdResult struct {
	result sql.Result
}

func (r *stdResult) RowsAffected() (int64, error) {
	return r.result.RowsAffected()
}

func (r *stdResult) LastInsertId() (int64, error) {
	return r.result.LastInsertId()
}
