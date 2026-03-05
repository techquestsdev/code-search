package cmd

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/techquestsdev/code-search/cmd/cli/client"
)

var replaceCmd = &cobra.Command{
	Use:   "replace <old> <new>",
	Short: "Search and replace across repositories",
	Long: `Search for a pattern and replace it with a new value across repositories.

The replace command works in two steps:
1. Preview: Search for matches (default mode)
2. Execute: Apply replacements and create merge requests

All replacements create merge requests - changes are never committed directly to main.

Filtering Options:
  -R, --repos      Filter by repositories (can specify multiple)
  -f, --files      File patterns to include (e.g., "*.go", "*.ts")
  -l, --langs      Filter by languages (e.g., "go", "typescript")
  -n, --limit      Maximum number of results (default: 1000)
  -C, --context    Lines of context around matches (default: 2)
  -r, --regex      Treat pattern as regex
  --case-sensitive Enable case-sensitive matching

Authentication:
  --token         Single token for all connections (wildcard)
  --tokens        Per-connection tokens (format: connection_name=token)

When the server is in read-only mode, you must provide tokens - server-side 
tokens cannot be used for write operations.

Examples:
  # Preview changes (default)
  code-search replace "oldFunc" "newFunc"

  # Search with spaces (use quotes)
  code-search replace "old function name" "newFunctionName"

  # Filter by language and show more context
  code-search replace "TODO" "DONE" --langs go --context 5

  # Execute with single token (applies to all connections)
  code-search replace "oldFunc" "newFunc" --execute --token ghp_xxx

  # Execute with per-connection tokens
  code-search replace "oldFunc" "newFunc" --execute --tokens github=ghp_xxx --tokens gitlab=glpat_xxx

  # Mix: wildcard + specific override
  code-search replace "oldFunc" "newFunc" --execute --token ghp_xxx --tokens gitlab=glpat_yyy

  # Use regex pattern
  code-search replace "v(\d+)\.(\d+)" "v$1.$2.0" --regex --execute --token ghp_xxx
  
  # Limit results and filter by file pattern
  code-search replace "deprecated" "legacy" --files "*.md" --limit 50`,
	Args: cobra.ExactArgs(2),
	RunE: runReplace,
}

func init() {
	rootCmd.AddCommand(replaceCmd)

	replaceCmd.Flags().BoolP("regex", "r", false, "Treat pattern as regex")
	replaceCmd.Flags().BoolP("preview", "p", true, "Preview changes without applying")
	replaceCmd.Flags().Bool("execute", false, "Execute the replacements (creates MR)")
	replaceCmd.Flags().StringSliceP("repos", "R", nil, "Filter by repositories")
	replaceCmd.Flags().StringSliceP("files", "f", nil, "File patterns to include")
	replaceCmd.Flags().StringSliceP("langs", "l", nil, "Filter by languages (can specify multiple)")
	replaceCmd.Flags().Bool("case-sensitive", false, "Case-sensitive matching")
	replaceCmd.Flags().IntP("limit", "n", 1000, "Maximum number of results")
	replaceCmd.Flags().IntP("context", "C", 2, "Lines of context around match")
	replaceCmd.Flags().StringP("branch", "b", "", "Branch name for changes")
	replaceCmd.Flags().String("mr-title", "", "MR/PR title")
	replaceCmd.Flags().String("mr-description", "", "MR/PR description")
	replaceCmd.Flags().
		StringSlice("tokens", nil, "Per-connection tokens (format: connection_name=token, can specify multiple)")
}

