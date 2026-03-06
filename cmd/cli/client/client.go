package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// Client is the API client for code-search.
type Client struct {
	baseURL     string
	httpClient  *http.Client
	token       string
	authToken   string
	iapClientID string
	iapToken    string
}

// ClientOption is a functional option for Client.
type ClientOption func(*Client)

// WithAuthToken sets the authentication token (JWT or API token) for Bearer auth.
// Takes priority over IAP authentication when both are set.
func WithAuthToken(token string) ClientOption {
	return func(c *Client) {
		c.authToken = token
	}
}

// WithIAPClientID sets the IAP OAuth client ID for servers behind Google IAP.
// When set, the client will fetch a gcloud identity token and include it in requests.
func WithIAPClientID(clientID string) ClientOption {
	return func(c *Client) {
		c.iapClientID = clientID
	}
}

// New creates a new API client.
func New(baseURL, token string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}

	return c
}

// fetchIAPToken fetches an identity token from gcloud for IAP authentication.
func (c *Client) fetchIAPToken(ctx context.Context) (string, error) {
	if c.iapClientID == "" {
		return "", nil
	}
	// Check if we already have a token cached
	if c.iapToken != "" {
		return c.iapToken, nil
	}
	// Fetch identity token using gcloud
	cmd := exec.CommandContext(
		ctx,
		"gcloud",
		"auth",
		"print-identity-token",
		"--audiences="+c.iapClientID,
	)

	output, err := cmd.Output()
	if err != nil {
		exitErr := &exec.ExitError{}
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf(
				"gcloud auth print-identity-token failed: %s",
				string(exitErr.Stderr),
			)
		}

		return "", fmt.Errorf(
			"failed to run gcloud: %w (make sure gcloud is installed and you're logged in with 'gcloud auth login')",
			err,
		)
	}

	c.iapToken = strings.TrimSpace(string(output))

	return c.iapToken, nil
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

// SearchResult represents a single search result (matches API response).
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
	resp, err := c.post(ctx, "/api/v1/search", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// SearchStreamEvent represents an event from the search stream.
type SearchStreamEvent struct {
	Type       string        `json:"type"` // "result", "progress", "done", "error"
	Result     *SearchResult `json:"result,omitempty"`
	TotalCount int           `json:"total_count,omitempty"`
	Duration   string        `json:"duration,omitempty"`
	Truncated  bool          `json:"truncated,omitempty"`
	Progress   int           `json:"progress,omitempty"`
	Error      string        `json:"error,omitempty"`
}

// SearchStreamCallback is called for each event in the search stream.
type SearchStreamCallback func(event *SearchStreamEvent) error

// SearchStream performs a streaming code search and calls the callback for each event.
func (c *Client) SearchStream(
	ctx context.Context,
	req *SearchRequest,
	callback SearchStreamCallback,
) error {
	jsonBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/api/v1/search/stream",
		bytes.NewReader(jsonBody),
	)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	if err := c.setAuthHeaders(ctx, httpReq); err != nil {
		return err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	// Parse SSE stream
	return c.parseSSEStream(resp.Body, callback)
}

// parseSSEStream parses Server-Sent Events from a reader and calls the callback.
func (c *Client) parseSSEStream(r io.Reader, callback SearchStreamCallback) error {
	reader := NewLineReader(r)

	var eventData strings.Builder

	for {
		line, err := reader.ReadLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}

			return fmt.Errorf("read line: %w", err)
		}

		// Empty line signals end of event
		if line == "" {
			if eventData.Len() > 0 {
				var event SearchStreamEvent

				err := json.Unmarshal([]byte(eventData.String()), &event)
				if err != nil {
					return fmt.Errorf("parse event: %w", err)
				}

				cbErr := callback(&event)
				if cbErr != nil {
					return cbErr
				}

				// Stop on done or error events
				if event.Type == "done" || event.Type == "error" {
					return nil
				}

				eventData.Reset()
			}

			continue
		}

		// Parse SSE format: "data: {...}"
		if after, ok := strings.CutPrefix(line, "data: "); ok {
			eventData.WriteString(after)
		}
	}
}

