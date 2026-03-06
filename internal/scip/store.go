// Package scip provides SCIP index storage and querying for precise code navigation.
package scip

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

// Occurrence represents a symbol occurrence in the code.
type Occurrence struct {
	Symbol     string `json:"symbol"`
	FilePath   string `json:"filePath"`
	StartLine  int    `json:"startLine"`         // 0-indexed
	StartCol   int    `json:"startCol"`          // 0-indexed
	EndLine    int    `json:"endLine"`           // 0-indexed
	EndCol     int    `json:"endCol"`            // 0-indexed
	Role       int    `json:"role"`              // Bitmask: 1=Definition, 2=Import, 4=WriteAccess, 8=ReadAccess
	SyntaxKind int    `json:"syntaxKind"`        // SCIP syntax kind
	Context    string `json:"context,omitempty"` // The source line content (populated by handlers)
}

// IsDefinition returns true if this occurrence is a definition.
func (o *Occurrence) IsDefinition() bool {
	return o.Role&1 != 0
}

// IsReference returns true if this occurrence is a read reference.
func (o *Occurrence) IsReference() bool {
	return o.Role&8 != 0 || o.Role == 0 // Read access or unspecified (default reference)
}

// IsImport returns true if this occurrence is an import.
func (o *Occurrence) IsImport() bool {
	return o.Role&2 != 0
}

// IsWrite returns true if this occurrence is a write.
func (o *Occurrence) IsWrite() bool {
	return o.Role&4 != 0
}

// extractSymbolPath extracts the version-independent part of a SCIP symbol.
// SCIP symbols have format: "scheme manager package version path"
// e.g.: "scip-go gomod gitlab.com/myorg/group/project 19ead6a9808a `...`/TFE#Example()."
// We extract everything after the 4th space (the actual symbol path).
func extractSymbolPath(symbol string) string {
	spaceCount := 0

	for i, c := range symbol {
		if c == ' ' {
			spaceCount++
			if spaceCount == 4 {
				return symbol[i+1:]
			}
		}
	}
	// Fallback: return the whole symbol if we can't parse it
	return symbol
}

// SymbolInfo represents metadata about a symbol.
type SymbolInfo struct {
	Symbol          string   `json:"symbol"`
	Documentation   string   `json:"documentation,omitempty"`
	Kind            int      `json:"kind,omitempty"`
	DisplayName     string   `json:"displayName,omitempty"`
	EnclosingSymbol string   `json:"enclosingSymbol,omitempty"`
	Relationships   []string `json:"relationships,omitempty"`
}

// DefinitionResult contains the result of a go-to-definition query.
type DefinitionResult struct {
	Symbol     string      `json:"symbol"`
	Definition *Occurrence `json:"definition,omitempty"`
	Info       *SymbolInfo `json:"info,omitempty"`
	External   bool        `json:"external"` // True if definition is in external package
}

// ReferencesResult contains the result of a find-references query.
type ReferencesResult struct {
	Symbol     string       `json:"symbol"`
	Definition *Occurrence  `json:"definition,omitempty"`
	References []Occurrence `json:"references"`
	TotalCount int          `json:"totalCount"`
}

// Store provides SQLite-based storage for SCIP index data.
type Store struct {
	cacheDir string
	dbs      map[int64]*sql.DB
	mu       sync.RWMutex
	logger   *zap.Logger
}

// NewStore creates a new SCIP store with the specified cache directory.
func NewStore(cacheDir string, logger *zap.Logger) (*Store, error) {
	err := os.MkdirAll(cacheDir, 0o755)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &Store{
		cacheDir: cacheDir,
		dbs:      make(map[int64]*sql.DB),
		logger:   logger,
	}, nil
}

