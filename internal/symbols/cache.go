// Package symbols provides symbol extraction and caching.
package symbols

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// CacheEntry represents a cached symbol with metadata.
type CacheEntry struct {
	Symbol

	RepoID    int64
	CommitSHA string
	CreatedAt time.Time
}

// SymbolCache provides SQLite-based caching for extracted symbols.
type SymbolCache struct {
	cacheDir string
	dbs      map[int64]*sql.DB
	mu       sync.RWMutex
}

// NewSymbolCache creates a new symbol cache with the specified cache directory.
func NewSymbolCache(cacheDir string) (*SymbolCache, error) {
	err := os.MkdirAll(cacheDir, 0o755)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &SymbolCache{
		cacheDir: cacheDir,
		dbs:      make(map[int64]*sql.DB),
	}, nil
}

// getDB returns or creates a SQLite database for a repository.
func (c *SymbolCache) getDB(repoID int64) (*sql.DB, error) {
	c.mu.RLock()

	if db, ok := c.dbs[repoID]; ok {
		c.mu.RUnlock()
		return db, nil
	}

	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if db, ok := c.dbs[repoID]; ok {
		return db, nil
	}

	dbPath := filepath.Join(c.cacheDir, fmt.Sprintf("repo_%d.db", repoID))

	db, err := sql.Open("sqlite3", dbPath+"?_journal=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Initialize schema
	if err := c.initSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	c.dbs[repoID] = db

	return db, nil
}

// initSchema creates the necessary tables if they don't exist.
func (c *SymbolCache) initSchema(db *sql.DB) error {
	schema := `
		CREATE TABLE IF NOT EXISTS symbols (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			file_path TEXT NOT NULL,
			name TEXT NOT NULL,
			kind TEXT NOT NULL,
			line INTEGER NOT NULL,
			column INTEGER NOT NULL,
			end_line INTEGER,
			end_column INTEGER,
			signature TEXT,
			parent TEXT,
			commit_sha TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		
		CREATE INDEX IF NOT EXISTS idx_symbols_file ON symbols(file_path);
		CREATE INDEX IF NOT EXISTS idx_symbols_name ON symbols(name);
		CREATE INDEX IF NOT EXISTS idx_symbols_kind ON symbols(kind);
		CREATE INDEX IF NOT EXISTS idx_symbols_commit ON symbols(commit_sha);
		
		CREATE TABLE IF NOT EXISTS cache_meta (
			file_path TEXT PRIMARY KEY,
			commit_sha TEXT NOT NULL,
			indexed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`

	_, err := db.ExecContext(context.Background(), schema)

	return err
}

// GetSymbols retrieves cached symbols for a file.
func (c *SymbolCache) GetSymbols(
	ctx context.Context,
	repoID int64,
	filePath, commitSHA string,
) ([]Symbol, bool, error) {
	db, err := c.getDB(repoID)
	if err != nil {
		return nil, false, err
	}

	// Check if cache is valid for this commit
	var cachedSHA string

	err = db.QueryRowContext(ctx,
		"SELECT commit_sha FROM cache_meta WHERE file_path = ?",
		filePath,
	).Scan(&cachedSHA)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil // No cache
	}

	if err != nil {
		return nil, false, err
	}

	if cachedSHA != commitSHA {
		return nil, false, nil // Cache is stale
	}

	// Fetch cached symbols
	rows, err := db.QueryContext(ctx, `
		SELECT name, kind, line, column, end_line, end_column, signature, parent
		FROM symbols
		WHERE file_path = ? AND commit_sha = ?
		ORDER BY line
	`, filePath, commitSHA)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	var symbols []Symbol

	for rows.Next() {
		var (
			s                  Symbol
			endLine, endColumn sql.NullInt64
			signature, parent  sql.NullString
		)

		err := rows.Scan(
			&s.Name,
			&s.Kind,
			&s.Line,
			&s.Column,
			&endLine,
			&endColumn,
			&signature,
			&parent,
		)
		if err != nil {
			return nil, false, err
		}

		if endLine.Valid {
			s.EndLine = int(endLine.Int64)
		}

		if endColumn.Valid {
			s.EndColumn = int(endColumn.Int64)
		}

		if signature.Valid {
			s.Signature = signature.String
		}

		if parent.Valid {
			s.Parent = parent.String
		}

		s.FilePath = filePath

		symbols = append(symbols, s)
	}

	return symbols, true, rows.Err()
}

