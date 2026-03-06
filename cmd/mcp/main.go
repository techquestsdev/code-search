package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/server"

	mcpserver "github.com/techquestsdev/code-search/cmd/mcp/server"
)

func main() {
	// All logging must go to stderr (stdout is used by MCP stdio protocol)
	log.SetOutput(os.Stderr)

	apiURL := flag.String("api-url", "http://localhost:8080", "Code Search API URL")
	authToken := flag.String("auth-token", "", "Authentication token for the Code Search API (env: CODE_SEARCH_AUTH_TOKEN)")
	transport := flag.String("transport", "stdio", "Transport type: stdio or http")
	httpAddr := flag.String("http-addr", ":9090", "HTTP listen address (for http transport)")

	flag.Parse()

	// Environment variable overrides
	if v := os.Getenv("CODE_SEARCH_API_URL"); v != "" {
		*apiURL = v
	}

	if v := os.Getenv("CODE_SEARCH_AUTH_TOKEN"); v != "" {
		*authToken = v
	}

	*authToken = strings.TrimSpace(*authToken)

	if v := os.Getenv("CODE_SEARCH_MCP_TRANSPORT"); v != "" {
		*transport = v
	}

	if v := os.Getenv("CODE_SEARCH_MCP_HTTP_ADDR"); v != "" {
		*httpAddr = v
	}

	if *authToken != "" {
		log.Printf("Authentication enabled (token configured)")
	} else {
		log.Printf("Running without authentication")
	}

	mcpServer := mcpserver.NewMCPServer(*apiURL, *authToken)

	switch *transport {
	case "stdio":
		log.Printf("Starting MCP server (stdio) connecting to API at %s", *apiURL)

		if err := server.ServeStdio(mcpServer); err != nil {
			fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
			os.Exit(1)
		}
	case "http":
		log.Printf("Starting MCP server (HTTP) on %s connecting to API at %s", *httpAddr, *apiURL)

		httpServer := server.NewStreamableHTTPServer(mcpServer)
		if err := httpServer.Start(*httpAddr); err != nil {
			fmt.Fprintf(os.Stderr, "MCP HTTP server error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown transport: %s (use 'stdio' or 'http')\n", *transport)
		os.Exit(1)
	}
}