// getDB returns or creates a SQLite database for a repository.
func (s *Store) getDB(repoID int64) (*sql.DB, error) {
	s.mu.RLock()

	if db, ok := s.dbs[repoID]; ok {
		s.mu.RUnlock()
		return db, nil
	}

	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if db, ok := s.dbs[repoID]; ok {
		return db, nil
	}

	dbPath := filepath.Join(s.cacheDir, fmt.Sprintf("scip_%d.db", repoID))

	db, err := sql.Open("sqlite3", dbPath+"?_journal=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Initialize schema
	if err := s.initSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	s.dbs[repoID] = db

	return db, nil
}

// initSchema creates the necessary tables if they don't exist.
func (s *Store) initSchema(db *sql.DB) error {
	schema := `
		-- Occurrences: every symbol usage location
		CREATE TABLE IF NOT EXISTS occurrences (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			symbol TEXT NOT NULL,
			file_path TEXT NOT NULL,
			start_line INTEGER NOT NULL,
			start_col INTEGER NOT NULL,
			end_line INTEGER NOT NULL,
			end_col INTEGER NOT NULL,
			role INTEGER NOT NULL DEFAULT 0,
			syntax_kind INTEGER NOT NULL DEFAULT 0
		);
		
		-- Index for finding occurrences at a position (go to definition)
		CREATE INDEX IF NOT EXISTS idx_occ_position ON occurrences(file_path, start_line, start_col);
		
		-- Index for finding all occurrences of a symbol (find references)
		CREATE INDEX IF NOT EXISTS idx_occ_symbol ON occurrences(symbol);
		
		-- Index for finding definitions
		CREATE INDEX IF NOT EXISTS idx_occ_definition ON occurrences(symbol, role) WHERE role & 1 = 1;
		
		-- Symbol information: metadata about symbols
		CREATE TABLE IF NOT EXISTS symbol_info (
			symbol TEXT PRIMARY KEY,
			documentation TEXT,
			kind INTEGER,
			display_name TEXT,
			enclosing_symbol TEXT
		);
		
		-- Symbol relationships (e.g., implements, extends)
		CREATE TABLE IF NOT EXISTS symbol_relationships (
			symbol TEXT NOT NULL,
			related_symbol TEXT NOT NULL,
			relationship_type TEXT NOT NULL,
			PRIMARY KEY (symbol, related_symbol, relationship_type)
		);
		
		CREATE INDEX IF NOT EXISTS idx_rel_symbol ON symbol_relationships(symbol);
		CREATE INDEX IF NOT EXISTS idx_rel_related ON symbol_relationships(related_symbol);
		
		-- Index metadata
		CREATE TABLE IF NOT EXISTS index_meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);
	`

	_, err := db.ExecContext(context.Background(), schema)

	return err
}

// HasIndex checks if a SCIP index exists for a repository.
func (s *Store) HasIndex(ctx context.Context, repoID int64) (bool, error) {
	db, err := s.getDB(repoID)
	if err != nil {
		return false, err
	}

	var count int

	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM occurrences LIMIT 1").Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// GetIndexMeta returns metadata about the SCIP index.
func (s *Store) GetIndexMeta(ctx context.Context, repoID int64) (map[string]string, error) {
	db, err := s.getDB(repoID)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, "SELECT key, value FROM index_meta")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	meta := make(map[string]string)

	for rows.Next() {
		var key, value string

		err := rows.Scan(&key, &value)
		if err != nil {
			return nil, err
		}

		meta[key] = value
	}

	return meta, rows.Err()
}

// GetOccurrenceAtPosition finds the symbol occurrence at a specific position.
func (s *Store) GetOccurrenceAtPosition(
	ctx context.Context,
	repoID int64,
	filePath string,
	line, col int,
) (*Occurrence, error) {
	db, err := s.getDB(repoID)
	if err != nil {
		return nil, err
	}

	// Find the most specific occurrence that contains the given position
	// We prioritize:
	// 1. Occurrences on the same line (single-line spans)
	// 2. Smallest span (most specific symbol)
	// 3. Occurrences where the column is within the symbol's column range
	row := db.QueryRowContext(ctx, `
		SELECT symbol, file_path, start_line, start_col, end_line, end_col, role, syntax_kind
		FROM occurrences
		WHERE file_path = ? 
		  AND start_line = ? 
		  AND start_col <= ? 
		  AND end_col >= ?
		ORDER BY (end_col - start_col) ASC
		LIMIT 1
	`, filePath, line, col, col)

	var occ Occurrence

	err = row.Scan(
		&occ.Symbol,
		&occ.FilePath,
		&occ.StartLine,
		&occ.StartCol,
		&occ.EndLine,
		&occ.EndCol,
		&occ.Role,
		&occ.SyntaxKind,
	)
	if errors.Is(err, sql.ErrNoRows) {
		// Try a broader search for multi-line spans
		row = db.QueryRowContext(ctx, `
			SELECT symbol, file_path, start_line, start_col, end_line, end_col, role, syntax_kind
			FROM occurrences
			WHERE file_path = ? 
			  AND start_line <= ? AND end_line >= ?
			  AND (start_line < ? OR (start_line = ? AND start_col <= ?))
			  AND (end_line > ? OR (end_line = ? AND end_col >= ?))
			ORDER BY (end_line - start_line) ASC, (end_col - start_col) ASC
			LIMIT 1
		`, filePath, line, line, line, line, col, line, line, col)

		err = row.Scan(
			&occ.Symbol,
			&occ.FilePath,
			&occ.StartLine,
			&occ.StartCol,
			&occ.EndLine,
			&occ.EndCol,
			&occ.Role,
			&occ.SyntaxKind,
		)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
	}

	if err != nil {
		return nil, err
	}

	return &occ, nil
}

// GetDefinition finds the definition of a symbol.
// It uses the version-independent symbol path for matching.
func (s *Store) GetDefinition(
	ctx context.Context,
	repoID int64,
	symbol string,
) (*Occurrence, error) {
	db, err := s.getDB(repoID)
	if err != nil {
		return nil, err
	}

	// Extract the symbol path for version-independent matching
	symbolPath := extractSymbolPath(symbol)

	row := db.QueryRowContext(ctx, `
		SELECT symbol, file_path, start_line, start_col, end_line, end_col, role, syntax_kind
		FROM occurrences
		WHERE (symbol = ? OR symbol LIKE ?) AND (role & 1) = 1
		LIMIT 1
	`, symbol, "%"+symbolPath)

	var occ Occurrence

	err = row.Scan(
		&occ.Symbol,
		&occ.FilePath,
		&occ.StartLine,
		&occ.StartCol,
		&occ.EndLine,
		&occ.EndCol,
		&occ.Role,
		&occ.SyntaxKind,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return &occ, nil
}

// GetSymbolInfo returns information about a symbol.
func (s *Store) GetSymbolInfo(
	ctx context.Context,
	repoID int64,
	symbol string,
) (*SymbolInfo, error) {
	db, err := s.getDB(repoID)
	if err != nil {
		return nil, err
	}

	row := db.QueryRowContext(ctx, `
		SELECT symbol, documentation, kind, display_name, enclosing_symbol
		FROM symbol_info
		WHERE symbol = ?
	`, symbol)

	var (
		info                        SymbolInfo
		doc, displayName, enclosing sql.NullString
		kind                        sql.NullInt64
	)

	err = row.Scan(&info.Symbol, &doc, &kind, &displayName, &enclosing)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	if doc.Valid {
		info.Documentation = doc.String
	}

	if kind.Valid {
		info.Kind = int(kind.Int64)
	}

	if displayName.Valid {
		info.DisplayName = displayName.String
	}

	if enclosing.Valid {
		info.EnclosingSymbol = enclosing.String
	}

	return &info, nil
}

// GetReferences finds all references to a symbol.
// It uses the version-independent symbol path for matching to handle cases
// where the same symbol was indexed at different commits.
func (s *Store) GetReferences(
	ctx context.Context,
	repoID int64,
	symbol string,
	limit int,
) ([]Occurrence, error) {
	db, err := s.getDB(repoID)
	if err != nil {
		return nil, err
	}

	// Extract the symbol path (version-independent part) for matching
	symbolPath := extractSymbolPath(symbol)

	// First try exact match, then fall back to suffix match
	query := `
		SELECT symbol, file_path, start_line, start_col, end_line, end_col, role, syntax_kind
		FROM occurrences
		WHERE symbol = ? OR symbol LIKE ?
		ORDER BY file_path, start_line, start_col
	`
	// Use % wildcard to match any version prefix
	args := []any{symbol, "%" + symbolPath}

	if limit > 0 {
		query += " LIMIT ?"

		args = append(args, limit)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []Occurrence

	for rows.Next() {
		var occ Occurrence

		err := rows.Scan(
			&occ.Symbol,
			&occ.FilePath,
			&occ.StartLine,
			&occ.StartCol,
			&occ.EndLine,
			&occ.EndCol,
			&occ.Role,
			&occ.SyntaxKind,
		)
		if err != nil {
			return nil, err
		}

		refs = append(refs, occ)
	}

	return refs, rows.Err()
}

// GetReferenceCount returns the count of references to a symbol.
func (s *Store) GetReferenceCount(ctx context.Context, repoID int64, symbol string) (int, error) {
	db, err := s.getDB(repoID)
	if err != nil {
		return 0, err
	}

	// Use the same matching logic as GetReferences
	symbolPath := extractSymbolPath(symbol)

	var count int

	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM occurrences WHERE symbol = ? OR symbol LIKE ?", symbol, "%"+symbolPath).
		Scan(&count)

	return count, err
}

// GoToDefinition performs a complete go-to-definition lookup.
func (s *Store) GoToDefinition(
	ctx context.Context,
	repoID int64,
	filePath string,
	line, col int,
) (*DefinitionResult, error) {
	// Find occurrence at position
	occ, err := s.GetOccurrenceAtPosition(ctx, repoID, filePath, line, col)
	if err != nil {
		return nil, err
	}

	if occ == nil {
		return nil, nil
	}

	result := &DefinitionResult{
		Symbol: occ.Symbol,
	}

	// If this occurrence is already a definition, return it
	if occ.IsDefinition() {
		result.Definition = occ
	} else {
		// Find the definition
		def, err := s.GetDefinition(ctx, repoID, occ.Symbol)
		if err != nil {
			return nil, err
		}

		result.Definition = def
		if def == nil {
			result.External = true
		}
	}

	// Get symbol info
	info, err := s.GetSymbolInfo(ctx, repoID, occ.Symbol)
	if err != nil {
		return nil, err
	}

	result.Info = info

	return result, nil
}

// FindReferences performs a complete find-references lookup.
func (s *Store) FindReferences(
	ctx context.Context,
	repoID int64,
	filePath string,
	line, col int,
	limit int,
) (*ReferencesResult, error) {
	// Find occurrence at position
	occ, err := s.GetOccurrenceAtPosition(ctx, repoID, filePath, line, col)
	if err != nil {
		return nil, err
	}

	if occ == nil {
		return nil, nil
	}

	// Debug: log the symbol found
	s.logger.Debug("FindReferences: found symbol",
		zap.String("symbol", occ.Symbol),
		zap.String("file", filePath),
		zap.Int("line", line),
		zap.Int("col", col),
	)

	result := &ReferencesResult{
		Symbol: occ.Symbol,
	}

	// Get definition
	def, err := s.GetDefinition(ctx, repoID, occ.Symbol)
	if err != nil {
		return nil, err
	}

	result.Definition = def

	// Get all references
	refs, err := s.GetReferences(ctx, repoID, occ.Symbol, limit)
	if err != nil {
		return nil, err
	}

	result.References = refs

	// Debug: log reference count
	s.logger.Debug("FindReferences: found references",
		zap.Int("count", len(refs)),
		zap.String("symbol", occ.Symbol),
	)

	// Get total count
	count, err := s.GetReferenceCount(ctx, repoID, occ.Symbol)
	if err != nil {
		return nil, err
	}

	result.TotalCount = count

	return result, nil
}

// ClearIndex removes all SCIP data for a repository.
func (s *Store) ClearIndex(ctx context.Context, repoID int64) error {
	// Close any existing connection first to ensure we can cleanly reinitialize
	s.mu.Lock()

	if db, ok := s.dbs[repoID]; ok {
		db.Close()
		delete(s.dbs, repoID)
	}

	s.mu.Unlock()

	db, err := s.getDB(repoID)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tables := []string{"occurrences", "symbol_info", "symbol_relationships", "index_meta"}
	for _, table := range tables {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Stats returns statistics about the SCIP index.
func (s *Store) Stats(ctx context.Context, repoID int64) (map[string]any, error) {
	db, err := s.getDB(repoID)
	if err != nil {
		return nil, err
	}

	stats := make(map[string]any)

	var totalOccurrences int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM occurrences").Scan(&totalOccurrences); err != nil {
		return nil, err
	}

	stats["total_occurrences"] = totalOccurrences

	var totalSymbols int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(DISTINCT symbol) FROM occurrences").Scan(&totalSymbols); err != nil {
		return nil, err
	}

	stats["total_symbols"] = totalSymbols

	var totalFiles int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(DISTINCT file_path) FROM occurrences").Scan(&totalFiles); err != nil {
		return nil, err
	}

	stats["total_files"] = totalFiles

	var definitions int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM occurrences WHERE (role & 1) = 1").Scan(&definitions); err != nil {
		return nil, err
	}

	stats["definitions"] = definitions

	var references int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM occurrences WHERE (role & 1) = 0").Scan(&references); err != nil {
		return nil, err
	}

	stats["references"] = references

	// Get index metadata
	meta, err := s.GetIndexMeta(ctx, repoID)
	if err != nil {
		return nil, err
	}

	stats["metadata"] = meta

	return stats, nil
}

