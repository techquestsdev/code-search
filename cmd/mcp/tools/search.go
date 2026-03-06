package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/techquestsdev/code-search/cmd/mcp/client"
)

// SearchCodeTool returns the tool definition for search_code.
func SearchCodeTool() mcp.Tool {
	return mcp.NewTool("search_code",
		mcp.WithDescription("Search for code patterns across all indexed repositories. Supports text and regex search with filtering by repo, language, and file pattern."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Search query (text or regex pattern). Supports Zoekt query syntax including operators like repo:, file:, lang:, case:yes, sym:, branch:"),
		),
		mcp.WithString("repos",
			mcp.Description("Comma-separated list of repository name patterns to search in (regex). Example: 'org/repo1,org/repo2'"),
		),
		mcp.WithString("languages",
			mcp.Description("Comma-separated list of languages to filter by. Example: 'go,typescript'"),
		),
		mcp.WithString("file_patterns",
			mcp.Description("Comma-separated list of file path patterns to filter by. Example: '*.go,*.ts'"),
		),
		mcp.WithBoolean("is_regex",
			mcp.Description("Treat query as a regular expression"),
		),
		mcp.WithBoolean("case_sensitive",
			mcp.Description("Enable case-sensitive search"),
		),
		mcp.WithNumber("context_lines",
			mcp.Description("Number of context lines before and after each match (default: 2)"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return (default: 50)"),
		),
	)
}

// HandleSearchCode returns a handler for the search_code tool.
func HandleSearchCode(c *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := req.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError("query is required"), nil
		}

		searchReq := &client.SearchRequest{
			Query:         query,
			ContextLines:  req.GetInt("context_lines", 2),
			Limit:         req.GetInt("limit", 50),
			IsRegex:       req.GetBool("is_regex", false),
			CaseSensitive: req.GetBool("case_sensitive", false),
		}

		if repos := req.GetString("repos", ""); repos != "" {
			searchReq.Repos = splitComma(repos)
		}

		if languages := req.GetString("languages", ""); languages != "" {
			searchReq.Languages = splitComma(languages)
		}

		if filePatterns := req.GetString("file_patterns", ""); filePatterns != "" {
			searchReq.FilePatterns = splitComma(filePatterns)
		}

		resp, err := c.Search(ctx, searchReq)
		if err != nil {
			return mcp.NewToolResultError(formatAPIError("search", err)), nil
		}

		return mcp.NewToolResultText(formatSearchResults(resp)), nil
	}
}

func formatSearchResults(resp *client.SearchResponse) string {
	if len(resp.Results) == 0 {
		return fmt.Sprintf("No results found (%s)", resp.Duration)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d matches (%s)\n", resp.TotalCount, resp.Duration)

	// Group results by repo+file
	type fileGroup struct {
		repo     string
		file     string
		language string
		results  []client.SearchResult
	}

	var (
		groups  []fileGroup
		current *fileGroup
	)

	for _, r := range resp.Results {
		if current == nil || current.repo != r.Repo || current.file != r.File {
			groups = append(groups, fileGroup{
				repo:     r.Repo,
				file:     r.File,
				language: r.Language,
			})
			current = &groups[len(groups)-1]
		}

		current.results = append(current.results, r)
	}

	for _, g := range groups {
		langLabel := ""
		if g.language != "" {
			langLabel = " (" + g.language + ")"
		}

		fmt.Fprintf(&sb, "\n--- %s :: %s%s ---\n", g.repo, g.file, langLabel)

		for _, r := range g.results {
			// Before context
			for i, line := range r.Context.Before {
				lineNum := r.Line - len(r.Context.Before) + i
				fmt.Fprintf(&sb, "  %d: %s\n", lineNum, line)
			}
			// Match line
			fmt.Fprintf(&sb, "> %d: %s\n", r.Line, r.Content)
			// After context
			for i, line := range r.Context.After {
				fmt.Fprintf(&sb, "  %d: %s\n", r.Line+1+i, line)
			}
		}
	}

	if resp.Truncated {
		sb.WriteString("\n[Results truncated. Increase limit parameter for more.]")
	}

	return sb.String()
}

func splitComma(s string) []string {
	parts := strings.Split(s, ",")

	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}

	return result
}

func formatAPIError(operation string, err error) string {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		if apiErr.IsNotFound() {
			return "Not found. Use list_repos to see available repositories."
		}

		return fmt.Sprintf("%s failed: %s", operation, apiErr.Body)
	}

	if strings.Contains(err.Error(), "connect to Code Search API") {
		return fmt.Sprintf("Failed to connect to Code Search API. Is the API server running? Error: %v", err)
	}

	return fmt.Sprintf("%s failed: %v", operation, err)
}
