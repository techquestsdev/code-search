package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// MySQLPool wraps sql.DB to implement the Pool interface for MySQL.
type MySQLPool struct {
	db *sql.DB
}

// openMySQL creates a new MySQL connection pool.
func openMySQL(ctx context.Context, url string, cfg PoolConfig) (*MySQLPool, error) {
	// Convert mysql:// URL format to DSN format
	dsn := convertMySQLURL(url)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Test connection
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}

	return &MySQLPool{db: db}, nil
}

// convertMySQLURL converts mysql://user:pass@host:port/db to DSN format.
func convertMySQLURL(url string) string {
	// If already in DSN format, return as-is
	if !strings.HasPrefix(url, "mysql://") {
		return url
	}

	// Remove mysql:// prefix
	url = strings.TrimPrefix(url, "mysql://")

	// Parse user:pass@host:port/db?params
	var userPass, hostPort, dbName, params string

	// Split by @ to get user:pass and host:port/db
	atIdx := strings.Index(url, "@")
	if atIdx != -1 {
		userPass = url[:atIdx]
		url = url[atIdx+1:]
	}

	// Split by / to get host:port and db
	slashIdx := strings.Index(url, "/")
	if slashIdx != -1 {
		hostPort = url[:slashIdx]
		dbName = url[slashIdx+1:]
	} else {
		hostPort = url
	}

	// Split by ? to get db and params
	qIdx := strings.Index(dbName, "?")
	if qIdx != -1 {
		params = dbName[qIdx+1:]
		dbName = dbName[:qIdx]
	}

	// Build DSN: user:pass@tcp(host:port)/db?params
	dsn := ""
	if userPass != "" {
		dsn = userPass + "@"
	}

	dsn += "tcp(" + hostPort + ")/" + dbName

	// Add parseTime=true for proper time.Time handling
	if params != "" {
		if !strings.Contains(params, "parseTime") {
			params += "&parseTime=true"
		}

		dsn += "?" + params
	} else {
		dsn += "?parseTime=true"
	}

	return dsn
}

// Query executes a query that returns rows.
func (p *MySQLPool) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	// Convert PostgreSQL placeholders to MySQL placeholders
	sql = ConvertPlaceholders(sql, DriverMySQL)

	rows, err := p.db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}

	return &stdRows{rows: rows}, nil
}

// QueryRow executes a query that returns at most one row.
func (p *MySQLPool) QueryRow(ctx context.Context, sql string, args ...any) Row {
	// Convert PostgreSQL placeholders to MySQL placeholders
	sql = ConvertPlaceholders(sql, DriverMySQL)

	return &stdRow{row: p.db.QueryRowContext(ctx, sql, args...)}
}

// Exec executes a query that doesn't return rows.
func (p *MySQLPool) Exec(ctx context.Context, sql string, args ...any) (Result, error) {
	// Convert PostgreSQL placeholders to MySQL placeholders
	sql = ConvertPlaceholders(sql, DriverMySQL)

	result, err := p.db.ExecContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}

	return &stdResult{result: result}, nil
}

// Ping verifies the connection is alive.
func (p *MySQLPool) Ping(ctx context.Context) error {
	return p.db.PingContext(ctx)
}

// Close closes the connection pool.
func (p *MySQLPool) Close() error {
	return p.db.Close()
}

// Driver returns the driver type.
func (p *MySQLPool) Driver() DriverType {
	return DriverMySQL
}

// Stats returns connection pool statistics.
func (p *MySQLPool) Stats() PoolStats {
	stats := p.db.Stats()

	return PoolStats{
		MaxOpenConnections: stats.MaxOpenConnections,
		OpenConnections:    stats.OpenConnections,
		InUse:              stats.InUse,
		Idle:               stats.Idle,
	}
}

// Underlying returns the underlying sql.DB.
func (p *MySQLPool) Underlying() *sql.DB {
	return p.db
}

// Begin starts a new transaction.
func (p *MySQLPool) Begin(ctx context.Context) (Tx, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &mysqlTx{tx: tx}, nil
}

// mysqlTx wraps sql.Tx to implement Tx interface.
type mysqlTx struct {
	tx *sql.Tx
}

func (t *mysqlTx) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	// Convert PostgreSQL placeholders to MySQL placeholders
	sql = ConvertPlaceholders(sql, DriverMySQL)

	rows, err := t.tx.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}

	return &stdRows{rows: rows}, nil
}

func (t *mysqlTx) QueryRow(ctx context.Context, sql string, args ...any) Row {
	// Convert PostgreSQL placeholders to MySQL placeholders
	sql = ConvertPlaceholders(sql, DriverMySQL)

	return &stdRow{row: t.tx.QueryRowContext(ctx, sql, args...)}
}

func (t *mysqlTx) Exec(ctx context.Context, sql string, args ...any) (Result, error) {
	// Convert PostgreSQL placeholders to MySQL placeholders
	sql = ConvertPlaceholders(sql, DriverMySQL)

	result, err := t.tx.ExecContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}

	return &stdResult{result: result}, nil
}

func (t *mysqlTx) Commit(ctx context.Context) error {
	return t.tx.Commit()
}

func (t *mysqlTx) Rollback(ctx context.Context) error {
	return t.tx.Rollback()
}

// IsMySQLDuplicateError checks if an error is a MySQL duplicate key error.
func IsMySQLDuplicateError(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(err.Error(), "Duplicate entry") ||
		strings.Contains(err.Error(), "Error 1062")
}