// LineReader is a simple line reader for SSE parsing.
type LineReader struct {
	reader *strings.Reader
	buf    []byte
}

// NewLineReader creates a new LineReader.
func NewLineReader(r io.Reader) *LineReader {
	// Read all data first (for simplicity)
	data, _ := io.ReadAll(r)

	return &LineReader{
		reader: strings.NewReader(string(data)),
		buf:    make([]byte, 0, 4096),
	}
}

// ReadLine reads a single line from the reader.
func (lr *LineReader) ReadLine() (string, error) {
	lr.buf = lr.buf[:0]

	for {
		b := make([]byte, 1)

		n, err := lr.reader.Read(b)
		if err != nil {
			if len(lr.buf) > 0 {
				return string(lr.buf), nil
			}

			return "", err
		}

		if n == 0 {
			continue
		}

		if b[0] == '\n' {
			// Trim trailing \r if present
			line := string(lr.buf)
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}

			return line, nil
		}

		lr.buf = append(lr.buf, b[0])
	}
}

// FindSymbolsRequest represents a find symbols request.
type FindSymbolsRequest struct {
	Name     string   `json:"name"`
	Kind     string   `json:"kind,omitempty"`
	Language string   `json:"language,omitempty"`
	Repos    []string `json:"repos,omitempty"`
	Limit    int      `json:"limit,omitempty"`
}

// Symbol represents a code symbol.
type Symbol struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Repo      string `json:"repo"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	Signature string `json:"signature,omitempty"`
	Language  string `json:"language"`
}

// FindSymbols finds symbol definitions.
func (c *Client) FindSymbols(ctx context.Context, req *FindSymbolsRequest) ([]Symbol, error) {
	resp, err := c.post(ctx, "/api/v1/symbols/find", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result []Symbol
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result, nil
}

// FindRefsRequest represents a find references request.
type FindRefsRequest struct {
	Symbol   string   `json:"symbol"`
	Repos    []string `json:"repos,omitempty"`
	Language string   `json:"language,omitempty"`
	Limit    int      `json:"limit,omitempty"`
}

// Reference represents a symbol reference.
type Reference struct {
	Repo    string `json:"repo"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Context string `json:"context"`
}

