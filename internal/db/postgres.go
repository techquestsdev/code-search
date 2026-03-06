package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresPool wraps pgxpool.Pool to implement the Pool interface.
type PostgresPool struct {
	pool *pgxpool.Pool
}

// openPostgres creates a new PostgreSQL connection pool.
func openPostgres(ctx context.Context, url string, cfg PoolConfig) (*PostgresPool, error) {
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}

	config.MaxConns = int32(cfg.MaxOpenConns)
	config.MinConns = int32(cfg.MaxIdleConns)
	config.MaxConnLifetime = cfg.ConnMaxLifetime

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &PostgresPool{pool: pool}, nil
}

// Query executes a query that returns rows.
func (p *PostgresPool) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	rows, err := p.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}

	return &pgxRows{rows: rows}, nil
}

// QueryRow executes a query that returns at most one row.
func (p *PostgresPool) QueryRow(ctx context.Context, sql string, args ...any) Row {
	return &pgxRow{row: p.pool.QueryRow(ctx, sql, args...)}
}

// Exec executes a query that doesn't return rows.
func (p *PostgresPool) Exec(ctx context.Context, sql string, args ...any) (Result, error) {
	tag, err := p.pool.Exec(ctx, sql, args...)
	if err != nil {
		return nil, err
	}

	return &pgxResult{tag: tag}, nil
}

// Ping verifies the connection is alive.
func (p *PostgresPool) Ping(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

// Close closes the connection pool.
func (p *PostgresPool) Close() error {
	p.pool.Close()
	return nil
}

// Driver returns the driver type.
func (p *PostgresPool) Driver() DriverType {
	return DriverPostgres
}

// Stats returns connection pool statistics.
func (p *PostgresPool) Stats() PoolStats {
	stats := p.pool.Stat()

	return PoolStats{
		MaxOpenConnections: int(stats.MaxConns()),
		OpenConnections:    int(stats.TotalConns()),
		InUse:              int(stats.AcquiredConns()),
		Idle:               int(stats.IdleConns()),
	}
}

// Underlying returns the underlying pgxpool.Pool for legacy code.
func (p *PostgresPool) Underlying() *pgxpool.Pool {
	return p.pool
}

// Begin starts a new transaction.
func (p *PostgresPool) Begin(ctx context.Context) (Tx, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}

	return &pgxTx{tx: tx}, nil
}

// pgxTx wraps pgx.Tx to implement Tx interface.
type pgxTx struct {
	tx pgx.Tx
}

func (t *pgxTx) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	rows, err := t.tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}

	return &pgxRows{rows: rows}, nil
}

func (t *pgxTx) QueryRow(ctx context.Context, sql string, args ...any) Row {
	return &pgxRow{row: t.tx.QueryRow(ctx, sql, args...)}
}

func (t *pgxTx) Exec(ctx context.Context, sql string, args ...any) (Result, error) {
	tag, err := t.tx.Exec(ctx, sql, args...)
	if err != nil {
		return nil, err
	}

	return &pgxResult{tag: tag}, nil
}

func (t *pgxTx) Commit(ctx context.Context) error {
	return t.tx.Commit(ctx)
}

func (t *pgxTx) Rollback(ctx context.Context) error {
	return t.tx.Rollback(ctx)
}

// pgxRows wraps pgx.Rows to implement Rows interface.
type pgxRows struct {
	rows pgx.Rows
}

func (r *pgxRows) Next() bool {
	return r.rows.Next()
}

func (r *pgxRows) Scan(dest ...any) error {
	return r.rows.Scan(dest...)
}

func (r *pgxRows) Close() {
	r.rows.Close()
}

func (r *pgxRows) Err() error {
	return r.rows.Err()
}

// pgxRow wraps pgx.Row to implement Row interface.
type pgxRow struct {
	row pgx.Row
}

func (r *pgxRow) Scan(dest ...any) error {
	return r.row.Scan(dest...)
}

// pgxResult wraps pgx command tag to implement Result interface.
type pgxResult struct {
	tag pgconn.CommandTag
}

func (r *pgxResult) RowsAffected() (int64, error) {
	return r.tag.RowsAffected(), nil
}

func (r *pgxResult) LastInsertId() (int64, error) {
	// PostgreSQL doesn't support LastInsertId directly, use RETURNING instead
	return 0, errors.New("postgres does not support LastInsertId, use RETURNING clause")
}

// IsNoRows checks if an error is pgx.ErrNoRows.
func IsNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