func runReplace(cmd *cobra.Command, args []string) error {
	searchPattern := args[0]
	replaceWith := args[1]

	token := viper.GetString("token")
	output := viper.GetString("output")

	execute, _ := cmd.Flags().GetBool("execute")
	isRegex, _ := cmd.Flags().GetBool("regex")
	caseSensitive, _ := cmd.Flags().GetBool("case-sensitive")
	repos, _ := cmd.Flags().GetStringSlice("repos")
	files, _ := cmd.Flags().GetStringSlice("files")
	langs, _ := cmd.Flags().GetStringSlice("langs")
	limit, _ := cmd.Flags().GetInt("limit")
	contextLines, _ := cmd.Flags().GetInt("context")
	branch, _ := cmd.Flags().GetString("branch")
	mrTitle, _ := cmd.Flags().GetString("mr-title")
	mrDescription, _ := cmd.Flags().GetString("mr-description")
	tokensFlag, _ := cmd.Flags().GetStringSlice("tokens")

	// Parse per-connection tokens (format: connection_name=token)
	perConnectionTokens := make(map[string]string)

	for _, t := range tokensFlag {
		parts := strings.SplitN(t, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid token format %q: expected connection_name=token", t)
		}

		perConnectionTokens[parts[0]] = parts[1]
	}

	c := newClient()

	// Check if server is in read-only mode (tokens required only in read-only mode)
	hasUserTokens := token != "" || len(perConnectionTokens) > 0
	if execute && !hasUserTokens {
		// Check server status to see if tokens are required
		status, err := c.GetReposStatus(context.Background())
		if err != nil {
			return fmt.Errorf("cannot reach server to check read-only status: %w", err)
		}

		if status.ReadOnly {
			return errors.New(
				"server is in read-only mode: --token or --tokens is required when using --execute",
			)
		}
		// Server is not read-only, tokens are optional (server will use its configured tokens)
	}

	// Always run preview first to get matches
	fmt.Println("Searching for matches...")

	previewResp, err := c.ReplacePreview(context.Background(), &client.ReplacePreviewRequest{
		SearchPattern: searchPattern,
		ReplaceWith:   replaceWith,
		IsRegex:       isRegex,
		CaseSensitive: caseSensitive,
		Repos:         repos,
		FilePatterns:  files,
		Languages:     langs,
		Limit:         limit,
		ContextLines:  contextLines,
	})
	if err != nil {
		return fmt.Errorf("replace preview failed: %w", err)
	}

	if len(previewResp.Matches) == 0 {
		fmt.Println("No matches found")
		return nil
	}

	// If execute flag is set, run the replacement with the preview matches
	if execute {
		// Convert preview matches to execute matches (deduplicate by repo+file)
		matchMap := make(map[string]client.ReplaceMatch)

		for _, m := range previewResp.Matches {
			key := fmt.Sprintf("%s:%s", m.Repo, m.File)
			if _, exists := matchMap[key]; !exists {
				matchMap[key] = client.ReplaceMatch{
					RepositoryID:   m.RepositoryID,
					RepositoryName: m.Repo,
					FilePath:       m.File,
				}
			}
		}

		matches := make([]client.ReplaceMatch, 0, len(matchMap))
		for _, m := range matchMap {
			matches = append(matches, m)
		}

		fmt.Printf(
			"\n>> Executing replacements on %d files (MR will be created)...\n",
			len(matches),
		)

		// Build user tokens map:
		// - "*" key is the wildcard token (--token flag)
		// - Connection names are per-connection tokens (--tokens flag)
		userTokens := make(map[string]string)

		// Add wildcard token if provided
		if token != "" {
			userTokens["*"] = token
		}

		// Add per-connection tokens (these take precedence over wildcard)
		maps.Copy(userTokens, perConnectionTokens)

		resp, err := c.ReplaceExecute(context.Background(), &client.ReplaceExecuteRequest{
			SearchPattern: searchPattern,
			ReplaceWith:   replaceWith,
			IsRegex:       isRegex,
			CaseSensitive: caseSensitive,
			Matches:       matches,
			BranchName:    branch,
			MRTitle:       mrTitle,
			MRDescription: mrDescription,
			UserTokens:    userTokens,
		})
		if err != nil {
			return fmt.Errorf("replace execute failed: %w", err)
		}

		switch output {
		case "json":
			return outputJSON(resp)
		default:
			fmt.Printf("[OK] Replace job queued\n")
			fmt.Printf("   Job ID: %s\n", resp.JobID)
			fmt.Printf("   Status: %s\n", resp.Status)

			if resp.Message != "" {
				fmt.Printf("   Message: %s\n", resp.Message)
			}

			return nil
		}
	}

	// Preview mode - show what would be changed
	fmt.Println("\n[PREVIEW] No changes will be made")

	switch output {
	case "json":
		return outputJSON(previewResp)
	default:
		return outputReplacePreviewText(previewResp, searchPattern, replaceWith)
	}
}

func outputReplacePreviewText(
	resp *client.ReplacePreviewResponse,
	oldPattern, newPattern string,
) error {
	if len(resp.Matches) == 0 {
		fmt.Println("No matches found")
		return nil
	}

	// Extract the actual search term from query syntax like "content:testerino lang:md ..."
	actualSearchTerm := extractSearchTerm(oldPattern)

	fmt.Printf(
		"\n%s '%s' → '%s'\n",
		colorYellow("Replace:"),
		actualSearchTerm,
		colorGreen(newPattern),
	)
	fmt.Println(strings.Repeat("─", 60))

	// Group results by repo/file for better display
	currentFile := ""

	for _, match := range resp.Matches {
		fileKey := match.Repo + "/" + match.File
		if fileKey != currentFile {
			fmt.Printf("\n%s  %s\n", colorGreen(match.Repo), colorCyan(match.File))

			currentFile = fileKey
		}

		// Print before context
		for i, line := range match.Context.Before {
			lineNum := match.Line - len(match.Context.Before) + i
			fmt.Printf("  %4d │ %s\n", lineNum, colorGray(line))
		}

		// Find all matches in content and show inline diff
		inlineDiff := buildInlineDiff(
			match.Content,
			actualSearchTerm,
			newPattern,
			match.MatchStart,
			match.MatchEnd,
		)
		fmt.Printf("  %4d │ %s\n", match.Line, inlineDiff)

		// Print after context
		for i, line := range match.Context.After {
			lineNum := match.Line + i + 1
			fmt.Printf("  %4d │ %s\n", lineNum, colorGray(line))
		}
	}

	fmt.Println()
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf(
		"%s %d matches found (took %s)\n",
		colorYellow("Summary:"),
		resp.TotalCount,
		resp.Duration,
	)
	fmt.Printf(
		"\n%s Run with %s to apply changes and create merge requests\n",
		colorCyan("Hint:"),
		colorYellow("--execute"),
	)

	return nil
}

