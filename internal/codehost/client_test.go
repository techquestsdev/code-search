package codehost

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitHubClient_doRequest(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify authorization header (GitHub uses Bearer token)
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-token" {
			t.Errorf("expected Authorization 'Bearer test-token', got %q", auth)
		}

		// Set rate limit headers
		w.Header().Set("X-Ratelimit-Remaining", "4999")
		w.Header().Set("X-Ratelimit-Reset", "1700000000")

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"login": "testuser"}`))
	}))
	defer server.Close()

	client := NewGitHubClient("test-token")
	client.baseURL = server.URL

	resp, err := client.doRequest(context.Background(), "GET", "/user", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestGitHubClient_doRequest_Error(t *testing.T) {
	client := NewGitHubClient("test-token")
	client.baseURL = "http://invalid-host-that-does-not-exist:12345"

	resp, err := client.doRequest(context.Background(), "GET", "/user", nil)
	if resp != nil {
		resp.Body.Close()
	}

	if err == nil {
		t.Error("expected error for invalid host")
	}
}

func TestGitHubClient_doRequest_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewGitHubClient("test-token")
	client.baseURL = server.URL

	resp, err := client.doRequest(context.Background(), "GET", "/user", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", resp.StatusCode)
	}
}

func TestGitLabClient_doRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify authorization header
		if auth := r.Header.Get("Private-Token"); auth != "test-token" {
			t.Errorf("expected Private-Token 'test-token', got %q", auth)
		}

		// Set rate limit headers (GitLab format)
		w.Header().Set("Ratelimit-Remaining", "1999")
		w.Header().Set("Ratelimit-Reset", "1700000000")

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"username": "testuser"}`))
	}))
	defer server.Close()

	client := NewGitLabClient(server.URL, "test-token")

	resp, err := client.doRequest(context.Background(), "GET", "/user", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestGitLabClient_doRequest_Error(t *testing.T) {
	client := NewGitLabClient("http://invalid-host:12345", "test-token")

	resp, err := client.doRequest(context.Background(), "GET", "/user", nil)
	if resp != nil {
		resp.Body.Close()
	}

	if err == nil {
		t.Error("expected error for invalid host")
	}
}

func TestBitbucketClient_doRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify accept header
		if accept := r.Header.Get("Accept"); accept != "application/json" {
			t.Errorf("expected Accept 'application/json', got %q", accept)
		}

		// Set rate limit headers
		w.Header().Set("X-Ratelimit-Remaining", "999")
		w.Header().Set("X-Ratelimit-Reset", "1700000000")

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"username": "testuser"}`))
	}))
	defer server.Close()

	client := NewBitbucketClient("user:app_password")
	client.baseURL = server.URL

	resp, err := client.doRequest(context.Background(), "GET", "/user", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestBitbucketClient_doRequest_BearerToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// For non-username:password format, should use Bearer auth
		auth := r.Header.Get("Authorization")
		if auth != "Bearer my-oauth-token" {
			t.Errorf("expected Authorization 'Bearer my-oauth-token', got %q", auth)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Without username:password format
	client := NewBitbucketClient("my-oauth-token")
	client.baseURL = server.URL

	resp, err := client.doRequest(context.Background(), "GET", "/user", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
}

func TestBitbucketClient_doRequest_Error(t *testing.T) {
	client := NewBitbucketClient("user:pass")
	client.baseURL = "http://invalid-host:12345"

	resp, err := client.doRequest(context.Background(), "GET", "/user", nil)
	if resp != nil {
		resp.Body.Close()
	}

	if err == nil {
		t.Error("expected error for invalid host")
	}
}

func TestGiteaClient_doRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify authorization header
		if auth := r.Header.Get("Authorization"); auth != "token test-token" {
			t.Errorf("expected Authorization 'token test-token', got %q", auth)
		}

		// Verify accept header
		if accept := r.Header.Get("Accept"); accept != "application/json" {
			t.Errorf("expected Accept 'application/json', got %q", accept)
		}

		// Set rate limit headers
		w.Header().Set("X-Ratelimit-Remaining", "499")
		w.Header().Set("X-Ratelimit-Reset", "1700000000")

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"login": "testuser"}`))
	}))
	defer server.Close()

	client := NewGiteaClient(server.URL, "test-token")

	resp, err := client.doRequest(context.Background(), "GET", "/user", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestGiteaClient_doRequest_Error(t *testing.T) {
	client := NewGiteaClient("http://invalid-host:12345", "test-token")

	resp, err := client.doRequest(context.Background(), "GET", "/user", nil)
	if resp != nil {
		resp.Body.Close()
	}

	if err == nil {
		t.Error("expected error for invalid host")
	}
}

func TestNewGitHubClient(t *testing.T) {
	client := NewGitHubClient("my-token")

	if client.token != "my-token" {
		t.Errorf("expected token 'my-token', got %q", client.token)
	}

	if client.baseURL != "https://api.github.com" {
		t.Errorf("expected baseURL 'https://api.github.com', got %q", client.baseURL)
	}
}

func TestNewGitHubEnterpriseClient(t *testing.T) {
	client := NewGitHubEnterpriseClient("https://github.mycompany.com", "my-token")

	if client.token != "my-token" {
		t.Errorf("expected token 'my-token', got %q", client.token)
	}

	if client.baseURL != "https://github.mycompany.com/api/v3" {
		t.Errorf("expected baseURL 'https://github.mycompany.com/api/v3', got %q", client.baseURL)
	}
}

func TestNewGitLabClient(t *testing.T) {
	client := NewGitLabClient("https://gitlab.example.com", "my-token")

	if client.token != "my-token" {
		t.Errorf("expected token 'my-token', got %q", client.token)
	}

	if client.baseURL != "https://gitlab.example.com/api/v4" {
		t.Errorf("expected baseURL to end with /api/v4, got %q", client.baseURL)
	}
}

func TestNewBitbucketClient(t *testing.T) {
	t.Run("with username:password", func(t *testing.T) {
		client := NewBitbucketClient("user:app_password")

		if client.username != "user" {
			t.Errorf("expected username 'user', got %q", client.username)
		}

		if client.appPassword != "app_password" {
			t.Errorf("expected appPassword 'app_password', got %q", client.appPassword)
		}
	})

	t.Run("with token only", func(t *testing.T) {
		client := NewBitbucketClient("my-oauth-token")

		if client.username != "" {
			t.Errorf("expected empty username, got %q", client.username)
		}

		if client.appPassword != "my-oauth-token" {
			t.Errorf("expected appPassword 'my-oauth-token', got %q", client.appPassword)
		}
	})
}

func TestNewBitbucketServerClient(t *testing.T) {
	client := NewBitbucketServerClient("https://bitbucket.mycompany.com", "my-token")

	if client.appPassword != "my-token" {
		t.Errorf("expected token 'my-token', got %q", client.appPassword)
	}

	if client.baseURL != "https://bitbucket.mycompany.com/rest/api/1.0" {
		t.Errorf("expected baseURL to end with /rest/api/1.0, got %q", client.baseURL)
	}
}

func TestNewGiteaClient(t *testing.T) {
	client := NewGiteaClient("https://gitea.example.com", "my-token")

	if client.token != "my-token" {
		t.Errorf("expected token 'my-token', got %q", client.token)
	}

	if client.baseURL != "https://gitea.example.com/api/v1" {
		t.Errorf("expected baseURL to end with /api/v1, got %q", client.baseURL)
	}
}

func TestGetCloneURL(t *testing.T) {
	tests := []struct {
		name     string
		client   interface{ GetCloneURL(string) string }
		repo     string
		expected string
	}{
		{
			name:     "GitHub",
			client:   NewGitHubClient("token"),
			repo:     "owner/repo",
			expected: "https://github.com/owner/repo.git",
		},
		{
			name:     "GitLab",
			client:   NewGitLabClient("https://gitlab.com", "token"),
			repo:     "owner/repo",
			expected: "https://gitlab.com/owner/repo.git",
		},
		{
			name:     "Bitbucket",
			client:   NewBitbucketClient("token"),
			repo:     "owner/repo",
			expected: "https://bitbucket.org/owner/repo.git",
		},
		{
			name:     "Gitea",
			client:   NewGiteaClient("https://gitea.example.com", "token"),
			repo:     "owner/repo",
			expected: "https://gitea.example.com/owner/repo.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.client.GetCloneURL(tt.repo)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
