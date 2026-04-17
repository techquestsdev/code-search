// Package scip provides the main service for SCIP code intelligence.
package scip

import (
	"context"
	"io"
	"os"

	"go.uber.org/zap"
)

// Service provides SCIP-based code intelligence.
type Service struct {
	store   *Store
	parser  *Parser
	indexer *Indexer
	logger  *zap.Logger
}

// NewService creates a new SCIP service.
// cacheDir should be the directory where SCIP databases are stored (e.g., data/scip).
func NewService(cacheDir string, logger *zap.Logger) (*Service, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, err
	}

	store, err := NewStore(cacheDir, logger)
	if err != nil {
		return nil, err
	}

	parser := NewParser(store)
	indexer := NewIndexer(DefaultIndexerConfig(), store, logger)

	return &Service{
		store:   store,
		parser:  parser,
		indexer: indexer,
		logger:  logger,
	}, nil
}

// NewServiceWithConfig creates a new SCIP service with an explicit IndexerConfig.
// This allows callers to configure indexer binary paths and timeouts.
func NewServiceWithConfig(
	cacheDir string,
	indexerCfg IndexerConfig,
	logger *zap.Logger,
) (*Service, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, err
	}

	store, err := NewStore(cacheDir, logger)
	if err != nil {
		return nil, err
	}

	parser := NewParser(store)
	indexer := NewIndexer(indexerCfg, store, logger)

	return &Service{
		store:   store,
		parser:  parser,
		indexer: indexer,
		logger:  logger,
	}, nil
}

// HasIndex returns true if the repository has a SCIP index.
func (s *Service) HasIndex(ctx context.Context, repoID int64) (bool, error) {
	return s.store.HasIndex(ctx, repoID)
}

// GetStats returns statistics about the SCIP index for a repository.
func (s *Service) GetStats(ctx context.Context, repoID int64) (map[string]any, error) {
	return s.store.Stats(ctx, repoID)
}

// GoToDefinition finds the definition of the symbol at the given position.
// Line is 1-indexed (from editor), column is 0-indexed.
// Returns nil if no SCIP index is available or no symbol is at the position.
func (s *Service) GoToDefinition(
	ctx context.Context,
	repoID int64,
	filePath string,
	line, col int,
) (*DefinitionResult, error) {
	hasIndex, err := s.store.HasIndex(ctx, repoID)
	if err != nil {
		return nil, err
	}

	if !hasIndex {
		return nil, nil
	}

	// Convert 1-indexed line to 0-indexed for database query
	dbLine := max(line-1, 0)

	return s.store.GoToDefinition(ctx, repoID, filePath, dbLine, col)
}

// FindReferences finds all references to the symbol at the given position.
// Line is 1-indexed (from editor), column is 0-indexed.
// Returns nil if no SCIP index is available or no symbol is at the position.
func (s *Service) FindReferences(
	ctx context.Context,
	repoID int64,
	filePath string,
	line, col int,
	limit int,
) (*ReferencesResult, error) {
	hasIndex, err := s.store.HasIndex(ctx, repoID)
	if err != nil {
		return nil, err
	}

	if !hasIndex {
		return nil, nil
	}

	// Convert 1-indexed line to 0-indexed for database query
	dbLine := max(line-1, 0)

	return s.store.FindReferences(ctx, repoID, filePath, dbLine, col, limit)
}

// GetOccurrenceAtPosition returns the symbol occurrence at a specific position.
// Line is 1-indexed (from editor), column is 0-indexed.
func (s *Service) GetOccurrenceAtPosition(
	ctx context.Context,
	repoID int64,
	filePath string,
	line, col int,
) (*Occurrence, error) {
	// Convert 1-indexed line to 0-indexed for database query
	dbLine := max(line-1, 0)

	return s.store.GetOccurrenceAtPosition(ctx, repoID, filePath, dbLine, col)
}

// GetSymbolInfo returns information about a symbol.
func (s *Service) GetSymbolInfo(
	ctx context.Context,
	repoID int64,
	symbol string,
) (*SymbolInfo, error) {
	return s.store.GetSymbolInfo(ctx, repoID, symbol)
}

// ImportFromFile imports a SCIP index from a file.
func (s *Service) ImportFromFile(ctx context.Context, repoID int64, filePath string) error {
	return s.parser.ImportFromFile(ctx, repoID, filePath)
}

// ImportFromReader imports a SCIP index from a reader.
func (s *Service) ImportFromReader(ctx context.Context, repoID int64, r io.Reader) error {
	return s.parser.ImportFromReader(ctx, repoID, r)
}

// ImportFromBytes imports a SCIP index from raw bytes.
func (s *Service) ImportFromBytes(ctx context.Context, repoID int64, data []byte) error {
	return s.parser.ImportFromBytes(ctx, repoID, data)
}

// Index runs the appropriate SCIP indexer for a repository.
func (s *Service) Index(
	ctx context.Context,
	repoID int64,
	repoPath string,
	language string,
) (*IndexResult, error) {
	return s.indexer.Index(ctx, repoID, repoPath, language)
}

// IndexWithOptions runs the appropriate SCIP indexer with optional credentials.
func (s *Service) IndexWithOptions(
	ctx context.Context,
	repoID int64,
	repoPath string,
	language string,
	opts *IndexOptions,
) (*IndexResult, error) {
	return s.indexer.IndexWithOptions(ctx, repoID, repoPath, language, opts)
}

// DetectLanguage detects the primary language of a repository.
func (s *Service) DetectLanguage(repoPath string) (string, error) {
	return s.indexer.DetectLanguage(repoPath)
}

// DetectLanguages detects all languages present in a repository.
// Languages are returned in priority order.
func (s *Service) DetectLanguages(repoPath string) []string {
	return s.indexer.DetectLanguages(repoPath)
}

// GetAvailableIndexers returns which indexers are available.
func (s *Service) GetAvailableIndexers() map[string]bool {
	return s.indexer.GetAvailableIndexers()
}

// IsIndexerAvailable checks if an indexer is available for a language.
func (s *Service) IsIndexerAvailable(language string) bool {
	return s.indexer.IsIndexerAvailable(language)
}

// SupportedLanguages returns the list of supported languages.
func (s *Service) SupportedLanguages() []string {
	return s.indexer.SupportedLanguages()
}

// ClearIndex removes the SCIP index for a repository.
func (s *Service) ClearIndex(ctx context.Context, repoID int64) error {
	return s.store.ClearIndex(ctx, repoID)
}

// DeleteIndex removes the SCIP database files for a repository entirely.
func (s *Service) DeleteIndex(ctx context.Context, repoID int64) error {
	return s.store.DeleteIndex(ctx, repoID)
}

// ListIndexedRepoIDs returns repo IDs that have SCIP database files on disk.
func (s *Service) ListIndexedRepoIDs() ([]int64, error) {
	return s.store.ListIndexedRepoIDs()
}

// SearchSymbols searches for symbols by name pattern.
func (s *Service) SearchSymbols(
	ctx context.Context,
	repoID int64,
	query string,
	limit int,
) ([]SymbolSearchResult, error) {
	return s.store.SearchSymbols(ctx, repoID, query, limit)
}

// ListFiles returns all indexed files for a repository.
func (s *Service) ListFiles(ctx context.Context, repoID int64, limit int) ([]string, error) {
	return s.store.ListFiles(ctx, repoID, limit)
}

// EvictCache evicts the in-memory cached connection for a repo.
func (s *Service) EvictCache(repoID int64) {
	s.store.EvictCache(repoID)
}

// Close releases resources.
func (s *Service) Close() error {
	return s.store.Close()
}
