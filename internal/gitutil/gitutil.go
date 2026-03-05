// Package gitutil provides shared git-related utility functions.
package gitutil

import (
	"strings"

	"github.com/aanogueira/code-search/internal/repos"
)

// AddAuthToURL adds authentication token to a git URL based on connection type.
// This is the canonical implementation used across the codebase.
func AddAuthToURL(url string, conn *repos.Connection) string {
	if conn == nil || conn.Token == "" {
		return url
	}

	switch conn.Type {
	case "github":
		// GitHub uses x-access-token format for GitHub Apps/PATs
		if strings.Contains(url, "github.com") {
			return strings.Replace(
				url,
				"https://github.com",
				"https://x-access-token:"+conn.Token+"@github.com",
				1,
			)
		}
	case "github_enterprise":
		// For GitHub Enterprise: https://x-access-token:token@ghe.example.com/owner/repo.git
		if conn.URL != "" {
			host := ExtractHost(conn.URL)
			if strings.Contains(url, host) {
				return strings.Replace(
					url,
					"https://"+host,
					"https://x-access-token:"+conn.Token+"@"+host,
					1,
				)
			}
		}
	case "gitlab":
		// For GitLab: https://oauth2:token@gitlab.com/owner/repo.git
		connURL := conn.URL
		if connURL == "" {
			connURL = "https://gitlab.com"
		}

		host := ExtractHost(connURL)
		if strings.Contains(url, host) {
			return strings.Replace(url, "https://"+host, "https://oauth2:"+conn.Token+"@"+host, 1)
		}
	case "gitea":
		// For Gitea: https://git:token@gitea.example.com/owner/repo.git
		if conn.URL != "" {
			host := ExtractHost(conn.URL)
			if strings.Contains(url, host) {
				return strings.Replace(url, "https://"+host, "https://git:"+conn.Token+"@"+host, 1)
			}
		}
	case "bitbucket":
		// For Bitbucket: https://x-token-auth:token@bitbucket.org/owner/repo.git
		if strings.Contains(url, "bitbucket.org") {
			return strings.Replace(
				url,
				"https://bitbucket.org",
				"https://x-token-auth:"+conn.Token+"@bitbucket.org",
				1,
			)
		}
	}

	return url
}

// ExtractHost extracts the host from a URL (e.g., "https://github.com/foo" -> "github.com").
func ExtractHost(url string) string {
	host := strings.TrimPrefix(strings.TrimPrefix(url, "https://"), "http://")
	host = strings.TrimSuffix(host, "/")
	// Remove any path components
	if idx := strings.Index(host, "/"); idx != -1 {
		host = host[:idx]
	}

	return host
}

// SanitizeGitOutput removes sensitive information (tokens) from git command output.
func SanitizeGitOutput(output string, conn *repos.Connection) string {
	if conn == nil || conn.Token == "" {
		return output
	}
	// Replace any occurrence of the token with [REDACTED]
	return strings.ReplaceAll(output, conn.Token, "[REDACTED]")
}
