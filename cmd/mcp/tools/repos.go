package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/techquestsdev/code-search/cmd/mcp/client"
)

// ListReposTool returns the tool definition for list_repos.
func ListReposTool() mcp.Tool {
	return mcp.NewTool(
		"list_repos",
		mcp.WithDescription(
			"List indexed repositories. Use this to discover available repositories and their IDs, which are needed for other tools like get_file_tree, get_file_content, and SCIP tools.",
		),
		mcp.WithString("search",
			mcp.Description("Filter repositories by name (substring match)"),
		),
		mcp.WithString("status",
			mcp.Description("Filter by index status: 'indexed', 'pending', 'failed'"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of repositories to return (default: 50)"),
		),
		mcp.WithNumber("offset",
			mcp.Description("Number of repositories to skip for pagination"),
		),
	)
}

// HandleListRepos returns a handler for the list_repos tool.
func HandleListRepos(c *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		params := client.ListReposParams{
			Search: req.GetString("search", ""),
			Status: req.GetString("status", ""),
			Limit:  req.GetInt("limit", 50),
			Offset: req.GetInt("offset", 0),
		}

		resp, err := c.ListRepos(ctx, params)
		if err != nil {
			return mcp.NewToolResultError(formatAPIError("list repos", err)), nil
		}

		return mcp.NewToolResultText(formatListRepos(resp)), nil
	}
}

// GetRepoBranchesTool returns the tool definition for get_repo_branches.
func GetRepoBranchesTool() mcp.Tool {
	return mcp.NewTool(
		"get_repo_branches",
		mcp.WithDescription(
			"List branches and tags for a repository. Returns the default branch, all branches, and all tags.",
		),
		mcp.WithNumber("repo_id",
			mcp.Required(),
			mcp.Description("Repository ID (use list_repos to find IDs)"),
		),
	)
}

// HandleGetRepoBranches returns a handler for the get_repo_branches tool.
func HandleGetRepoBranches(c *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		repoID, err := req.RequireInt("repo_id")
		if err != nil {
			return mcp.NewToolResultError("repo_id is required"), nil
		}

		resp, err := c.GetBranchesAndTags(ctx, int64(repoID))
		if err != nil {
			return mcp.NewToolResultError(formatAPIError("get branches", err)), nil
		}

		return mcp.NewToolResultText(formatBranches(resp)), nil
	}
}

func formatListRepos(resp *client.ListReposResponse) string {
	if len(resp.Repos) == 0 {
		return "No repositories found."
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Repositories (showing %d of %d):\n\n", len(resp.Repos), resp.TotalCount)
	fmt.Fprintf(
		&sb,
		"%-6s %-50s %-10s %-10s %s\n",
		"ID",
		"Name",
		"Status",
		"Branch",
		"Last Indexed",
	)
	sb.WriteString(strings.Repeat("-", 110) + "\n")

	for _, r := range resp.Repos {
		lastIndexed := "-"
		if r.LastIndexed != "" {
			lastIndexed = r.LastIndexed
		}

		branch := r.DefaultBranch
		if branch == "" {
			branch = "-"
		}

		fmt.Fprintf(&sb, "%-6d %-50s %-10s %-10s %s\n",
			r.ID, r.Name, r.Status, branch, lastIndexed)
	}

	if resp.HasMore {
		fmt.Fprintf(
			&sb,
			"\n[More results available. Use offset=%d to see next page.]",
			resp.Offset+len(resp.Repos),
		)
	}

	return sb.String()
}

func formatBranches(resp *client.RefsResponse) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Default branch: %s\n", resp.DefaultBranch)

	if len(resp.Branches) > 0 {
		fmt.Fprintf(&sb, "\nBranches (%d):\n", len(resp.Branches))

		for _, b := range resp.Branches {
			fmt.Fprintf(&sb, "  %s\n", b)
		}
	} else {
		sb.WriteString("\nNo branches found.\n")
	}

	if len(resp.Tags) > 0 {
		fmt.Fprintf(&sb, "\nTags (%d):\n", len(resp.Tags))

		for _, t := range resp.Tags {
			fmt.Fprintf(&sb, "  %s\n", t)
		}
	}

	return sb.String()
}