// EvictCache closes and removes the in-memory cached connection for a repo.
// The next access will re-open the database from disk, picking up any changes.
func (s *Store) EvictCache(repoID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if db, ok := s.dbs[repoID]; ok {
		_ = db.Close()

		delete(s.dbs, repoID)
	}
}

// Close closes all database connections.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var errs []error

	for _, db := range s.dbs {
		err := db.Close()
		if err != nil {
			errs = append(errs, err)
		}
	}

	s.dbs = make(map[int64]*sql.DB)

	if len(errs) > 0 {
		return fmt.Errorf("errors closing databases: %v", errs)
	}

	return nil
}

// DeleteIndex removes the SCIP database files for a repository, freeing disk space.
// Unlike ClearIndex (which empties tables), this removes the SQLite files entirely.
func (s *Store) DeleteIndex(ctx context.Context, repoID int64) error {
	s.mu.Lock()

	if db, ok := s.dbs[repoID]; ok {
		db.Close()
		delete(s.dbs, repoID)
	}

	s.mu.Unlock()

	// Remove the main database file and WAL/SHM files
	base := filepath.Join(s.cacheDir, fmt.Sprintf("scip_%d.db", repoID))
	for _, f := range []string{base, base + "-wal", base + "-shm"} {
		if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", f, err)
		}
	}

	return nil
}

