package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/techquestsdev/code-search/cmd/mcp/client"
)

// GetFileTreeTool returns the tool definition for get_file_tree.
func GetFileTreeTool() mcp.Tool {
	return mcp.NewTool(
		"get_file_tree",
		mcp.WithDescription(
			"Browse directory contents in a repository. Returns files and subdirectories at the specified path.",
		),
		mcp.WithNumber("repo_id",
			mcp.Required(),
			mcp.Description("Repository ID (use list_repos to find IDs)"),
		),
		mcp.WithString("path",
			mcp.Description("Directory path to list (default: root '/')"),
		),
		mcp.WithString(
			"ref",
			mcp.Description(
				"Git ref (branch, tag, or commit SHA) to browse (default: repository's default branch)",
			),
		),
	)
}

// HandleGetFileTree returns a handler for the get_file_tree tool.
func HandleGetFileTree(c *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		repoID, err := req.RequireInt("repo_id")
		if err != nil {
			return mcp.NewToolResultError("repo_id is required"), nil
		}

		path := req.GetString("path", "")
		ref := req.GetString("ref", "")

		resp, err := c.GetFileTree(ctx, int64(repoID), path, ref)
		if err != nil {
			return mcp.NewToolResultError(formatAPIError("get file tree", err)), nil
		}

		return mcp.NewToolResultText(formatFileTree(resp, int64(repoID))), nil
	}
}

// GetFileContentTool returns the tool definition for get_file_content.
func GetFileContentTool() mcp.Tool {
	return mcp.NewTool(
		"get_file_content",
		mcp.WithDescription(
			"Read the content of a file in a repository. Returns the file content with syntax highlighting language detection. Binary files return a message instead of content.",
		),
		mcp.WithNumber("repo_id",
			mcp.Required(),
			mcp.Description("Repository ID (use list_repos to find IDs)"),
		),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("File path within the repository"),
		),
		mcp.WithString(
			"ref",
			mcp.Description(
				"Git ref (branch, tag, or commit SHA) to read from (default: repository's default branch)",
			),
		),
	)
}

// HandleGetFileContent returns a handler for the get_file_content tool.
func HandleGetFileContent(c *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		repoID, err := req.RequireInt("repo_id")
		if err != nil {
			return mcp.NewToolResultError("repo_id is required"), nil
		}

		path, err := req.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError("path is required"), nil
		}

		ref := req.GetString("ref", "")

		resp, err := c.GetFileContent(ctx, int64(repoID), path, ref)
		if err != nil {
			return mcp.NewToolResultError(formatAPIError("get file content", err)), nil
		}

		return mcp.NewToolResultText(formatFileContent(resp)), nil
	}
}

func formatFileTree(resp *client.TreeResponse, repoID int64) string {
	var sb strings.Builder

	displayPath := resp.Path
	if displayPath == "" {
		displayPath = "/"
	}

	fmt.Fprintf(&sb, "Files in repo #%d at \"%s\" (ref: %s):\n\n", repoID, displayPath, resp.Ref)

	dirs := 0
	files := 0

	for _, e := range resp.Entries {
		if e.Type == "dir" {
			fmt.Fprintf(&sb, "  [dir]  %s/\n", e.Name)

			dirs++
		} else {
			sizeStr := formatSize(e.Size)

			langStr := ""
			if e.Language != "" {
				langStr = e.Language + ", "
			}

			fmt.Fprintf(&sb, "  [file] %s (%s%s)\n", e.Name, langStr, sizeStr)

			files++
		}
	}

	if len(resp.Entries) == 0 {
		sb.WriteString("  (empty directory)\n")
	}

	fmt.Fprintf(&sb, "\n%d directories, %d files", dirs, files)

	return sb.String()
}

func formatFileContent(resp *client.BlobResponse) string {
	var sb strings.Builder

	if resp.Binary {
		fmt.Fprintf(&sb, "Binary file: %s (%s)\n", resp.Path, formatSize(resp.Size))
		sb.WriteString("Binary file content cannot be displayed.")

		return sb.String()
	}

	lang := resp.Language
	if lang == "" {
		lang = "text"
	}

	fmt.Fprintf(&sb, "File: %s | Language: %s | Size: %s | Ref: %s\n\n",
		resp.Path, lang, formatSize(resp.Size), resp.Ref)
	fmt.Fprintf(&sb, "```%s\n", strings.ToLower(lang))
	sb.WriteString(resp.Content)

	if !strings.HasSuffix(resp.Content, "\n") {
		sb.WriteString("\n")
	}

	sb.WriteString("```")

	return sb.String()
}

func formatSize(bytes int64) string {
	switch {
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
