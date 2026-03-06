package db

import (
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5"
)

// ErrNoRows is returned when a query returns no results.
// It wraps different driver-specific errors into a common error.
var ErrNoRows = errors.New("no rows in result set")

// IsNoRowsError checks if an error is a "no rows" error from any supported driver.
func IsNoRowsError(err error) bool {
	if err == nil {
		return false
	}

	// Check for pgx error
	if errors.Is(err, pgx.ErrNoRows) {
		return true
	}

	// Check for standard sql error
	if errors.Is(err, sql.ErrNoRows) {
		return true
	}

	// Check if it's our wrapped error
	if errors.Is(err, ErrNoRows) {
		return true
	}

	return false
}

// NormalizeError converts driver-specific errors to common errors.
func NormalizeError(err error) error {
	if err == nil {
		return nil
	}

	if IsNoRowsError(err) {
		return ErrNoRows
	}

	return err
}

// MustGetPostgresPool panics if the pool is not a PostgresPool.
// This is useful for legacy code that requires pgxpool.Pool directly.
func MustGetPostgresPool(p Pool) *PostgresPool {
	if pg, ok := p.(*PostgresPool); ok {
		return pg
	}

	panic("expected PostgresPool but got different driver")
}

// TryGetPostgresPool returns the underlying PostgresPool if available.
func TryGetPostgresPool(p Pool) (*PostgresPool, bool) {
	pg, ok := p.(*PostgresPool)
	return pg, ok
}

// TryGetMySQLPool returns the underlying MySQLPool if available.
func TryGetMySQLPool(p Pool) (*MySQLPool, bool) {
	my, ok := p.(*MySQLPool)
	return my, ok
}