// buildInlineDiff creates an inline diff showing strikethrough red for deletions and green for insertions.
// Uses the exact match positions from the server for accurate highlighting.
// Example: "hello testerino world" with positions 6-15 becomes "hello [strikethrough]testerino[/][green]demo[/] world".
func buildInlineDiff(content, searchTerm, replacement string, matchStart, matchEnd int) string {
	// If we have valid match positions from the server, use them for precise highlighting
	if matchStart >= 0 && matchEnd > matchStart && matchEnd <= len(content) {
		var result strings.Builder

		// Text before match
		result.WriteString(content[:matchStart])

		// Strikethrough red for the deleted text
		result.WriteString("\033[9;31m") // strikethrough + red
		result.WriteString(content[matchStart:matchEnd])
		result.WriteString("\033[0m")

		// Bold green for the replacement
		result.WriteString("\033[1;32m") // bold + green
		result.WriteString(replacement)
		result.WriteString("\033[0m")

		// Text after match
		result.WriteString(content[matchEnd:])

		return result.String()
	}

	// Fallback: search for term manually (handles case-insensitive search)
	if searchTerm == "" {
		return content
	}

	var result strings.Builder

	lowerContent := strings.ToLower(content)
	lowerTerm := strings.ToLower(searchTerm)
	lastIndex := 0

	for {
		idx := strings.Index(lowerContent[lastIndex:], lowerTerm)
		if idx == -1 {
			// No more matches, append the rest
			result.WriteString(content[lastIndex:])
			break
		}

		// Actual index in original content
		actualIdx := lastIndex + idx

		// Append text before the match
		result.WriteString(content[lastIndex:actualIdx])

		// Append strikethrough red for the deleted text
		result.WriteString("\033[9;31m")                                   // strikethrough + red
		result.WriteString(content[actualIdx : actualIdx+len(searchTerm)]) // use original case
		result.WriteString("\033[0m")

		// Append bold green for the replacement
		result.WriteString("\033[1;32m") // bold + green
		result.WriteString(replacement)
		result.WriteString("\033[0m")

		lastIndex = actualIdx + len(searchTerm)
	}

	return result.String()
}

// extractSearchTerm extracts the actual search term from a Zoekt query.
// Handles quoted strings for expressions with spaces.
// For example:
//   - "hello world" -> "hello world" (quoted string)
//   - content:testerino lang:md repo:... -> "testerino"
//   - myFunction -> "myFunction"
func extractSearchTerm(query string) string {
	query = strings.TrimSpace(query)

	// Handle quoted strings (double or single quotes)
	if len(query) >= 2 {
		if (query[0] == '"' && query[len(query)-1] == '"') ||
			(query[0] == '\'' && query[len(query)-1] == '\'') {
			return query[1 : len(query)-1]
		}
	}

	// Check for content: prefix with possible quotes
	if idx := strings.Index(query, "content:"); idx >= 0 {
		rest := query[idx+8:] // skip "content:"

		// Handle quoted content
		if len(rest) > 0 && (rest[0] == '"' || rest[0] == '\'') {
			quote := rest[0]

			endIdx := strings.IndexByte(rest[1:], quote)
			if endIdx >= 0 {
				return rest[1 : endIdx+1]
			}
		}

		// Find end of term (space or end of string)
		endIdx := strings.IndexAny(rest, " \t")
		if endIdx == -1 {
			return rest
		}

		return rest[:endIdx]
	}

	// If no special prefix, collect all non-filter terms
	var terms []string

	inQuote := false

	var (
		quoteChar   byte
		currentTerm strings.Builder
	)

	for i := range len(query) {
		c := query[i]

		if !inQuote && (c == '"' || c == '\'') {
			inQuote = true
			quoteChar = c

			continue
		}

		if inQuote && c == quoteChar {
			inQuote = false

			if currentTerm.Len() > 0 {
				terms = append(terms, currentTerm.String())
				currentTerm.Reset()
			}

			continue
		}

		if !inQuote && (c == ' ' || c == '\t') {
			if currentTerm.Len() > 0 {
				term := currentTerm.String()
				// Only add non-filter terms
				if !strings.Contains(term, ":") {
					terms = append(terms, term)
				}

				currentTerm.Reset()
			}

			continue
		}

		currentTerm.WriteByte(c)
	}

	// Handle remaining term
	if currentTerm.Len() > 0 {
		term := currentTerm.String()
		if !strings.Contains(term, ":") {
			terms = append(terms, term)
		}
	}

	// Join terms with spaces
	if len(terms) > 0 {
		return strings.Join(terms, " ")
	}

	// Fallback to original query
	return query
}
