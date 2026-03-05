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

	"github.com/aanogueira/code-search/internal/metrics"
	"github.com/aanogueira/code-search/internal/tracing"
)

// BitbucketClient implements Client for Bitbucket Cloud.
type BitbucketClient struct {
	baseURL     string
	username    string
	appPassword string
	httpClient  *http.Client
}

// NewBitbucketClient creates a new Bitbucket Cloud client.
// token should be in format "username:app_password" or just the app password.
func NewBitbucketClient(token string) *BitbucketClient {
	username := ""
	appPassword := token

	// Check if token is in "username:app_password" format
	if parts := strings.SplitN(token, ":", 2); len(parts) == 2 {
		username = parts[0]
		appPassword = parts[1]
	}

	return &BitbucketClient{
		baseURL:     "https://api.bitbucket.org/2.0",
		username:    username,
		appPassword: appPassword,
		httpClient:  sharedHTTPClient,
	}
}

// NewBitbucketServerClient creates a Bitbucket Server (self-hosted) client.
func NewBitbucketServerClient(baseURL, token string) *BitbucketClient {
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &BitbucketClient{
		baseURL:     baseURL + "/rest/api/1.0",
		appPassword: token,
		httpClient:  sharedHTTPClient,
	}
}

func (c *BitbucketClient) doRequest(
	ctx context.Context,
	method, path string,
	body io.Reader,
) (*http.Response, error) {
	ctx, span := tracing.StartSpan(ctx, "codehost.bitbucket.request")
	defer span.End()

	url := c.baseURL + path

	tracing.SetAttributes(ctx,
		tracing.AttrHostType.String("bitbucket"),
		attribute.String("http.method", method),
		attribute.String("http.url", url),
	)

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		tracing.RecordError(ctx, err)
		return nil, err
	}

	// Bitbucket Cloud uses Basic Auth with app passwords
	if c.username != "" && c.appPassword != "" {
		req.SetBasicAuth(c.username, c.appPassword)
	} else if c.appPassword != "" {
		req.Header.Set("Authorization", "Bearer "+c.appPassword)
	}

	req.Header.Set("Accept", "application/json")

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		metrics.RecordCodeHostRequest("bitbucket", path, duration, false)
		metrics.RecordError("codehost", "request_failed")
		tracing.RecordError(ctx, err)

		return nil, err
	}

	// Record metrics
	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	metrics.RecordCodeHostRequest("bitbucket", path, duration, success)

	// Extract and record rate limit info from headers (Bitbucket uses standard headers)
	if remaining := resp.Header.Get("X-Ratelimit-Remaining"); remaining != "" {
		if r, err := strconv.Atoi(remaining); err == nil {
			resetStr := resp.Header.Get("X-Ratelimit-Reset")

			resetTime := time.Now().Add(time.Hour) // Default
			if reset, err := strconv.ParseInt(resetStr, 10, 64); err == nil {
				resetTime = time.Unix(reset, 0)
			}

			metrics.SetCodeHostRateLimit("bitbucket", r, resetTime)
		}
	}

	tracing.SetAttributes(ctx, attribute.Int("http.status_code", resp.StatusCode))

	if success {
		tracing.SetOK(ctx)
	}

	return resp, nil
}

