package codehost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/aanogueira/code-search/internal/metrics"
	"github.com/aanogueira/code-search/internal/tracing"
)

// sharedTransport is a shared HTTP transport with connection pooling.
// This allows all code host clients to share connections efficiently.
var sharedTransport = &http.Transport{
	// Connection pool settings
	MaxIdleConns:        100,              // Total max idle connections across all hosts
	MaxIdleConnsPerHost: 10,               // Max idle connections per host
	MaxConnsPerHost:     20,               // Max total connections per host
	IdleConnTimeout:     90 * time.Second, // How long to keep idle connections

	// Connection timeouts
	DialContext: (&net.Dialer{
		Timeout:   10 * time.Second, // Connection timeout
		KeepAlive: 30 * time.Second, // TCP keep-alive interval
	}).DialContext,
	TLSHandshakeTimeout:   10 * time.Second,
	ResponseHeaderTimeout: 30 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,

	// Enable HTTP/2 if available
	ForceAttemptHTTP2: true,
}

// DefaultHTTPTimeout is the default timeout for HTTP requests to code hosts.
const DefaultHTTPTimeout = 30 * time.Second

// sharedHTTPClient is the shared HTTP client used by all code host clients.
var sharedHTTPClient = &http.Client{
	Timeout:   DefaultHTTPTimeout,
	Transport: sharedTransport,
}

// Repository represents a repository from a code host.
type Repository struct {
	Name          string
	FullName      string
	CloneURL      string
	DefaultBranch string
	Private       bool
	Archived      bool
}

// MergeRequest represents a merge/pull request.
type MergeRequest struct {
	ID        int
	Number    int
	Title     string
	URL       string
	State     string
	CreatedAt time.Time
}

// Client interface for code host operations.
type Client interface {
	ValidateCredentials(ctx context.Context) error
	ListRepositories(ctx context.Context) ([]Repository, error)
	GetRepository(ctx context.Context, name string) (*Repository, error)
	CreateMergeRequest(
		ctx context.Context,
		repo, title, description, sourceBranch, targetBranch string,
	) (*MergeRequest, error)
	GetCloneURL(repo string) string
}

// GitHubClient implements Client for GitHub.
type GitHubClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewGitHubClient creates a new GitHub client.
func NewGitHubClient(token string) *GitHubClient {
	return &GitHubClient{
		baseURL:    "https://api.github.com",
		token:      token,
		httpClient: sharedHTTPClient,
	}
}

// NewGitHubEnterpriseClient creates a GitHub Enterprise client.
func NewGitHubEnterpriseClient(baseURL, token string) *GitHubClient {
	// Ensure proper URL format
	baseURL = strings.TrimSuffix(baseURL, "/")
	if !strings.HasSuffix(baseURL, "/api/v3") {
		baseURL = baseURL + "/api/v3"
	}

	return &GitHubClient{
		baseURL:    baseURL,
		token:      token,
		httpClient: sharedHTTPClient,
	}
}

func (c *GitHubClient) doRequest(
	ctx context.Context,
	method, path string,
	body io.Reader,
) (*http.Response, error) {
	ctx, span := tracing.StartSpan(ctx, "codehost.github.request")
	defer span.End()

	url := c.baseURL + path

	tracing.SetAttributes(ctx,
		tracing.AttrHostType.String("github"),
		attribute.String("http.method", method),
		attribute.String("http.url", url),
	)

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		tracing.RecordError(ctx, err)
		return nil, err
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	req.Header.Set("Accept", "application/vnd.github+json")

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		metrics.RecordCodeHostRequest("github", path, duration, false)
		metrics.RecordError("codehost", "request_failed")
		tracing.RecordError(ctx, err)

		return nil, err
	}

	// Record metrics
	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	metrics.RecordCodeHostRequest("github", path, duration, success)

	// Extract and record rate limit info from headers
	if remaining := resp.Header.Get("X-Ratelimit-Remaining"); remaining != "" {
		if r, err := strconv.Atoi(remaining); err == nil {
			resetStr := resp.Header.Get("X-Ratelimit-Reset")

			resetTime := time.Now().Add(time.Hour) // Default
			if reset, err := strconv.ParseInt(resetStr, 10, 64); err == nil {
				resetTime = time.Unix(reset, 0)
			}

			metrics.SetCodeHostRateLimit("github", r, resetTime)
		}
	}

	tracing.SetAttributes(ctx, attribute.Int("http.status_code", resp.StatusCode))

	if success {
		tracing.SetOK(ctx)
	}

	return resp, nil
}