// SetSymbols stores symbols in the cache.
func (c *SymbolCache) SetSymbols(
	ctx context.Context,
	repoID int64,
	filePath, commitSHA string,
	symbols []Symbol,
) error {
	db, err := c.getDB(repoID)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete old symbols for this file
	_, err = tx.ExecContext(ctx, "DELETE FROM symbols WHERE file_path = ?", filePath)
	if err != nil {
		return err
	}

	// Insert new symbols
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO symbols (file_path, name, kind, line, column, end_line, end_column, signature, parent, commit_sha)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, s := range symbols {
		var endLine, endColumn any
		if s.EndLine > 0 {
			endLine = s.EndLine
		}

		if s.EndColumn > 0 {
			endColumn = s.EndColumn
		}

		_, err = stmt.ExecContext(
			ctx,
			filePath,
			s.Name,
			s.Kind,
			s.Line,
			s.Column,
			endLine,
			endColumn,
			s.Signature,
			s.Parent,
			commitSHA,
		)
		if err != nil {
			return err
		}
	}

	// Update cache metadata
	_, err = tx.ExecContext(ctx, `
		INSERT OR REPLACE INTO cache_meta (file_path, commit_sha, indexed_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
	`, filePath, commitSHA)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// InvalidateFile removes cached symbols for a file.
func (c *SymbolCache) InvalidateFile(ctx context.Context, repoID int64, filePath string) error {
	db, err := c.getDB(repoID)
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, "DELETE FROM symbols WHERE file_path = ?", filePath)
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, "DELETE FROM cache_meta WHERE file_path = ?", filePath)

	return err
}

// InvalidateRepo clears all cached symbols for a repository.
func (c *SymbolCache) InvalidateRepo(ctx context.Context, repoID int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if db, ok := c.dbs[repoID]; ok {
		db.Close()
		delete(c.dbs, repoID)
	}

	dbPath := filepath.Join(c.cacheDir, fmt.Sprintf("repo_%d.db", repoID))

	return os.Remove(dbPath)
}

// SearchSymbols searches for symbols by name across all files in a repo.
func (c *SymbolCache) SearchSymbols(
	ctx context.Context,
	repoID int64,
	query string,
	kind string,
	limit int,
) ([]Symbol, error) {
	db, err := c.getDB(repoID)
	if err != nil {
		return nil, err
	}

	var args []any

	sqlQuery := `
		SELECT file_path, name, kind, line, column, end_line, end_column, signature, parent
		FROM symbols
		WHERE name LIKE ?
	`

	args = append(args, "%"+query+"%")

	if kind != "" {
		sqlQuery += " AND kind = ?"

		args = append(args, kind)
	}

	sqlQuery += " ORDER BY name LIMIT ?"

	args = append(args, limit)

	rows, err := db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var symbols []Symbol

	for rows.Next() {
		var (
			s                  Symbol
			endLine, endColumn sql.NullInt64
			signature, parent  sql.NullString
		)

		err := rows.Scan(
			&s.FilePath,
			&s.Name,
			&s.Kind,
			&s.Line,
			&s.Column,
			&endLine,
			&endColumn,
			&signature,
			&parent,
		)
		if err != nil {
			return nil, err
		}

		if endLine.Valid {
			s.EndLine = int(endLine.Int64)
		}

		if endColumn.Valid {
			s.EndColumn = int(endColumn.Int64)
		}

		if signature.Valid {
			s.Signature = signature.String
		}

		if parent.Valid {
			s.Parent = parent.String
		}

		symbols = append(symbols, s)
	}

	return symbols, rows.Err()
}

// Close closes all database connections.
func (c *SymbolCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var errs []error

	for _, db := range c.dbs {
		err := db.Close()
		if err != nil {
			errs = append(errs, err)
		}
	}

	c.dbs = make(map[int64]*sql.DB)

	if len(errs) > 0 {
		return fmt.Errorf("errors closing databases: %v", errs)
	}

	return nil
}

// Stats returns cache statistics for a repository.
func (c *SymbolCache) Stats(ctx context.Context, repoID int64) (map[string]any, error) {
	db, err := c.getDB(repoID)
	if err != nil {
		return nil, err
	}

	stats := make(map[string]any)

	var totalSymbols int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM symbols").Scan(&totalSymbols); err != nil {
		return nil, err
	}

	stats["total_symbols"] = totalSymbols

	var totalFiles int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(DISTINCT file_path) FROM symbols").Scan(&totalFiles); err != nil {
		return nil, err
	}

	stats["total_files"] = totalFiles

	// Get symbol counts by kind
	rows, err := db.QueryContext(ctx, "SELECT kind, COUNT(*) FROM symbols GROUP BY kind")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byKind := make(map[string]int)

	for rows.Next() {
		var (
			kind  string
			count int
		)

		err := rows.Scan(&kind, &count)
		if err != nil {
			return nil, err
		}

		byKind[kind] = count
	}

	stats["by_kind"] = byKind

	return stats, nil
}
