package search

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/aanogueira/code-search/internal/sharding"
)

// ShardedClient manages multiple Zoekt clients for horizontal scaling.
type ShardedClient struct {
	clients []*Client
	shards  []string // URLs of each shard
}

// NewShardedClient creates a client that can query multiple Zoekt shards
// shardURLs can be:
// - Single URL: "http://localhost:6070" (no sharding)
// - Multiple URLs: "http://zoekt-0:6070,http://zoekt-1:6070,http://zoekt-2:6070"
// - Kubernetes headless service pattern: "zoekt-{0..2}.zoekt.default.svc:6070".
func NewShardedClient(shardURLs string) *ShardedClient {
	urls := parseShardURLs(shardURLs)

	clients := make([]*Client, len(urls))
	for i, url := range urls {
		clients[i] = NewClient(url)
	}

	return &ShardedClient{
		clients: clients,
		shards:  urls,
	}
}

// parseShardURLs parses comma-separated URLs or expands patterns.
func parseShardURLs(input string) []string {
	// Simple comma-separated parsing for now
	urls := make([]string, 0)
	current := ""

	for _, c := range input {
		if c == ',' {
			if current != "" {
				urls = append(urls, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}

	if current != "" {
		urls = append(urls, current)
	}

	if len(urls) == 0 {
		urls = append(urls, "http://localhost:6070")
	}

	return urls
}

// GetShardForRepo returns the shard index for a repository
// Uses consistent hashing based on repo name.
func (sc *ShardedClient) GetShardForRepo(repoName string) int {
	return sharding.GetShardForRepo(repoName, len(sc.clients))
}

// GetClientForRepo returns the client for a specific repository.
func (sc *ShardedClient) GetClientForRepo(repoName string) *Client {
	return sc.clients[sc.GetShardForRepo(repoName)]
}

// Search queries all shards in parallel and merges results.
func (sc *ShardedClient) Search(ctx context.Context, q Query) (*SearchResponse, error) {
	if len(sc.clients) == 1 {
		return sc.clients[0].Search(ctx, q)
	}

	type shardResult struct {
		response *SearchResponse
		err      error
		shard    int
	}

	results := make(chan shardResult, len(sc.clients))

	var wg sync.WaitGroup

	// Apply per-shard result limit to bound memory during fan-out.
	// Each shard returns up to 2x the requested limit; we truncate after merge.
	shardQuery := q
	if shardQuery.MaxResults > 0 {
		shardQuery.MaxResults = shardQuery.MaxResults * 2
	}

	// Query all shards in parallel
	for i, client := range sc.clients {
		wg.Add(1)

		go func(idx int, c *Client) {
			defer wg.Done()

			resp, err := c.Search(ctx, shardQuery)
			results <- shardResult{response: resp, err: err, shard: idx}
		}(i, client)
	}

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect and merge results
	var merged SearchResponse

	merged.Stats.FilesLoaded = 0
	merged.Results = make([]Result, 0)

	var (
		totalDuration time.Duration
		errors        []error
	)

	for result := range results {
		if result.err != nil {
			errors = append(errors, fmt.Errorf("shard %d: %w", result.shard, result.err))
			continue
		}

		if result.response != nil {
			merged.Results = append(merged.Results, result.response.Results...)
			merged.TotalMatches += result.response.TotalMatches
			merged.Stats.FilesLoaded += result.response.Stats.FilesLoaded
			merged.Stats.FilesConsidered += result.response.Stats.FilesConsidered

			merged.Stats.ShardsScanned += result.response.Stats.ShardsScanned
			if result.response.Duration > totalDuration {
				totalDuration = result.response.Duration
			}
		}
	}

	merged.Duration = totalDuration

	// Sort results by repo for consistent ordering
	sort.Slice(merged.Results, func(i, j int) bool {
		return merged.Results[i].Repo < merged.Results[j].Repo
	})

	// Apply limit after merging
	if q.MaxResults > 0 && len(merged.Results) > q.MaxResults {
		merged.Results = merged.Results[:q.MaxResults]
	}

	// Return first error if all shards failed
	if len(errors) == len(sc.clients) {
		return nil, fmt.Errorf("all shards failed: %w", errors[0])
	}

	return &merged, nil
}

// ListRepos queries all shards and returns combined repo list.
func (sc *ShardedClient) ListRepos(ctx context.Context) ([]RepoInfo, error) {
	if len(sc.clients) == 1 {
		return sc.clients[0].ListRepos(ctx)
	}

	type shardResult struct {
		repos []RepoInfo
		err   error
		shard int
	}

	results := make(chan shardResult, len(sc.clients))

	var wg sync.WaitGroup

	for i, client := range sc.clients {
		wg.Add(1)

		go func(idx int, c *Client) {
			defer wg.Done()

			repos, err := c.ListRepos(ctx)
			results <- shardResult{repos: repos, err: err, shard: idx}
		}(i, client)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	allRepos := make([]RepoInfo, 0)

	var errors []error

	for result := range results {
		if result.err != nil {
			errors = append(errors, fmt.Errorf("shard %d: %w", result.shard, result.err))
			continue
		}
		// Add shard info to each repo
		for i := range result.repos {
			result.repos[i].Shard = result.shard
		}

		allRepos = append(allRepos, result.repos...)
	}

	if len(errors) == len(sc.clients) {
		return nil, fmt.Errorf("all shards failed: %w", errors[0])
	}

	// Sort by name for consistent ordering
	sort.Slice(allRepos, func(i, j int) bool {
		return allRepos[i].Name < allRepos[j].Name
	})

	return allRepos, nil
}

// Health checks all shards and returns overall health.
func (sc *ShardedClient) Health(ctx context.Context) error {
	if len(sc.clients) == 1 {
		return sc.clients[0].Health(ctx)
	}

	var wg sync.WaitGroup

	errors := make(chan error, len(sc.clients))

	for i, client := range sc.clients {
		wg.Add(1)

		go func(idx int, c *Client) {
			defer wg.Done()

			err := c.Health(ctx)
			if err != nil {
				errors <- fmt.Errorf("shard %d: %w", idx, err)
			}
		}(i, client)
	}

	wg.Wait()
	close(errors)

	// Collect errors
	var errs []error
	for err := range errors {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("%d/%d shards unhealthy: %w", len(errs), len(sc.clients), errs[0])
	}

	return nil
}

// ShardCount returns the number of shards.
func (sc *ShardedClient) ShardCount() int {
	return len(sc.clients)
}

// ShardURLs returns the URLs of all shards.
func (sc *ShardedClient) ShardURLs() []string {
	return sc.shards
}

// Close closes all client connections.
func (sc *ShardedClient) Close() error {
	var firstErr error

	for _, client := range sc.clients {
		err := client.Close()
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// StreamSearch performs streaming search across all shards in parallel.
// Results from all shards are merged into a single channel as they arrive.
// This provides true streaming with faster time-to-first-result.
func (sc *ShardedClient) StreamSearch(ctx context.Context, q Query) <-chan StreamSearchResult {
	results := make(chan StreamSearchResult, 100)

	// For single shard, just pass through
	if len(sc.clients) == 1 {
		go func() {
			defer close(results)

			for result := range sc.clients[0].StreamSearch(ctx, q) {
				results <- result
			}
		}()

		return results
	}

	go func() {
		defer close(results)

		var wg sync.WaitGroup

		// Aggregate stats from all shards
		var (
			statsMu     sync.Mutex
			totalStats  Stats
			shardsDone  int
			shardsTotal = len(sc.clients)
		)

		// Start streaming search on all shards in parallel
		for shardIdx, client := range sc.clients {
			wg.Add(1)

			go func(idx int, c *Client) {
				defer wg.Done()

				for streamResult := range c.StreamSearch(ctx, q) {
					if streamResult.Error != nil {
						// Send error but continue - partial results are still useful
						select {
						case results <- StreamSearchResult{
							Error: fmt.Errorf("shard %d: %w", idx, streamResult.Error),
						}:
						case <-ctx.Done():
							return
						}

						continue
					}

					if streamResult.Result != nil {
						// Forward result directly
						select {
						case results <- streamResult:
						case <-ctx.Done():
							return
						}
					}

					if streamResult.Stats != nil {
						// Aggregate stats
						statsMu.Lock()

						totalStats.FilesConsidered += streamResult.Stats.FilesConsidered
						totalStats.FilesLoaded += streamResult.Stats.FilesLoaded
						totalStats.FilesSkipped += streamResult.Stats.FilesSkipped

						totalStats.ShardsScanned += streamResult.Stats.ShardsScanned
						if streamResult.Stats.Duration > totalStats.Duration {
							totalStats.Duration = streamResult.Stats.Duration
						}

						statsMu.Unlock()
					}

					if streamResult.Done {
						statsMu.Lock()

						shardsDone++
						allDone := shardsDone == shardsTotal

						statsMu.Unlock()

						if allDone {
							// All shards completed - send final done signal
							select {
							case results <- StreamSearchResult{
								Stats: &totalStats,
								Done:  true,
							}:
							case <-ctx.Done():
							}
						}
					}
				}
			}(shardIdx, client)
		}

		wg.Wait()
	}()

	return results
}
