package search

import (
	"context"
	"time"
)

// Service provides search functionality.
type Service struct {
	client        *Client        // Single client (for backward compatibility)
	shardedClient *ShardedClient // Sharded client (for multi-shard deployments)
	useSharding   bool
}

// NewService creates a new search service with a single Zoekt backend.
func NewService(zoektURL string) *Service {
	return &Service{
		client:      NewClient(zoektURL),
		useSharding: false,
	}
}

// NewShardedService creates a search service with multiple Zoekt shards
// zoektURLs should be comma-separated: "http://zoekt-0:6070,http://zoekt-1:6070"
func NewShardedService(zoektURLs string) *Service {
	shardedClient := NewShardedClient(zoektURLs)

	// If only one shard, use regular client for simplicity
	if len(shardedClient.clients) == 1 {
		return &Service{
			client:      shardedClient.clients[0],
			useSharding: false,
		}
	}

	return &Service{
		shardedClient: shardedClient,
		useSharding:   true,
	}
}

// SearchRequest represents a search request.
type SearchRequest struct {
	Query         string   `json:"query"`
	IsRegex       bool     `json:"is_regex"`
	CaseSensitive bool     `json:"case_sensitive"`
	Repos         []string `json:"repos,omitempty"`
	Languages     []string `json:"languages,omitempty"`
	FilePatterns  []string `json:"file_patterns,omitempty"`
	Limit         int      `json:"limit"`
	ContextLines  int      `json:"context_lines"`
}

// SearchResult represents a single search result for API responses.
type SearchResult struct {
	Repo       string        `json:"repo"`
	File       string        `json:"file"`
	Line       int           `json:"line"`
	Column     int           `json:"column"`
	Content    string        `json:"content"`
	Context    ResultContext `json:"context"`
	Language   string        `json:"language"`
	MatchStart int           `json:"match_start"`
	MatchEnd   int           `json:"match_end"`
}

// ResultContext represents lines around a match.
type ResultContext struct {
	Before []string `json:"before"`
	After  []string `json:"after"`
}

// SearchResults represents the search response.
type SearchResults struct {
	Results    []SearchResult `json:"results"`
	TotalCount int            `json:"total_count"`
	Truncated  bool           `json:"truncated"`
	Duration   time.Duration  `json:"duration"`
	Stats      SearchStats    `json:"stats"`
}

// SearchStats contains search statistics.
type SearchStats struct {
	FilesSearched int `json:"files_searched"`
	ReposSearched int `json:"repos_searched"`
}

// Search performs a search and returns formatted results.
func (s *Service) Search(ctx context.Context, req SearchRequest) (*SearchResults, error) {
	// Set defaults - 0 means unlimited (use high value for zoekt)
	if req.Limit == 0 {
		req.Limit = 10000
	}

	// Build query
	q := Query{
		Pattern:       req.Query,
		IsRegex:       req.IsRegex,
		CaseSensitive: req.CaseSensitive,
		Repos:         req.Repos,
		Languages:     req.Languages,
		FilePatterns:  req.FilePatterns,
		MaxResults:    req.Limit,
	}

	// Execute search (use sharded client if available)
	var (
		resp *SearchResponse
		err  error
	)

	if s.useSharding {
		resp, err = s.shardedClient.Search(ctx, q)
	} else {
		resp, err = s.client.Search(ctx, q)
	}

	if err != nil {
		return nil, err
	}

	// Convert to API response format
	results := make([]SearchResult, 0)

	for _, repoResult := range resp.Results {
		for _, file := range repoResult.Files {
			for _, match := range file.Matches {
				result := SearchResult{
					Repo:       repoResult.Repo,
					File:       file.Name,
					Line:       match.LineNum,
					Column:     match.Start,
					Content:    match.Line,
					Language:   file.Language,
					MatchStart: match.Start,
					MatchEnd:   match.End,
					Context: ResultContext{
						Before: splitLines(match.Before),
						After:  splitLines(match.After),
					},
				}
				results = append(results, result)

				// Stop if we've reached the limit
				if len(results) >= req.Limit {
					break
				}
			}

			if len(results) >= req.Limit {
				break
			}
		}

		if len(results) >= req.Limit {
			break
		}
	}

	return &SearchResults{
		Results:    results,
		TotalCount: resp.TotalMatches,
		Truncated:  len(results) >= req.Limit && resp.TotalMatches > len(results),
		Duration:   resp.Duration,
		Stats: SearchStats{
			FilesSearched: resp.Stats.FilesLoaded,
			ReposSearched: len(resp.Results),
		},
	}, nil
}

