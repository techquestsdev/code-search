// Package files provides file browsing and content retrieval for Git repositories.
package files

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/aanogueira/code-search/internal/sharding"
)

// FederatedClient handles proxying file requests to the correct indexer shard.
type FederatedClient struct {
	indexerService string // Headless service name (e.g., "code-search-indexer-headless")
	indexerPort    int    // Port for indexer HTTP API
	totalShards    int    // Total number of shards
	httpClient     *http.Client
}

// NewFederatedClient creates a client for federated file access.
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

// GetShardForRepo returns which shard should handle a repo.
func (c *FederatedClient) GetShardForRepo(repoName string) int {
	return sharding.GetShardForRepo(repoName, c.totalShards)
}

// getShardURL returns the URL for a specific shard.
// In Kubernetes, StatefulSet pods are addressable as: <pod-name>.<headless-service>.
func (c *FederatedClient) getShardURL(shard int) string {
	// Pattern: code-search-indexer-0.code-search-indexer-headless:8081
	podName := fmt.Sprintf("code-search-indexer-%d", shard)
	return fmt.Sprintf("http://%s.%s:%d", podName, c.indexerService, c.indexerPort)
}

// RepoExists checks if a repository exists on its assigned shard.
func (c *FederatedClient) RepoExists(ctx context.Context, repoName string) (bool, error) {
	shard := c.GetShardForRepo(repoName)
	shardURL := c.getShardURL(shard)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/files/%s/exists", shardURL, url.PathEscape(repoName)), nil)
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
		Exists bool `json:"exists"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}

	return result.Exists, nil
}

// ListTree proxies a tree listing request to the correct shard.
func (c *FederatedClient) ListTree(
	ctx context.Context,
	repoName, path, ref string,
) ([]TreeEntry, error) {
	shard := c.GetShardForRepo(repoName)
	shardURL := c.getShardURL(shard)

	reqURL := fmt.Sprintf("%s/files/%s/tree?path=%s&ref=%s",
		shardURL,
		url.PathEscape(repoName),
		url.QueryEscape(path),
		url.QueryEscape(ref),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("shard %d unreachable: %w", shard, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusMisdirectedRequest {
		return nil, fmt.Errorf("repo %s routed to wrong shard", repoName)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("shard returned %d: %s", resp.StatusCode, string(body))
	}

	var entries []TreeEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, err
	}

	return entries, nil
}

// GetBlob proxies a blob request to the correct shard.
func (c *FederatedClient) GetBlob(
	ctx context.Context,
	repoName, path, ref string,
) (*BlobResponse, error) {
	shard := c.GetShardForRepo(repoName)
	shardURL := c.getShardURL(shard)

	reqURL := fmt.Sprintf("%s/files/%s/blob?path=%s&ref=%s",
		shardURL,
		url.PathEscape(repoName),
		url.QueryEscape(path),
		url.QueryEscape(ref),
	)

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
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("shard returned %d: %s", resp.StatusCode, string(body))
	}

	var blob BlobResponse
	if err := json.NewDecoder(resp.Body).Decode(&blob); err != nil {
		return nil, err
	}

	return &blob, nil
}

// GetRawBlob proxies a raw file content request to the correct shard.
// Returns the response body as a ReadCloser — the caller must close it.
func (c *FederatedClient) GetRawBlob(
	ctx context.Context,
	repoName, path, ref string,
) (io.ReadCloser, int64, error) {
	shard := c.GetShardForRepo(repoName)
	shardURL := c.getShardURL(shard)

	reqURL := fmt.Sprintf("%s/files/%s/raw?path=%s&ref=%s",
		shardURL,
		url.PathEscape(repoName),
		url.QueryEscape(path),
		url.QueryEscape(ref),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, 0, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("shard %d unreachable: %w", shard, err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, 0, fmt.Errorf("shard returned %d: %s", resp.StatusCode, string(body))
	}

	return resp.Body, resp.ContentLength, nil
}

// GetBranches proxies a branch listing request to the correct shard.
func (c *FederatedClient) GetBranches(ctx context.Context, repoName string) ([]string, error) {
	shard := c.GetShardForRepo(repoName)
	shardURL := c.getShardURL(shard)

	reqURL := fmt.Sprintf("%s/files/%s/branches", shardURL, url.PathEscape(repoName))

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
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("shard returned %d: %s", resp.StatusCode, string(body))
	}

	var branches []string
	if err := json.NewDecoder(resp.Body).Decode(&branches); err != nil {
		return nil, err
	}

	return branches, nil
}

// GetTags proxies a tag listing request to the correct shard.
func (c *FederatedClient) GetTags(ctx context.Context, repoName string) ([]string, error) {
	shard := c.GetShardForRepo(repoName)
	shardURL := c.getShardURL(shard)

	reqURL := fmt.Sprintf("%s/files/%s/tags", shardURL, url.PathEscape(repoName))

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
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("shard returned %d: %s", resp.StatusCode, string(body))
	}

	var tags []string
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, err
	}

	return tags, nil
}