// FindRefs finds references to a symbol.
func (c *Client) FindRefs(ctx context.Context, req *FindRefsRequest) ([]Reference, error) {
	resp, err := c.post(ctx, "/api/v1/symbols/refs", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result []Reference
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result, nil
}

// FindFilesRequest represents a find files request.
type FindFilesRequest struct {
	Pattern string `json:"pattern"`
	IsRegex bool   `json:"is_regex,omitempty"`
	Repo    string `json:"repo,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

// FileResult represents a file search result.
type FileResult struct {
	Repository string `json:"repository"`
	FilePath   string `json:"file_path"`
}

// FindFilesResponse represents the find files response.
type FindFilesResponse struct {
	Files []FileResult `json:"files"`
	Total int          `json:"total"`
}

// FindFiles finds files matching a pattern using the search API with file: filter.
func (c *Client) FindFiles(ctx context.Context, req *FindFilesRequest) (*FindFilesResponse, error) {
	// Build search query using file: filter
	query := "file:" + req.Pattern
	if req.Repo != "" {
		query = "repo:" + req.Repo + " " + query
	}

	searchReq := &SearchRequest{
		Query:   query,
		IsRegex: req.IsRegex,
		Limit:   req.Limit,
	}

	resp, err := c.Search(ctx, searchReq)
	if err != nil {
		return nil, err
	}

	// Extract unique files from search results
	seen := make(map[string]bool)

	var files []FileResult

	for _, r := range resp.Results {
		key := r.Repo + "/" + r.File
		if !seen[key] {
			seen[key] = true

			files = append(files, FileResult{
				Repository: r.Repo,
				FilePath:   r.File,
			})
		}
	}

	return &FindFilesResponse{
		Files: files,
		Total: len(files),
	}, nil
}

// ReplacePreviewRequest represents a request to preview replacements.
type ReplacePreviewRequest struct {
	SearchPattern string   `json:"search_pattern"`
	ReplaceWith   string   `json:"replace_with"`
	IsRegex       bool     `json:"is_regex"`
	CaseSensitive bool     `json:"case_sensitive"`
	FilePatterns  []string `json:"file_patterns,omitempty"`
	Repos         []string `json:"repos,omitempty"`
	Languages     []string `json:"languages,omitempty"`
	Limit         int      `json:"limit,omitempty"`
	ContextLines  int      `json:"context_lines,omitempty"`
}

// ReplacePreviewMatch represents a potential replacement.
type ReplacePreviewMatch struct {
	RepositoryID int64         `json:"repository_id"`
	Repo         string        `json:"repo"`
	File         string        `json:"file"`
	Line         int           `json:"line"`
	Column       int           `json:"column"`
	Content      string        `json:"content"`
	MatchStart   int           `json:"match_start"`
	MatchEnd     int           `json:"match_end"`
	Context      ResultContext `json:"context"`
}

// ReplacePreviewResponse represents the replace preview response.
type ReplacePreviewResponse struct {
	Matches    []ReplacePreviewMatch `json:"matches"`
	TotalCount int                   `json:"total_count"`
	Duration   string                `json:"duration"`
}

// ReplacePreview previews what would be replaced without making changes.
func (c *Client) ReplacePreview(
	ctx context.Context,
	req *ReplacePreviewRequest,
) (*ReplacePreviewResponse, error) {
	resp, err := c.post(ctx, "/api/v1/replace/preview", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ReplacePreviewResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// ReplaceMatch represents a pre-computed match from preview.
type ReplaceMatch struct {
	RepositoryID   int64  `json:"repository_id"`
	RepositoryName string `json:"repository_name"`
	FilePath       string `json:"file_path"`
}

// ReplaceExecuteRequest represents a request to execute replacements.
type ReplaceExecuteRequest struct {
	SearchPattern string   `json:"search_pattern"`
	ReplaceWith   string   `json:"replace_with"`
	IsRegex       bool     `json:"is_regex"`
	CaseSensitive bool     `json:"case_sensitive"`
	FilePatterns  []string `json:"file_patterns,omitempty"`
	// Matches from preview (required) - MR is always created
	Matches       []ReplaceMatch `json:"matches"`
	BranchName    string         `json:"branch_name,omitempty"`
	MRTitle       string         `json:"mr_title,omitempty"`
	MRDescription string         `json:"mr_description,omitempty"`
	// User-provided tokens for authentication (map of connection_id or "*" for all -> token)
	UserTokens map[string]string `json:"user_tokens,omitempty"`
}

// ReplaceExecuteResponse represents the replace execute response.
type ReplaceExecuteResponse struct {
	JobID   string `json:"job_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// ReplaceExecute executes replacements across repositories.
func (c *Client) ReplaceExecute(
	ctx context.Context,
	req *ReplaceExecuteRequest,
) (*ReplaceExecuteResponse, error) {
	resp, err := c.post(ctx, "/api/v1/replace/execute", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ReplaceExecuteResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// Repository represents a repository.
type Repository struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	CloneURL    string   `json:"clone_url"`
	Branches    []string `json:"branches"`
	Status      string   `json:"status"`
	LastIndexed string   `json:"last_indexed,omitempty"`
	Excluded    bool     `json:"excluded"`
	Deleted     bool     `json:"deleted"`
}

// ListRepositoriesResponse represents the list repositories response.
type ListRepositoriesResponse struct {
	Repos      []Repository `json:"repos"`
	TotalCount int          `json:"total_count"`
}

// ListRepositories lists all indexed repositories.
func (c *Client) ListRepositories(
	ctx context.Context,
	connectionID *int64,
) (*ListRepositoriesResponse, error) {
	path := "/api/v1/repos"
	if connectionID != nil {
		path += fmt.Sprintf("?connection_id=%d", *connectionID)
	}

	resp, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ListRepositoriesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// ReposStatus represents the repos status response.
type ReposStatus struct {
	ReadOnly bool `json:"readonly"`
}

// GetReposStatus gets the repos readonly status from the server.
func (c *Client) GetReposStatus(ctx context.Context) (*ReposStatus, error) {
	resp, err := c.get(ctx, "/api/v1/repos/status")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ReposStatus
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// GetRepository gets a specific repository by ID.
func (c *Client) GetRepository(ctx context.Context, id int64) (*Repository, error) {
	path := fmt.Sprintf("/api/v1/repos/by-id/%d", id)

	resp, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result Repository
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// AddRepositoryRequest represents a request to add a repository.
type AddRepositoryRequest struct {
	ConnectionID  int64    `json:"connection_id"`
	Name          string   `json:"name"`
	CloneURL      string   `json:"clone_url"`
	DefaultBranch string   `json:"default_branch"`
	Branches      []string `json:"branches,omitempty"`
}

// AddRepository adds a new repository.
func (c *Client) AddRepository(
	ctx context.Context,
	req *AddRepositoryRequest,
) (*Repository, error) {
	resp, err := c.post(ctx, "/api/v1/repos", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result Repository
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// SyncRepository triggers a sync for a repository by ID.
func (c *Client) SyncRepository(ctx context.Context, id int64) error {
	path := fmt.Sprintf("/api/v1/repos/by-id/%d/sync", id)

	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return err
	}

	resp.Body.Close()

	return nil
}

// SyncAllRepositories triggers a sync for all repositories via scheduler.
func (c *Client) SyncAllRepositories(ctx context.Context) error {
	resp, err := c.post(ctx, "/api/v1/scheduler/sync-all", nil)
	if err != nil {
		return err
	}

	resp.Body.Close()

	return nil
}

// RemoveRepository removes a repository by ID.
func (c *Client) RemoveRepository(ctx context.Context, id int64) error {
	path := fmt.Sprintf("/api/v1/repos/by-id/%d", id)

	resp, err := c.delete(ctx, path)
	if err != nil {
		return err
	}

	resp.Body.Close()

	return nil
}

// ExcludeRepository excludes a repository from sync and indexing.
func (c *Client) ExcludeRepository(ctx context.Context, id int64) error {
	path := fmt.Sprintf("/api/v1/repos/by-id/%d/exclude", id)

	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return err
	}

	resp.Body.Close()

	return nil
}

// IncludeRepository includes a previously excluded repository.
func (c *Client) IncludeRepository(ctx context.Context, id int64) error {
	path := fmt.Sprintf("/api/v1/repos/by-id/%d/include", id)

	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return err
	}

	resp.Body.Close()

	return nil
}

// RestoreRepository restores a previously deleted repository.
func (c *Client) RestoreRepository(ctx context.Context, id int64) error {
	path := fmt.Sprintf("/api/v1/repos/by-id/%d/restore", id)

	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return err
	}

	resp.Body.Close()

	return nil
}

// setAuthHeaders sets authorization headers.
// Priority: authToken (JWT/API token) > IAP token.
// Note: The --token flag is for code host authentication (GitHub/GitLab PAT),
// which is passed in request bodies (UserTokens), not in HTTP headers.
func (c *Client) setAuthHeaders(ctx context.Context, req *http.Request) error {
	// Auth token takes priority (enterprise OIDC session or API token)
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
		return nil
	}

	// Fall back to IAP token if configured (for bypassing GCP IAP proxy)
	if c.iapClientID != "" {
		iapToken, err := c.fetchIAPToken(ctx)
		if err != nil {
			return fmt.Errorf("fetch IAP token: %w", err)
		}

		if iapToken != "" {
			req.Header.Set("Authorization", "Bearer "+iapToken)
		}
	}
	// Note: c.token is NOT set in headers - it's passed in request bodies
	// for code host authentication (UserTokens in replace requests)
	return nil
}

// get performs a GET request.
func (c *Client) get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if err := c.setAuthHeaders(ctx, req); err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	return resp, nil
}

// post performs a POST request.
func (c *Client) post(ctx context.Context, path string, body any) (*http.Response, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+path,
		bytes.NewReader(jsonBody),
	)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	if err := c.setAuthHeaders(ctx, req); err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	return resp, nil
}

// delete performs a DELETE request.
func (c *Client) delete(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if err := c.setAuthHeaders(ctx, req); err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	return resp, nil
}
