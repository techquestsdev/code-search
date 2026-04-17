package scip

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/techquestsdev/code-search/internal/sharding"
)

// FederatedClient proxies SCIP requests to the correct indexer shard
// based on repository ownership (FNV-1a hash on repo name).
type FederatedClient struct {
	indexerService string
	indexerPort    int
	totalShards    int
	httpClient     *http.Client
}

// NewFederatedClient creates a client for federated SCIP access.
func NewFederatedClient(indexerService string, indexerPort, totalShards int) *FederatedClient {
	return &FederatedClient{
		indexerService: indexerService,
		indexerPort:    indexerPort,
		totalShards:    totalShards,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *FederatedClient) getShardForRepo(repoName string) int {
	return sharding.GetShardForRepo(repoName, c.totalShards)
}

func (c *FederatedClient) getShardURL(shard int) string {
	podName := fmt.Sprintf("code-search-indexer-%d", shard)
	return fmt.Sprintf("http://%s.%s:%d", podName, c.indexerService, c.indexerPort)
}

// HasIndex checks if a SCIP index exists for a repo on its owning shard.
func (c *FederatedClient) HasIndex(
	ctx context.Context,
	repoName string,
	repoID int64,
) (bool, error) {
	shard := c.getShardForRepo(repoName)
	shardURL := c.getShardURL(shard)

	reqURL := fmt.Sprintf("%s/scip/repos/%d/status", shardURL, repoID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return false, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("shard %d unreachable: %w", shard, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, nil
	}

	var result struct {
		HasIndex bool `json:"has_index"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}

	return result.HasIndex, nil
}

// GoToDefinition proxies a go-to-definition request to the owning shard.
func (c *FederatedClient) GoToDefinition(
	ctx context.Context,
	repoName string,
	repoID int64,
	filePath string,
	line, column int,
) (json.RawMessage, error) {
	shard := c.getShardForRepo(repoName)
	shardURL := c.getShardURL(shard)

	reqURL := fmt.Sprintf("%s/scip/repos/%d/definition", shardURL, repoID)

	body := fmt.Sprintf(`{"filePath":%q,"line":%d,"column":%d}`,
		filePath, line, column)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL,
		io.NopCloser(strings.NewReader(body)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("shard %d unreachable: %w", shard, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("shard returned %d: %s", resp.StatusCode, string(respBody))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(data), nil
}

// FindReferences proxies a find-references request to the owning shard.
// For cross-repo references, this could fan out to all shards in the future.
func (c *FederatedClient) FindReferences(
	ctx context.Context,
	repoName string,
	repoID int64,
	filePath string,
	line, column, limit int,
) (json.RawMessage, error) {
	shard := c.getShardForRepo(repoName)
	shardURL := c.getShardURL(shard)

	reqURL := fmt.Sprintf("%s/scip/repos/%d/references", shardURL, repoID)

	body := fmt.Sprintf(`{"filePath":%q,"line":%d,"column":%d,"limit":%d}`,
		filePath, line, column, limit)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL,
		io.NopCloser(strings.NewReader(body)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("shard %d unreachable: %w", shard, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("shard returned %d: %s", resp.StatusCode, string(respBody))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(data), nil
}

// GetStats proxies a stats request to the owning shard.
func (c *FederatedClient) GetStats(
	ctx context.Context,
	repoName string,
	repoID int64,
) (map[string]any, error) {
	shard := c.getShardForRepo(repoName)
	shardURL := c.getShardURL(shard)

	reqURL := fmt.Sprintf("%s/scip/repos/%d/status", shardURL, repoID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("shard %d unreachable: %w", shard, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("shard returned %d", resp.StatusCode)
	}

	var result struct {
		Stats map[string]any `json:"stats"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Stats, nil
}

// SearchSymbols proxies a symbol search request to the owning shard.
func (c *FederatedClient) SearchSymbols(
	ctx context.Context,
	repoName string,
	repoID int64,
	query string,
	limit int,
) (json.RawMessage, error) {
	shard := c.getShardForRepo(repoName)
	shardURL := c.getShardURL(shard)

	reqURL := fmt.Sprintf("%s/scip/repos/%d/symbols/search", shardURL, repoID)

	body := fmt.Sprintf(`{"query":%q,"limit":%d}`, query, limit)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL,
		io.NopCloser(strings.NewReader(body)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("shard %d unreachable: %w", shard, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("shard returned %d: %s", resp.StatusCode, string(respBody))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(data), nil
}

// ListFiles proxies a file listing request to the owning shard.
func (c *FederatedClient) ListFiles(
	ctx context.Context,
	repoName string,
	repoID int64,
	limit int,
) (json.RawMessage, error) {
	shard := c.getShardForRepo(repoName)
	shardURL := c.getShardURL(shard)

	reqURL := fmt.Sprintf("%s/scip/repos/%d/files?limit=%d",
		shardURL, repoID, limit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("shard %d unreachable: %w", shard, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("shard returned %d: %s", resp.StatusCode, string(respBody))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(data), nil
}
