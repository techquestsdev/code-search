package replace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/techquestsdev/code-search/internal/queue"
)

// FederatedClient handles proxying replace requests to the correct indexer shards.
type FederatedClient struct {
	indexerService string // Headless service name (e.g., "code-search-indexer-headless")
	indexerPort    int    // Port for indexer HTTP API
	totalShards    int    // Total number of shards
	httpClient     *http.Client
}

// NewFederatedClient creates a client for federated replace operations.
func NewFederatedClient(indexerService string, indexerPort, totalShards int) *FederatedClient {
	return &FederatedClient{
		indexerService: indexerService,
		indexerPort:    indexerPort,
		totalShards:    totalShards,
		httpClient: &http.Client{
			Timeout: 10 * time.Minute, // Replace operations can take a while
		},
	}
}

// GetShardForRepo returns which shard owns a repo.
func (c *FederatedClient) GetShardForRepo(repoName string) int {
	if c.totalShards <= 1 {
		return 0
	}

	h := fnv.New32a()
	h.Write([]byte(repoName))

	return int(h.Sum32() % uint32(c.totalShards))
}

// getShardURL returns the URL for a specific shard.
func (c *FederatedClient) getShardURL(shard int) string {
	podName := fmt.Sprintf("code-search-indexer-%d", shard)
	return fmt.Sprintf("http://%s.%s:%d", podName, c.indexerService, c.indexerPort)
}

// ExecuteRequest is the request body for federated replace execution.
type ExecuteRequest struct {
	SearchPattern string               `json:"search_pattern"`
	ReplaceWith   string               `json:"replace_with"`
	IsRegex       bool                 `json:"is_regex"`
	CaseSensitive bool                 `json:"case_sensitive"`
	Matches       []queue.ReplaceMatch `json:"matches"`
	BranchName    string               `json:"branch_name,omitempty"`
	MRTitle       string               `json:"mr_title,omitempty"`
	MRDescription string               `json:"mr_description,omitempty"`
	UserTokens    map[string]string    `json:"user_tokens,omitempty"`
	ReposReadOnly bool                 `json:"repos_readonly"`
}

// ShardResult holds the result from one shard.
type ShardResult struct {
	Shard  int
	Result *ExecuteResult
	Error  error
}

// Execute fans out replace operations to the correct indexer shards.
func (c *FederatedClient) Execute(
	ctx context.Context,
	opts ExecuteOptions,
) (*ExecuteResult, error) {
	// Group matches by shard
	shardMatches := make(map[int][]queue.ReplaceMatch)

	for _, m := range opts.Matches {
		shard := c.GetShardForRepo(m.RepositoryName)
		shardMatches[shard] = append(shardMatches[shard], m)
	}

	if len(shardMatches) == 0 {
		return &ExecuteResult{
			TotalFiles:    0,
			ModifiedFiles: 0,
			RepoResults:   []RepoResult{},
		}, nil
	}

	// Execute on each shard in parallel
	var wg sync.WaitGroup

	results := make(chan ShardResult, len(shardMatches))

	for shard, matches := range shardMatches {
		wg.Add(1)

		go func(s int, m []queue.ReplaceMatch) {
			defer wg.Done()

			result, err := c.executeOnShard(ctx, s, m, opts)
			results <- ShardResult{Shard: s, Result: result, Error: err}
		}(shard, matches)
	}

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Merge results from all shards
	merged := &ExecuteResult{
		RepoResults: []RepoResult{},
	}

	var errors []string

	for result := range results {
		if result.Error != nil {
			errors = append(errors, fmt.Sprintf("shard %d: %v", result.Shard, result.Error))
			continue
		}

		if result.Result != nil {
			merged.TotalFiles += result.Result.TotalFiles
			merged.ModifiedFiles += result.Result.ModifiedFiles
			merged.RepoResults = append(merged.RepoResults, result.Result.RepoResults...)
		}
	}

	if len(errors) > 0 && len(merged.RepoResults) == 0 {
		return nil, fmt.Errorf("all shards failed: %v", errors)
	}

	// If some shards failed but others succeeded, include error info
	if len(errors) > 0 {
		merged.RepoResults = append(merged.RepoResults, RepoResult{
			RepositoryName: "[shard-errors]",
			Error:          fmt.Sprintf("Some shards failed: %v", errors),
		})
	}

	return merged, nil
}

// executeOnShard sends the replace request to a specific shard.
func (c *FederatedClient) executeOnShard(
	ctx context.Context,
	shard int,
	matches []queue.ReplaceMatch,
	opts ExecuteOptions,
) (*ExecuteResult, error) {
	shardURL := c.getShardURL(shard)

	req := ExecuteRequest{
		SearchPattern: opts.SearchPattern,
		ReplaceWith:   opts.ReplaceWith,
		IsRegex:       opts.IsRegex,
		CaseSensitive: opts.CaseSensitive,
		Matches:       matches,
		BranchName:    opts.BranchName,
		MRTitle:       opts.MRTitle,
		MRDescription: opts.MRDescription,
		UserTokens:    opts.UserTokens,
		ReposReadOnly: opts.ReposReadOnly,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		shardURL+"/replace/execute", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("shard %d unreachable: %w", shard, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("shard %d returned %d: %s", shard, resp.StatusCode, string(respBody))
	}

	var result ExecuteResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}