// ValidateCredentials checks if the credentials are valid.
// If no credentials provided, skips validation (for public repo access).
func (c *BitbucketClient) ValidateCredentials(ctx context.Context) error {
	// No credentials means unauthenticated access - skip validation
	if c.appPassword == "" {
		return nil
	}

	resp, err := c.doRequest(ctx, "GET", "/user", nil)
	if err != nil {
		return fmt.Errorf("bitbucket request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return errors.New("invalid credentials: unauthorized")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("bitbucket error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ListRepositories lists all accessible repositories.
func (c *BitbucketClient) ListRepositories(ctx context.Context) ([]Repository, error) {
	var allRepos []Repository

	// First, get the current user to find workspaces
	resp, err := c.doRequest(ctx, "GET", "/user", nil)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	defer resp.Body.Close()

	var user struct {
		Username string `json:"username"`
		UUID     string `json:"uuid"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("decode user: %w", err)
	}

	// List repositories the user has access to
	nextURL := fmt.Sprintf("/repositories/%s?pagelen=100", user.Username)

	for nextURL != "" {
		// Handle pagination - next might be full URL
		path := nextURL
		if strings.HasPrefix(nextURL, "https://") {
			path = strings.TrimPrefix(nextURL, c.baseURL)
		}

		resp, err := c.doRequest(ctx, "GET", path, nil)
		if err != nil {
			return nil, fmt.Errorf("list repos: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			return nil, fmt.Errorf("bitbucket error %d: %s", resp.StatusCode, string(body))
		}

		var page struct {
			Values []struct {
				Name     string `json:"name"`
				FullName string `json:"full_name"`
				Links    struct {
					Clone []struct {
						Name string `json:"name"`
						Href string `json:"href"`
					} `json:"clone"`
				} `json:"links"`
				Mainbranch struct {
					Name string `json:"name"`
				} `json:"mainbranch"`
				IsPrivate bool `json:"is_private"`
			} `json:"values"`
			Next string `json:"next"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decode response: %w", err)
		}

		resp.Body.Close()

		for _, r := range page.Values {
			cloneURL := ""

			for _, link := range r.Links.Clone {
				if link.Name == "https" {
					cloneURL = link.Href
					break
				}
			}

			defaultBranch := r.Mainbranch.Name
			if defaultBranch == "" {
				defaultBranch = "main"
			}

			allRepos = append(allRepos, Repository{
				Name:          r.Name,
				FullName:      r.FullName,
				CloneURL:      cloneURL,
				DefaultBranch: defaultBranch,
				Private:       r.IsPrivate,
				Archived:      false, // Bitbucket doesn't have archived concept in API
			})
		}

		nextURL = page.Next
	}

	return allRepos, nil
}

// GetRepository gets a specific repository.
func (c *BitbucketClient) GetRepository(ctx context.Context, name string) (*Repository, error) {
	path := "/repositories/" + name

	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("bitbucket request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bitbucket error %d: %s", resp.StatusCode, string(body))
	}

	var r struct {
		Name     string `json:"name"`
		FullName string `json:"full_name"`
		Links    struct {
			Clone []struct {
				Name string `json:"name"`
				Href string `json:"href"`
			} `json:"clone"`
		} `json:"links"`
		Mainbranch struct {
			Name string `json:"name"`
		} `json:"mainbranch"`
		IsPrivate bool `json:"is_private"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	cloneURL := ""

	for _, link := range r.Links.Clone {
		if link.Name == "https" {
			cloneURL = link.Href
			break
		}
	}

	defaultBranch := r.Mainbranch.Name
	if defaultBranch == "" {
		defaultBranch = "main"
	}

	return &Repository{
		Name:          r.Name,
		FullName:      r.FullName,
		CloneURL:      cloneURL,
		DefaultBranch: defaultBranch,
		Private:       r.IsPrivate,
		Archived:      false,
	}, nil
}

// CreateMergeRequest creates a pull request on Bitbucket.
func (c *BitbucketClient) CreateMergeRequest(
	ctx context.Context,
	repo, title, description, sourceBranch, targetBranch string,
) (*MergeRequest, error) {
	path := fmt.Sprintf("/repositories/%s/pullrequests", repo)
	payload := fmt.Sprintf(`{
		"title": %q,
		"description": %q,
		"source": {"branch": {"name": %q}},
		"destination": {"branch": {"name": %q}}
	}`, title, description, sourceBranch, targetBranch)

	resp, err := c.doRequest(ctx, "POST", path, strings.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("bitbucket request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bitbucket error %d: %s", resp.StatusCode, string(body))
	}

	var pr struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
		Links struct {
			HTML struct {
				Href string `json:"href"`
			} `json:"html"`
		} `json:"links"`
		State     string    `json:"state"`
		CreatedOn time.Time `json:"created_on"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &MergeRequest{
		ID:        pr.ID,
		Number:    pr.ID,
		Title:     pr.Title,
		URL:       pr.Links.HTML.Href,
		State:     pr.State,
		CreatedAt: pr.CreatedOn,
	}, nil
}

// GetCloneURL returns the clone URL for a repository.
func (c *BitbucketClient) GetCloneURL(repo string) string {
	return fmt.Sprintf("https://bitbucket.org/%s.git", repo)
}