// ValidateCredentials checks if the token is valid by fetching the current user.
// If no token is provided, skips validation (for public repo access).
func (c *GitHubClient) ValidateCredentials(ctx context.Context) error {
	// No token means unauthenticated access - skip validation
	// Public repos can be accessed without auth
	if c.token == "" {
		return nil
	}

	resp, err := c.doRequest(ctx, "GET", "/user", nil)
	if err != nil {
		return fmt.Errorf("github request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return errors.New("invalid credentials: unauthorized")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ListRepositories lists all accessible repositories.
func (c *GitHubClient) ListRepositories(ctx context.Context) ([]Repository, error) {
	var allRepos []Repository

	page := 1

	for {
		path := fmt.Sprintf("/user/repos?per_page=100&page=%d", page)

		resp, err := c.doRequest(ctx, "GET", path, nil)
		if err != nil {
			return nil, fmt.Errorf("github request: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			return nil, fmt.Errorf("github error %d: %s", resp.StatusCode, string(body))
		}

		var ghRepos []struct {
			Name          string `json:"name"`
			FullName      string `json:"full_name"`
			CloneURL      string `json:"clone_url"`
			SSHURL        string `json:"ssh_url"`
			DefaultBranch string `json:"default_branch"`
			Private       bool   `json:"private"`
			Archived      bool   `json:"archived"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&ghRepos); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decode response: %w", err)
		}

		resp.Body.Close()

		if len(ghRepos) == 0 {
			break
		}

		for _, r := range ghRepos {
			allRepos = append(allRepos, Repository{
				Name:          r.Name,
				FullName:      r.FullName,
				CloneURL:      r.CloneURL,
				DefaultBranch: r.DefaultBranch,
				Private:       r.Private,
				Archived:      r.Archived,
			})
		}

		page++
	}

	return allRepos, nil
}

// GetRepository gets a specific repository.
func (c *GitHubClient) GetRepository(ctx context.Context, name string) (*Repository, error) {
	path := "/repos/" + name

	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("github request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github error %d: %s", resp.StatusCode, string(body))
	}

	var r struct {
		Name          string `json:"name"`
		FullName      string `json:"full_name"`
		CloneURL      string `json:"clone_url"`
		DefaultBranch string `json:"default_branch"`
		Private       bool   `json:"private"`
		Archived      bool   `json:"archived"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &Repository{
		Name:          r.Name,
		FullName:      r.FullName,
		CloneURL:      r.CloneURL,
		DefaultBranch: r.DefaultBranch,
		Private:       r.Private,
		Archived:      r.Archived,
	}, nil
}

// CreateMergeRequest creates a pull request on GitHub.
func (c *GitHubClient) CreateMergeRequest(
	ctx context.Context,
	repo, title, description, sourceBranch, targetBranch string,
) (*MergeRequest, error) {
	path := fmt.Sprintf("/repos/%s/pulls", repo)
	payload := fmt.Sprintf(`{"title":%q,"body":%q,"head":%q,"base":%q}`,
		title, description, sourceBranch, targetBranch)

	resp, err := c.doRequest(ctx, "POST", path, strings.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("github request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github error %d: %s", resp.StatusCode, string(body))
	}

	var pr struct {
		ID        int       `json:"id"`
		Number    int       `json:"number"`
		Title     string    `json:"title"`
		HTMLURL   string    `json:"html_url"`
		State     string    `json:"state"`
		CreatedAt time.Time `json:"created_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &MergeRequest{
		ID:        pr.ID,
		Number:    pr.Number,
		Title:     pr.Title,
		URL:       pr.HTMLURL,
		State:     pr.State,
		CreatedAt: pr.CreatedAt,
	}, nil
}

// GetCloneURL returns the clone URL for a repository.
func (c *GitHubClient) GetCloneURL(repo string) string {
	return fmt.Sprintf("https://github.com/%s.git", repo)
}

// GitLabClient implements Client for GitLab.
type GitLabClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewGitLabClient creates a new GitLab client.
func NewGitLabClient(baseURL, token string) *GitLabClient {
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &GitLabClient{
		baseURL:    baseURL + "/api/v4",
		token:      token,
		httpClient: sharedHTTPClient,
	}
}

func (c *GitLabClient) doRequest(
	ctx context.Context,
	method, path string,
	body io.Reader,
) (*http.Response, error) {
	ctx, span := tracing.StartSpan(ctx, "codehost.gitlab.request")
	defer span.End()

	url := c.baseURL + path

	tracing.SetAttributes(ctx,
		tracing.AttrHostType.String("gitlab"),
		attribute.String("http.method", method),
		attribute.String("http.url", url),
	)

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		tracing.RecordError(ctx, err)
		return nil, err
	}

	if c.token != "" {
		req.Header.Set("Private-Token", c.token)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		metrics.RecordCodeHostRequest("gitlab", path, duration, false)
		metrics.RecordError("codehost", "request_failed")
		tracing.RecordError(ctx, err)

		return nil, err
	}

	// Record metrics
	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	metrics.RecordCodeHostRequest("gitlab", path, duration, success)

	// Extract and record rate limit info from headers
	if remaining := resp.Header.Get("Ratelimit-Remaining"); remaining != "" {
		if r, err := strconv.Atoi(remaining); err == nil {
			resetStr := resp.Header.Get("Ratelimit-Reset")

			resetTime := time.Now().Add(time.Hour) // Default
			if reset, err := strconv.ParseInt(resetStr, 10, 64); err == nil {
				resetTime = time.Unix(reset, 0)
			}

			metrics.SetCodeHostRateLimit("gitlab", r, resetTime)
		}
	}

	tracing.SetAttributes(ctx, attribute.Int("http.status_code", resp.StatusCode))

	if success {
		tracing.SetOK(ctx)
	}

	return resp, nil
}

// ValidateCredentials checks if the token is valid by fetching the current user.
// If no token is provided, skips validation (for public repo access).
func (c *GitLabClient) ValidateCredentials(ctx context.Context) error {
	// No token means unauthenticated access - skip validation
	// Public repos can be accessed without auth
	if c.token == "" {
		return nil
	}

	resp, err := c.doRequest(ctx, "GET", "/user", nil)
	if err != nil {
		return fmt.Errorf("gitlab request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return errors.New("invalid credentials: unauthorized")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gitlab error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ListRepositories lists all accessible projects.
func (c *GitLabClient) ListRepositories(ctx context.Context) ([]Repository, error) {
	var allRepos []Repository

	page := 1

	for {
		path := fmt.Sprintf("/projects?membership=true&per_page=100&page=%d", page)

		resp, err := c.doRequest(ctx, "GET", path, nil)
		if err != nil {
			return nil, fmt.Errorf("gitlab request: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			return nil, fmt.Errorf("gitlab error %d: %s", resp.StatusCode, string(body))
		}

		var glProjects []struct {
			ID                int    `json:"id"`
			Name              string `json:"name"`
			PathWithNamespace string `json:"path_with_namespace"`
			HTTPURLToRepo     string `json:"http_url_to_repo"`
			DefaultBranch     string `json:"default_branch"`
			Visibility        string `json:"visibility"`
			Archived          bool   `json:"archived"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&glProjects); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decode response: %w", err)
		}

		resp.Body.Close()

		if len(glProjects) == 0 {
			break
		}

		for _, p := range glProjects {
			allRepos = append(allRepos, Repository{
				Name:          p.Name,
				FullName:      p.PathWithNamespace,
				CloneURL:      p.HTTPURLToRepo,
				DefaultBranch: p.DefaultBranch,
				Private:       p.Visibility != "public",
				Archived:      p.Archived,
			})
		}

		page++
	}

	return allRepos, nil
}

// GetRepository gets a specific project.
func (c *GitLabClient) GetRepository(ctx context.Context, name string) (*Repository, error) {
	// URL encode the project path
	encodedName := strings.ReplaceAll(name, "/", "%2F")
	path := "/projects/" + encodedName

	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("gitlab request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gitlab error %d: %s", resp.StatusCode, string(body))
	}

	var p struct {
		Name              string `json:"name"`
		PathWithNamespace string `json:"path_with_namespace"`
		HTTPURLToRepo     string `json:"http_url_to_repo"`
		DefaultBranch     string `json:"default_branch"`
		Visibility        string `json:"visibility"`
		Archived          bool   `json:"archived"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &Repository{
		Name:          p.Name,
		FullName:      p.PathWithNamespace,
		CloneURL:      p.HTTPURLToRepo,
		DefaultBranch: p.DefaultBranch,
		Private:       p.Visibility != "public",
		Archived:      p.Archived,
	}, nil
}

// CreateMergeRequest creates a merge request on GitLab.
func (c *GitLabClient) CreateMergeRequest(
	ctx context.Context,
	repo, title, description, sourceBranch, targetBranch string,
) (*MergeRequest, error) {
	encodedRepo := strings.ReplaceAll(repo, "/", "%2F")
	path := fmt.Sprintf("/projects/%s/merge_requests", encodedRepo)
	payload := fmt.Sprintf(`{"title":%q,"description":%q,"source_branch":%q,"target_branch":%q}`,
		title, description, sourceBranch, targetBranch)

	resp, err := c.doRequest(ctx, "POST", path, strings.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("gitlab request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gitlab error %d: %s", resp.StatusCode, string(body))
	}

	var mr struct {
		ID        int       `json:"id"`
		IID       int       `json:"iid"`
		Title     string    `json:"title"`
		WebURL    string    `json:"web_url"`
		State     string    `json:"state"`
		CreatedAt time.Time `json:"created_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&mr); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &MergeRequest{
		ID:        mr.ID,
		Number:    mr.IID,
		Title:     mr.Title,
		URL:       mr.WebURL,
		State:     mr.State,
		CreatedAt: mr.CreatedAt,
	}, nil
}

// GetCloneURL returns the clone URL for a repository.
func (c *GitLabClient) GetCloneURL(repo string) string {
	baseURL := strings.TrimSuffix(c.baseURL, "/api/v4")
	return fmt.Sprintf("%s/%s.git", baseURL, repo)
}

// NewClient creates a code host client based on type.
func NewClient(hostType, baseURL, token string) (Client, error) {
	switch hostType {
	case "github":
		return NewGitHubClient(token), nil
	case "github_enterprise":
		return NewGitHubEnterpriseClient(baseURL, token), nil
	case "gitlab":
		return NewGitLabClient(baseURL, token), nil
	case "bitbucket":
		return NewBitbucketClient(token), nil
	case "bitbucket_server":
		return NewBitbucketServerClient(baseURL, token), nil
	case "gitea":
		return NewGiteaClient(baseURL, token), nil
	default:
		return nil, fmt.Errorf("unsupported code host type: %s", hostType)
	}
}
