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

// SearchSymbolsTool returns the tool definition for search_symbols.
func SearchSymbolsTool() mcp.Tool {
	return mcp.NewTool("search_symbols",
		mcp.WithDescription("Search for symbol definitions (functions, classes, methods, variables) in a repository using SCIP code intelligence. Requires SCIP indexing to be enabled on the server."),
		mcp.WithNumber("repo_id",
			mcp.Required(),
			mcp.Description("Repository ID (use list_repos to find IDs)"),
		),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Symbol name to search for (substring match)"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return (default: 20)"),
		),
	)
}

// HandleSearchSymbols returns a handler for the search_symbols tool.
func HandleSearchSymbols(c *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		repoID, err := req.RequireInt("repo_id")
		if err != nil {
			return mcp.NewToolResultError("repo_id is required"), nil
		}

		query, err := req.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError("query is required"), nil
		}

		limit := req.GetInt("limit", 20)

		resp, err := c.SearchSymbols(ctx, int64(repoID), query, limit)
		if err != nil {
			return mcp.NewToolResultError(formatSCIPError("search symbols", err)), nil
		}

		return mcp.NewToolResultText(formatSymbolResults(resp)), nil
	}
}

// GoToDefinitionTool returns the tool definition for go_to_definition.
func GoToDefinitionTool() mcp.Tool {
	return mcp.NewTool("go_to_definition",
		mcp.WithDescription("Jump to the definition of a symbol at a specific location in a file. Uses SCIP code intelligence for precise navigation. Line is 1-indexed, column is 0-indexed."),
		mcp.WithNumber("repo_id",
			mcp.Required(),
			mcp.Description("Repository ID (use list_repos to find IDs)"),
		),
		mcp.WithString("file",
			mcp.Required(),
			mcp.Description("File path within the repository"),
		),
		mcp.WithNumber("line",
			mcp.Required(),
			mcp.Description("Line number (1-indexed, as shown in editors)"),
		),
		mcp.WithNumber("column",
			mcp.Required(),
			mcp.Description("Column number (0-indexed)"),
		),
	)
}

// HandleGoToDefinition returns a handler for the go_to_definition tool.
func HandleGoToDefinition(c *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		repoID, err := req.RequireInt("repo_id")
		if err != nil {
			return mcp.NewToolResultError("repo_id is required"), nil
		}

		file, err := req.RequireString("file")
		if err != nil {
			return mcp.NewToolResultError("file is required"), nil
		}

		line, err := req.RequireInt("line")
		if err != nil {
			return mcp.NewToolResultError("line is required"), nil
		}

		column, err := req.RequireInt("column")
		if err != nil {
			return mcp.NewToolResultError("column is required"), nil
		}

		resp, err := c.GoToDefinition(ctx, int64(repoID), file, line, column)
		if err != nil {
			return mcp.NewToolResultError(formatSCIPError("go to definition", err)), nil
		}

		return mcp.NewToolResultText(formatDefinition(resp)), nil
	}
}

// FindReferencesTool returns the tool definition for find_references.
func FindReferencesTool() mcp.Tool {
	return mcp.NewTool("find_references",
		mcp.WithDescription("Find all usages/references of a symbol at a specific location in a file. Uses SCIP code intelligence. Line is 1-indexed, column is 0-indexed."),
		mcp.WithNumber("repo_id",
			mcp.Required(),
			mcp.Description("Repository ID (use list_repos to find IDs)"),
		),
		mcp.WithString("file",
			mcp.Required(),
			mcp.Description("File path within the repository"),
		),
		mcp.WithNumber("line",
			mcp.Required(),
			mcp.Description("Line number (1-indexed, as shown in editors)"),
		),
		mcp.WithNumber("column",
			mcp.Required(),
			mcp.Description("Column number (0-indexed)"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of references to return (default: 50)"),
		),
	)
}

// HandleFindReferences returns a handler for the find_references tool.
func HandleFindReferences(c *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		repoID, err := req.RequireInt("repo_id")
		if err != nil {
			return mcp.NewToolResultError("repo_id is required"), nil
		}

		file, err := req.RequireString("file")
		if err != nil {
			return mcp.NewToolResultError("file is required"), nil
		}

		line, err := req.RequireInt("line")
		if err != nil {
			return mcp.NewToolResultError("line is required"), nil
		}

		column, err := req.RequireInt("column")
		if err != nil {
			return mcp.NewToolResultError("column is required"), nil
		}

		limit := req.GetInt("limit", 50)

		resp, err := c.FindReferences(ctx, int64(repoID), file, line, column, limit)
		if err != nil {
			return mcp.NewToolResultError(formatSCIPError("find references", err)), nil
		}

		return mcp.NewToolResultText(formatReferences(resp)), nil
	}
}

