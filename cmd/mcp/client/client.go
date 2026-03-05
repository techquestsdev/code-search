package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client is an HTTP client for the Code Search API.
type Client struct {
	baseURL    string
	authToken  string
	httpClient *http.Client
}

// New creates a new API client. If authToken is non-empty, requests will
// include an Authorization: Bearer header.
func New(baseURL, authToken string) *Client {
	return &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		authToken: authToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// setAuthHeaders sets the Authorization header if a token is configured.
func (c *Client) setAuthHeaders(req *http.Request) {
	token := strings.TrimSpace(c.authToken)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

// get performs a GET request and returns the response body.
func (c *Client) get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	c.setAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connect to Code Search API at %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	return body, nil
}

// post performs a POST request and returns the response body.
func (c *Client) post(ctx context.Context, path string, payload any) ([]byte, error) {
	var bodyReader io.Reader
	if payload != nil {
		jsonBody, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.setAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connect to Code Search API at %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	return body, nil
}

// APIError represents an API error response.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error (%d): %s", e.StatusCode, e.Body)
}

// IsNotFound returns true if the error is a 404.
func (e *APIError) IsNotFound() bool {
	return e.StatusCode == 404
}

// SearchRequest represents a search request.
type SearchRequest struct {
	Query         string   `json:"query"`
	IsRegex       bool     `json:"is_regex,omitempty"`
	CaseSensitive bool     `json:"case_sensitive,omitempty"`
	Repos         []string `json:"repos,omitempty"`
	Languages     []string `json:"languages,omitempty"`
	FilePatterns  []string `json:"file_patterns,omitempty"`
	Limit         int      `json:"limit,omitempty"`
	ContextLines  int      `json:"context_lines,omitempty"`
}

// SearchResult represents a single search result.
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

// SearchResponse represents the search API response.
type SearchResponse struct {
	Results    []SearchResult `json:"results"`
	TotalCount int            `json:"total_count"`
	Truncated  bool           `json:"truncated"`
	Duration   string         `json:"duration"`
}

// Search performs a code search.
func (c *Client) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	data, err := c.post(ctx, "/api/v1/search", req)
	if err != nil {
		return nil, err
	}

	var result SearchResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// Repository represents a repository.
type Repository struct {
	ID            int64    `json:"id"`
	Name          string   `json:"name"`
	CloneURL      string   `json:"clone_url"`
	DefaultBranch string   `json:"default_branch"`
	Branches      []string `json:"branches"`
	Status        string   `json:"status"`
	LastIndexed   string   `json:"last_indexed,omitempty"`
	Excluded      bool     `json:"excluded"`
	Deleted       bool     `json:"deleted"`
}

// ListReposResponse represents the list repos response.
type ListReposResponse struct {
	Repos      []Repository `json:"repos"`
	TotalCount int          `json:"total_count"`
	Limit      int          `json:"limit"`
	Offset     int          `json:"offset"`
	HasMore    bool         `json:"has_more"`
}

// ListReposParams contains parameters for listing repos.
type ListReposParams struct {
	Search string
	Status string
	Limit  int
	Offset int
}

// ListRepos lists indexed repositories.
func (c *Client) ListRepos(ctx context.Context, params ListReposParams) (*ListReposResponse, error) {
	q := url.Values{}
	if params.Search != "" {
		q.Set("search", params.Search)
	}
	if params.Status != "" {
		q.Set("status", params.Status)
	}
	if params.Limit > 0 {
		q.Set("limit", strconv.Itoa(params.Limit))
	}
	if params.Offset > 0 {
		q.Set("offset", strconv.Itoa(params.Offset))
	}

	path := "/api/v1/repos"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}

	data, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}

	var result ListReposResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// TreeEntry represents a file or directory in a tree listing.
type TreeEntry struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Path     string `json:"path"`
	Size     int64  `json:"size,omitempty"`
	Language string `json:"language,omitempty"`
}

// TreeResponse represents the tree API response.
type TreeResponse struct {
	Entries []TreeEntry `json:"entries"`
	Path    string      `json:"path"`
	Ref     string      `json:"ref"`
}

