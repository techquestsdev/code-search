package codehost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/techquestsdev/code-search/internal/metrics"
	"github.com/techquestsdev/code-search/internal/tracing"
)

// GiteaClient implements Client for Gitea.
type GiteaClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewGiteaClient creates a new Gitea client.
func NewGiteaClient(baseURL, token string) *GiteaClient {
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &GiteaClient{
		baseURL:    baseURL + "/api/v1",
		token:      token,
		httpClient: sharedHTTPClient,
	}
}

func (c *GiteaClient) doRequest(
	ctx context.Context,
	method, path string,
	body io.Reader,
) (*http.Response, error) {
	ctx, span := tracing.StartSpan(ctx, "codehost.gitea.request")
	defer span.End()

	url := c.baseURL + path

	tracing.SetAttributes(ctx,
		tracing.AttrHostType.String("gitea"),
		attribute.String("http.method", method),
		attribute.String("http.url", url),
	)

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		tracing.RecordError(ctx, err)
		return nil, err
	}

	if c.token != "" {
		req.Header.Set("Authorization", "token "+c.token)
	}

	req.Header.Set("Accept", "application/json")

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		metrics.RecordCodeHostRequest("gitea", path, duration, false)
		metrics.RecordError("codehost", "request_failed")
		tracing.RecordError(ctx, err)

		return nil, err
	}

	// Record metrics
	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	metrics.RecordCodeHostRequest("gitea", path, duration, success)

	// Extract and record rate limit info from headers (Gitea uses standard headers)
	if remaining := resp.Header.Get("X-Ratelimit-Remaining"); remaining != "" {
		if r, err := strconv.Atoi(remaining); err == nil {
			resetStr := resp.Header.Get("X-Ratelimit-Reset")

			resetTime := time.Now().Add(time.Hour) // Default
			if reset, err := strconv.ParseInt(resetStr, 10, 64); err == nil {
				resetTime = time.Unix(reset, 0)
			}

			metrics.SetCodeHostRateLimit("gitea", r, resetTime)
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
func (c *GiteaClient) ValidateCredentials(ctx context.Context) error {
	// No token means unauthenticated access - skip validation
	if c.token == "" {
		return nil
	}

	resp, err := c.doRequest(ctx, "GET", "/user", nil)
	if err != nil {
		return fmt.Errorf("gitea request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return errors.New("invalid credentials: unauthorized")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gitea error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ListRepositories lists all accessible repositories.
func (c *GiteaClient) ListRepositories(ctx context.Context) ([]Repository, error) {
	var allRepos []Repository

	page := 1
	limit := 50

	for {
		path := fmt.Sprintf("/user/repos?page=%d&limit=%d", page, limit)

		resp, err := c.doRequest(ctx, "GET", path, nil)
		if err != nil {
			return nil, fmt.Errorf("gitea request: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			return nil, fmt.Errorf("gitea error %d: %s", resp.StatusCode, string(body))
		}

		var giteaRepos []struct {
			Name          string `json:"name"`
			FullName      string `json:"full_name"`
			CloneURL      string `json:"clone_url"`
			SSHURL        string `json:"ssh_url"`
			DefaultBranch string `json:"default_branch"`
			Private       bool   `json:"private"`
			Archived      bool   `json:"archived"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&giteaRepos); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decode response: %w", err)
		}

		resp.Body.Close()

		if len(giteaRepos) == 0 {
			break
		}

		for _, r := range giteaRepos {
			allRepos = append(allRepos, Repository{
				Name:          r.Name,
				FullName:      r.FullName,
				CloneURL:      r.CloneURL,
				DefaultBranch: r.DefaultBranch,
				Private:       r.Private,
				Archived:      r.Archived,
			})
		}

		if len(giteaRepos) < limit {
			break
		}

		page++
	}

	return allRepos, nil
}

// GetRepository gets a specific repository.
func (c *GiteaClient) GetRepository(ctx context.Context, name string) (*Repository, error) {
	path := "/repos/" + name

	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("gitea request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gitea error %d: %s", resp.StatusCode, string(body))
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

// CreateMergeRequest creates a pull request on Gitea.
func (c *GiteaClient) CreateMergeRequest(
	ctx context.Context,
	repo, title, description, sourceBranch, targetBranch string,
) (*MergeRequest, error) {
	path := fmt.Sprintf("/repos/%s/pulls", repo)
	payload := fmt.Sprintf(`{"title":%q,"body":%q,"head":%q,"base":%q}`,
		title, description, sourceBranch, targetBranch)

	resp, err := c.doRequest(ctx, "POST", path, strings.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("gitea request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gitea error %d: %s", resp.StatusCode, string(body))
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
func (c *GiteaClient) GetCloneURL(repo string) string {
	baseURL := strings.TrimSuffix(c.baseURL, "/api/v1")
	return fmt.Sprintf("%s/%s.git", baseURL, repo)
}
