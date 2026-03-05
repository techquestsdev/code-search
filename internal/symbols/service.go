// Package symbols provides a unified symbol extraction service.
package symbols

import (
	"context"
	"sort"
)

// Service provides symbol extraction with caching.
type Service struct {
	treeSitter *TreeSitterService
	cache      *SymbolCache
}

// NewService creates a new symbol service.
func NewService(cacheDir string) (*Service, error) {
	cache, err := NewSymbolCache(cacheDir)
	if err != nil {
		return nil, err
	}

	return &Service{
		treeSitter: NewTreeSitterService(),
		cache:      cache,
	}, nil
}

// NewServiceWithoutCache creates a symbol service without caching (useful for testing).
func NewServiceWithoutCache() *Service {
	return &Service{
		treeSitter: NewTreeSitterService(),
		cache:      nil,
	}
}

// ExtractSymbols extracts symbols from code, using cache when available.
func (s *Service) ExtractSymbols(
	ctx context.Context,
	repoID int64,
	filePath, commitSHA string,
	code []byte,
	language string,
) ([]Symbol, error) {
	// Try cache first
	if s.cache != nil && commitSHA != "" {
		symbols, found, err := s.cache.GetSymbols(ctx, repoID, filePath, commitSHA)
		if err == nil && found {
			return symbols, nil
		}
	}

	// Extract using Tree-sitter
	symbols, err := s.treeSitter.ExtractSymbols(ctx, code, language)
	if err != nil {
		return nil, err
	}

	// Set file path on all symbols
	for i := range symbols {
		symbols[i].FilePath = filePath
	}

	// Sort by line number
	sort.Slice(symbols, func(i, j int) bool {
		return symbols[i].Line < symbols[j].Line
	})

	// Cache the results
	if s.cache != nil && commitSHA != "" && len(symbols) > 0 {
		_ = s.cache.SetSymbols(ctx, repoID, filePath, commitSHA, symbols)
	}

	return symbols, nil
}

// GetSymbolAtPosition returns the symbol at a specific position.
func (s *Service) GetSymbolAtPosition(
	ctx context.Context,
	code []byte,
	language string,
	line, col int,
) (*Symbol, error) {
	return s.treeSitter.GetSymbolAtPosition(ctx, code, language, line, col)
}

// SearchSymbols searches for symbols in a repository's cache.
func (s *Service) SearchSymbols(
	ctx context.Context,
	repoID int64,
	query, kind string,
	limit int,
) ([]Symbol, error) {
	if s.cache == nil {
		return nil, nil
	}

	return s.cache.SearchSymbols(ctx, repoID, query, kind, limit)
}

// InvalidateFile invalidates cached symbols for a file.
func (s *Service) InvalidateFile(ctx context.Context, repoID int64, filePath string) error {
	if s.cache == nil {
		return nil
	}

	return s.cache.InvalidateFile(ctx, repoID, filePath)
}

// InvalidateRepo invalidates all cached symbols for a repository.
func (s *Service) InvalidateRepo(ctx context.Context, repoID int64) error {
	if s.cache == nil {
		return nil
	}

	return s.cache.InvalidateRepo(ctx, repoID)
}

// Stats returns cache statistics.
func (s *Service) Stats(ctx context.Context, repoID int64) (map[string]any, error) {
	if s.cache == nil {
		return map[string]any{}, nil
	}

	return s.cache.Stats(ctx, repoID)
}

// IsLanguageSupported checks if Tree-sitter supports a language.
func (s *Service) IsLanguageSupported(language string) bool {
	return s.treeSitter.IsSupported(language)
}

// SupportedLanguages returns the list of supported languages.
func (s *Service) SupportedLanguages() []string {
	return s.treeSitter.SupportedLanguages()
}

// Close closes the service and releases resources.
func (s *Service) Close() error {
	if s.cache != nil {
		return s.cache.Close()
	}

	return nil
}