// GetFileTree returns directory contents for a repository.
func (c *Client) GetFileTree(ctx context.Context, repoID int64, path, ref string) (*TreeResponse, error) {
	q := url.Values{}
	if path != "" {
		q.Set("path", path)
	}
	if ref != "" {
		q.Set("ref", ref)
	}

	reqPath := fmt.Sprintf("/api/v1/repos/by-id/%d/tree", repoID)
	if len(q) > 0 {
		reqPath += "?" + q.Encode()
	}

	data, err := c.get(ctx, reqPath)
	if err != nil {
		return nil, err
	}

	var result TreeResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// BlobResponse represents file content.
type BlobResponse struct {
	Content  string `json:"content"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Language string `json:"language"`
	Binary   bool   `json:"binary"`
	Ref      string `json:"ref"`
}

// GetFileContent returns the content of a file.
func (c *Client) GetFileContent(ctx context.Context, repoID int64, path, ref string) (*BlobResponse, error) {
	q := url.Values{}
	q.Set("path", path)
	if ref != "" {
		q.Set("ref", ref)
	}

	reqPath := fmt.Sprintf("/api/v1/repos/by-id/%d/blob?%s", repoID, q.Encode())

	data, err := c.get(ctx, reqPath)
	if err != nil {
		return nil, err
	}

	var result BlobResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// RefsResponse represents branches and tags.
type RefsResponse struct {
	Branches      []string `json:"branches"`
	Tags          []string `json:"tags"`
	DefaultBranch string   `json:"default_branch"`
}

// GetBranchesAndTags returns branches and tags for a repository.
func (c *Client) GetBranchesAndTags(ctx context.Context, repoID int64) (*RefsResponse, error) {
	path := fmt.Sprintf("/api/v1/repos/by-id/%d/refs", repoID)

	data, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}

	var result RefsResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// SymbolSearchResult represents a symbol search result.
type SymbolSearchResult struct {
	Symbol        string `json:"symbol"`
	DisplayName   string `json:"displayName,omitempty"`
	FilePath      string `json:"filePath"`
	Line          int    `json:"line"`
	Documentation string `json:"documentation,omitempty"`
}

// SearchSymbolsResponse represents the search symbols API response.
type SearchSymbolsResponse struct {
	Results []SymbolSearchResult `json:"results"`
	Count   int                  `json:"count"`
}

// SearchSymbols searches for symbols in a repository.
func (c *Client) SearchSymbols(ctx context.Context, repoID int64, query string, limit int) (*SearchSymbolsResponse, error) {
	body := map[string]any{
		"query": query,
	}
	if limit > 0 {
		body["limit"] = limit
	}

	path := fmt.Sprintf("/api/v1/scip/repos/%d/symbols/search", repoID)

	data, err := c.post(ctx, path, body)
	if err != nil {
		return nil, err
	}

	var result SearchSymbolsResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// Occurrence represents a SCIP occurrence.
type Occurrence struct {
	Symbol    string `json:"symbol"`
	FilePath  string `json:"filePath"`
	StartLine int    `json:"startLine"`
	StartCol  int    `json:"startCol"`
	EndLine   int    `json:"endLine"`
	EndCol    int    `json:"endCol"`
	Context   string `json:"context,omitempty"`
}

// SymbolInfo contains metadata about a symbol.
type SymbolInfo struct {
	Symbol        string `json:"symbol"`
	Documentation string `json:"documentation,omitempty"`
	DisplayName   string `json:"displayName,omitempty"`
}

// GoToDefinitionResponse represents the go-to-definition API response.
type GoToDefinitionResponse struct {
	Found      bool        `json:"found"`
	Symbol     string      `json:"symbol,omitempty"`
	Definition *Occurrence `json:"definition,omitempty"`
	Info       *SymbolInfo `json:"info,omitempty"`
	External   bool        `json:"external"`
}

// GoToDefinition finds the definition of a symbol.
func (c *Client) GoToDefinition(ctx context.Context, repoID int64, file string, line, column int) (*GoToDefinitionResponse, error) {
	body := map[string]any{
		"filePath": file,
		"line":     line,
		"column":   column,
	}

	path := fmt.Sprintf("/api/v1/scip/repos/%d/definition", repoID)

	data, err := c.post(ctx, path, body)
	if err != nil {
		return nil, err
	}

	var result GoToDefinitionResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// FindReferencesResponse represents the find-references API response.
type FindReferencesResponse struct {
	Found      bool         `json:"found"`
	Symbol     string       `json:"symbol,omitempty"`
	Definition *Occurrence  `json:"definition,omitempty"`
	References []Occurrence `json:"references"`
	TotalCount int          `json:"totalCount"`
}

// FindReferences finds all references to a symbol.
func (c *Client) FindReferences(ctx context.Context, repoID int64, file string, line, column, limit int) (*FindReferencesResponse, error) {
	body := map[string]any{
		"filePath": file,
		"line":     line,
		"column":   column,
	}
	if limit > 0 {
		body["limit"] = limit
	}

	path := fmt.Sprintf("/api/v1/scip/repos/%d/references", repoID)

	data, err := c.post(ctx, path, body)
	if err != nil {
		return nil, err
	}

	var result FindReferencesResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}