// splitLines splits a string into lines, handling empty strings.
func splitLines(s string) []string {
	if s == "" {
		return []string{}
	}

	lines := make([]string, 0)
	start := 0

	for i := range len(s) {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}

	if start < len(s) {
		lines = append(lines, s[start:])
	}

	return lines
}

// StreamSearchResult represents a single result event from streaming search.
type StreamSearchEvent struct {
	Result *SearchResult // A single search result
	Stats  *SearchStats  // Stats update
	Done   bool          // Stream complete
	Error  error         // Error if something went wrong
}

// StreamSearch performs streaming search and sends results as they arrive.
// This provides faster time-to-first-result compared to batch Search.
func (s *Service) StreamSearch(ctx context.Context, req SearchRequest) <-chan StreamSearchEvent {
	results := make(chan StreamSearchEvent, 100)

	go func() {
		defer close(results)

		// Set defaults
		if req.Limit == 0 {
			req.Limit = 10000
		}

		// Build query
		q := Query{
			Pattern:       req.Query,
			IsRegex:       req.IsRegex,
			CaseSensitive: req.CaseSensitive,
			Repos:         req.Repos,
			Languages:     req.Languages,
			FilePatterns:  req.FilePatterns,
			MaxResults:    req.Limit,
		}

		// Get the appropriate stream
		var streamCh <-chan StreamSearchResult
		if s.useSharding {
			streamCh = s.shardedClient.StreamSearch(ctx, q)
		} else {
			streamCh = s.client.StreamSearch(ctx, q)
		}

		resultCount := 0
		limitReached := false

		for streamResult := range streamCh {
			// Check context cancellation
			select {
			case <-ctx.Done():
				return
			default:
			}

			if streamResult.Error != nil {
				results <- StreamSearchEvent{Error: streamResult.Error}
				continue
			}

			if streamResult.Result != nil && !limitReached {
				// Convert client Result to API SearchResult format
				for _, file := range streamResult.Result.Files {
					for _, match := range file.Matches {
						if resultCount >= req.Limit {
							limitReached = true
							break
						}

						result := &SearchResult{
							Repo:       streamResult.Result.Repo,
							File:       file.Name,
							Line:       match.LineNum,
							Column:     match.Start,
							Content:    match.Line,
							Language:   file.Language,
							MatchStart: match.Start,
							MatchEnd:   match.End,
							Context: ResultContext{
								Before: splitLines(match.Before),
								After:  splitLines(match.After),
							},
						}
						results <- StreamSearchEvent{Result: result}

						resultCount++
					}

					if limitReached {
						break
					}
				}
			}

			if streamResult.Stats != nil {
				results <- StreamSearchEvent{
					Stats: &SearchStats{
						FilesSearched: streamResult.Stats.FilesLoaded,
						ReposSearched: streamResult.Stats.ShardsScanned, // Approximation
					},
				}
			}

			if streamResult.Done || limitReached {
				results <- StreamSearchEvent{Done: true}
				return
			}
		}
	}()

	return results
}

// Health checks if the search service is healthy.
func (s *Service) Health(ctx context.Context) error {
	if s.useSharding {
		return s.shardedClient.Health(ctx)
	}

	return s.client.Health(ctx)
}

// ListIndexedRepos returns a list of indexed repositories from Zoekt.
func (s *Service) ListIndexedRepos(ctx context.Context) ([]RepoInfo, error) {
	if s.useSharding {
		return s.shardedClient.ListRepos(ctx)
	}

	return s.client.ListRepos(ctx)
}

// Close closes all client connections.
func (s *Service) Close() error {
	if s.useSharding {
		return s.shardedClient.Close()
	}

	return s.client.Close()
}