func formatSCIPError(operation string, err error) string {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		if apiErr.IsNotFound() {
			return "SCIP code intelligence is not available for this repository. The repository may not have a SCIP index, or SCIP is not enabled on this server."
		}

		return fmt.Sprintf("%s failed: %s", operation, apiErr.Body)
	}

	return formatAPIError(operation, err)
}

func formatSymbolResults(resp *client.SearchSymbolsResponse) string {
	if resp.Count == 0 {
		return "No symbols found matching the query."
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d symbols:\n\n", resp.Count)

	for _, s := range resp.Results {
		name := s.DisplayName
		if name == "" {
			name = s.Symbol
		}

		fmt.Fprintf(&sb, "  %s\n", name)
		fmt.Fprintf(&sb, "    Location: %s:%d\n", s.FilePath, s.Line)

		if s.Documentation != "" {
			fmt.Fprintf(&sb, "    Doc: %s\n", s.Documentation)
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

func formatDefinition(resp *client.GoToDefinitionResponse) string {
	if !resp.Found {
		return "No definition found at the specified location."
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Symbol: %s\n", resp.Symbol)

	if resp.External {
		sb.WriteString("(external symbol - definition is in an external package)\n")
	}

	if resp.Definition != nil {
		fmt.Fprintf(&sb, "\nDefinition: %s:%d:%d\n",
			resp.Definition.FilePath,
			resp.Definition.StartLine+1,
			resp.Definition.StartCol)

		if resp.Definition.Context != "" {
			fmt.Fprintf(&sb, "  %s\n", resp.Definition.Context)
		}
	}

	if resp.Info != nil && resp.Info.Documentation != "" {
		fmt.Fprintf(&sb, "\nDocumentation: %s\n", resp.Info.Documentation)
	}

	return sb.String()
}

func formatReferences(resp *client.FindReferencesResponse) string {
	if !resp.Found {
		return "No references found at the specified location."
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Symbol: %s\n", resp.Symbol)
	fmt.Fprintf(&sb, "Total references: %d\n", resp.TotalCount)

	if resp.Definition != nil {
		fmt.Fprintf(&sb, "\nDefinition: %s:%d:%d\n",
			resp.Definition.FilePath,
			resp.Definition.StartLine+1,
			resp.Definition.StartCol)

		if resp.Definition.Context != "" {
			fmt.Fprintf(&sb, "  %s\n", resp.Definition.Context)
		}
	}

	if len(resp.References) > 0 {
		fmt.Fprintf(&sb, "\nReferences (%d):\n", len(resp.References))

		for _, ref := range resp.References {
			fmt.Fprintf(&sb, "  %s:%d:%d",
				ref.FilePath,
				ref.StartLine+1,
				ref.StartCol)

			if ref.Context != "" {
				fmt.Fprintf(&sb, "  |  %s", ref.Context)
			}

			sb.WriteString("\n")
		}
	}

	if resp.TotalCount > len(resp.References) {
		fmt.Fprintf(&sb, "\n[Showing %d of %d references. Increase limit for more.]",
			len(resp.References), resp.TotalCount)
	}

	return sb.String()
}
