package server

import (
	"github.com/mark3labs/mcp-go/server"

	"github.com/aanogueira/code-search/cmd/mcp/client"
	"github.com/aanogueira/code-search/cmd/mcp/tools"
)

// NewMCPServer creates a new MCP server with all tools registered.
// If authToken is non-empty, the API client will send it as a Bearer token.
func NewMCPServer(apiURL, authToken string) *server.MCPServer {
	apiClient := client.New(apiURL, authToken)

	s := server.NewMCPServer(
		"code-search",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	// Search
	s.AddTool(tools.SearchCodeTool(), tools.HandleSearchCode(apiClient))

	// Repositories
	s.AddTool(tools.ListReposTool(), tools.HandleListRepos(apiClient))
	s.AddTool(tools.GetRepoBranchesTool(), tools.HandleGetRepoBranches(apiClient))

	// File browsing
	s.AddTool(tools.GetFileTreeTool(), tools.HandleGetFileTree(apiClient))
	s.AddTool(tools.GetFileContentTool(), tools.HandleGetFileContent(apiClient))

	// SCIP code intelligence
	s.AddTool(tools.SearchSymbolsTool(), tools.HandleSearchSymbols(apiClient))
	s.AddTool(tools.GoToDefinitionTool(), tools.HandleGoToDefinition(apiClient))
	s.AddTool(tools.FindReferencesTool(), tools.HandleFindReferences(apiClient))

	return s
}