// ListIndexedRepoIDs returns the repo IDs that have SCIP database files on disk.
func (s *Store) ListIndexedRepoIDs() ([]int64, error) {
	matches, err := filepath.Glob(filepath.Join(s.cacheDir, "scip_*.db"))
	if err != nil {
		return nil, fmt.Errorf("glob scip databases: %w", err)
	}

	var ids []int64

	for _, m := range matches {
		var id int64

		base := filepath.Base(m)
		if _, err := fmt.Sscanf(base, "scip_%d.db", &id); err == nil {
			ids = append(ids, id)
		}
	}

	return ids, nil
}

// SymbolSearchResult represents a symbol found by search.
type SymbolSearchResult struct {
	Symbol        string `json:"symbol"`
	DisplayName   string `json:"displayName,omitempty"`
	FilePath      string `json:"filePath"`
	Line          int    `json:"line"`
	Documentation string `json:"documentation,omitempty"`
}

// SearchSymbols searches for symbols by name pattern.
func (s *Store) SearchSymbols(
	ctx context.Context,
	repoID int64,
	query string,
	limit int,
) ([]SymbolSearchResult, error) {
	db, err := s.getDB(repoID)
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 50
	}

	// Search for symbols containing the query (case-insensitive)
	// Look in both symbol names and display names
	rows, err := db.QueryContext(ctx, `
		SELECT DISTINCT o.symbol, si.display_name, o.file_path, o.start_line, si.documentation
		FROM occurrences o
		LEFT JOIN symbol_info si ON o.symbol = si.symbol
		WHERE (o.role & 1) = 1  -- Only definitions
		  AND (
			o.symbol LIKE '%' || ? || '%'
			OR si.display_name LIKE '%' || ? || '%'
		  )
		ORDER BY 
			CASE WHEN si.display_name = ? THEN 0
			     WHEN si.display_name LIKE ? || '%' THEN 1
			     ELSE 2 END,
			o.file_path
		LIMIT ?
	`, query, query, query, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SymbolSearchResult

	for rows.Next() {
		var (
			r                          SymbolSearchResult
			displayName, documentation sql.NullString
		)

		err := rows.Scan(&r.Symbol, &displayName, &r.FilePath, &r.Line, &documentation)
		if err != nil {
			return nil, err
		}

		r.DisplayName = displayName.String
		r.Documentation = documentation.String
		results = append(results, r)
	}

	return results, rows.Err()
}

// ListFiles returns all files in the SCIP index.
func (s *Store) ListFiles(ctx context.Context, repoID int64, limit int) ([]string, error) {
	db, err := s.getDB(repoID)
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 100
	}

	rows, err := db.QueryContext(ctx, `
		SELECT DISTINCT file_path FROM occurrences ORDER BY file_path LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []string

	for rows.Next() {
		var f string

		err := rows.Scan(&f)
		if err != nil {
			return nil, err
		}

		files = append(files, f)
	}

	return files, rows.Err()
}
