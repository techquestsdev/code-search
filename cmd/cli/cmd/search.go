package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/techquestsdev/code-search/cmd/cli/client"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search code across repositories",
	Long: `Search for code patterns across all indexed repositories.

Examples:
  code-search search "func main"
  code-search search "FOO" --repos myorg/myrepo --langs go`,
	Args: cobra.ExactArgs(1),
	RunE: runSearch,
}

func init() {
	rootCmd.AddCommand(searchCmd)

	searchCmd.Flags().BoolP("regex", "r", false, "Treat query as regex")
	searchCmd.Flags().
		StringSliceP("repos", "R", nil, "Filter by repositories (can specify multiple)")
	searchCmd.Flags().StringSliceP("langs", "l", nil, "Filter by languages (can specify multiple)")
	searchCmd.Flags().
		StringSliceP("files", "f", nil, "Filter by file patterns (can specify multiple)")
	searchCmd.Flags().IntP("limit", "n", 100, "Maximum number of results")
	searchCmd.Flags().Bool("case-sensitive", false, "Case-sensitive search")
	searchCmd.Flags().IntP("context", "C", 2, "Lines of context around match")
	searchCmd.Flags().
		Bool("stream", false, "Stream results as they arrive (faster time-to-first-result)")
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := args[0]
	output := viper.GetString("output")

	isRegex, _ := cmd.Flags().GetBool("regex")
	repos, _ := cmd.Flags().GetStringSlice("repos")
	langs, _ := cmd.Flags().GetStringSlice("langs")
	files, _ := cmd.Flags().GetStringSlice("files")
	limit, _ := cmd.Flags().GetInt("limit")
	caseSensitive, _ := cmd.Flags().GetBool("case-sensitive")
	contextLines, _ := cmd.Flags().GetInt("context")
	stream, _ := cmd.Flags().GetBool("stream")

	c := newClient()

	req := &client.SearchRequest{
		Query:         query,
		IsRegex:       isRegex,
		CaseSensitive: caseSensitive,
		Repos:         repos,
		Languages:     langs,
		FilePatterns:  files,
		Limit:         limit,
		ContextLines:  contextLines,
	}

	// Use streaming if requested
	if stream {
		return runStreamSearch(c, req, output)
	}

	resp, err := c.Search(context.Background(), req)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	// Format output
	switch output {
	case "json":
		return outputJSON(resp)
	default:
		return outputSearchText(resp)
	}
}

func outputSearchText(resp *client.SearchResponse) error {
	if len(resp.Results) == 0 {
		fmt.Println("No results found")
		return nil
	}

	fmt.Println(strings.Repeat("─", 60))

	// Group results by repo/file for better display
	currentFile := ""
	fileCount := 0
	seenFiles := make(map[string]bool)

	// Group consecutive same-file, same-line results
	type lineGroup struct {
		result client.SearchResult // first result (carries context/content)
		ranges [][2]int            // all highlight ranges
	}

	var groups []lineGroup

	for _, result := range resp.Results {
		fileKey := result.Repo + "/" + result.File

		last := len(groups) - 1
		if last >= 0 && groups[last].result.Repo+"/"+groups[last].result.File == fileKey && groups[last].result.Line == result.Line {
			groups[last].ranges = append(groups[last].ranges, [2]int{result.MatchStart, result.MatchEnd})
		} else {
			groups = append(groups, lineGroup{
				result: result,
				ranges: [][2]int{{result.MatchStart, result.MatchEnd}},
			})
		}
	}

	for _, g := range groups {
		fileKey := g.result.Repo + "/" + g.result.File
		if fileKey != currentFile {
			fmt.Printf("\n%s  %s\n", colorGreen(g.result.Repo), colorCyan(g.result.File))

			currentFile = fileKey
			if !seenFiles[fileKey] {
				seenFiles[fileKey] = true
				fileCount++
			}
		}
		// Print before context
		for i, line := range g.result.Context.Before {
			lineNum := g.result.Line - len(g.result.Context.Before) + i
			fmt.Printf("  %4d │ %s\n", lineNum, colorGray(line))
		}
		// Print matching line with all highlights
		fmt.Printf(
			"  %4d │ %s\n",
			g.result.Line,
			highlightMatches(g.result.Content, g.ranges),
		)
		// Print after context
		for i, line := range g.result.Context.After {
			lineNum := g.result.Line + i + 1
			fmt.Printf("  %4d │ %s\n", lineNum, colorGray(line))
		}
	}

	fmt.Println()
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf(
		"%s %d matches in %d files (took %s)\n",
		colorYellow("Summary:"),
		resp.TotalCount,
		fileCount,
		resp.Duration,
	)

	if resp.Truncated {
		fmt.Printf(
			"%s Results truncated. Use %s to see more.\n",
			colorCyan("Hint:"),
			colorYellow("--limit"),
		)
	}

	return nil
}

// Color helpers.
func colorGreen(s string) string {
	return "\033[32m" + s + "\033[0m"
}

func colorCyan(s string) string {
	return "\033[36m" + s + "\033[0m"
}

func colorGray(s string) string {
	return "\033[90m" + s + "\033[0m"
}

func colorYellow(s string) string {
	return "\033[33m" + s + "\033[0m"
}

// highlightMatches highlights multiple non-overlapping ranges in yellow/bold.
func highlightMatches(s string, ranges [][2]int) string {
	if len(ranges) == 0 {
		return s
	}

	// Sort by start position
	sorted := make([][2]int, len(ranges))
	copy(sorted, ranges)

	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j][0] < sorted[j-1][0]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}

	// Merge overlapping ranges
	merged := [][2]int{sorted[0]}
	for i := 1; i < len(sorted); i++ {
		last := &merged[len(merged)-1]
		if sorted[i][0] <= last[1] {
			if sorted[i][1] > last[1] {
				last[1] = sorted[i][1]
			}
		} else {
			merged = append(merged, sorted[i])
		}
	}

	var b strings.Builder

	cursor := 0

	for _, r := range merged {
		start, end := r[0], r[1]
		if start < 0 || end > len(s) || start >= end {
			continue
		}

		b.WriteString(s[cursor:start])
		b.WriteString("\033[1;33m")
		b.WriteString(s[start:end])
		b.WriteString("\033[0m")

		cursor = end
	}

	b.WriteString(s[cursor:])

	return b.String()
}

func outputJSON(data any) error {
	return outputJSONFormatted(data)
}

func outputJSONFormatted(data any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")

	return encoder.Encode(data)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	return s[:maxLen-3] + "..."
}

// runStreamSearch performs a streaming search and outputs results as they arrive.
func runStreamSearch(c *client.Client, req *client.SearchRequest, output string) error {
	// Track state for text output
	currentFile := ""
	fileCount := 0
	seenFiles := make(map[string]bool)
	headerPrinted := false

	var allResults []client.SearchResult

	// Use JSON mode for streaming JSON output
	if output == "json" {
		// For JSON output, collect all results and output at the end
		return c.SearchStream(
			context.Background(),
			req,
			func(event *client.SearchStreamEvent) error {
				switch event.Type {
				case "result":
					if event.Result != nil {
						allResults = append(allResults, *event.Result)
					}
				case "done":
					resp := &client.SearchResponse{
						Results:    allResults,
						TotalCount: event.TotalCount,
						Truncated:  event.Truncated,
						Duration:   event.Duration,
					}

					return outputJSON(resp)
				case "error":
					return fmt.Errorf("search error: %s", event.Error)
				}

				return nil
			},
		)
	}

	// Buffer for grouping same-line matches in streaming mode
	var (
		pendingResult *client.SearchResult
		pendingRanges [][2]int
	)

	flushPending := func() {
		if pendingResult == nil {
			return
		}
		// Print before context
		for i, line := range pendingResult.Context.Before {
			lineNum := pendingResult.Line - len(pendingResult.Context.Before) + i
			fmt.Printf("  %4d │ %s\n", lineNum, colorGray(line))
		}
		// Print matching line with all highlights
		fmt.Printf(
			"  %4d │ %s\n",
			pendingResult.Line,
			highlightMatches(pendingResult.Content, pendingRanges),
		)
		// Print after context
		for i, line := range pendingResult.Context.After {
			lineNum := pendingResult.Line + i + 1
			fmt.Printf("  %4d │ %s\n", lineNum, colorGray(line))
		}

		pendingResult = nil
		pendingRanges = nil
	}

	// Text output: stream results as they arrive
	return c.SearchStream(context.Background(), req, func(event *client.SearchStreamEvent) error {
		switch event.Type {
		case "result":
			if event.Result == nil {
				return nil
			}

			result := event.Result

			// Print header on first result
			if !headerPrinted {
				fmt.Println(strings.Repeat("─", 60))

				headerPrinted = true
			}

			fileKey := result.Repo + "/" + result.File

			// Check if this is a same-line match we can merge
			if pendingResult != nil && pendingResult.Repo+"/"+pendingResult.File == fileKey && pendingResult.Line == result.Line {
				pendingRanges = append(pendingRanges, [2]int{result.MatchStart, result.MatchEnd})
				return nil
			}

			// Flush previous buffered line
			flushPending()

			if fileKey != currentFile {
				fmt.Printf("\n%s  %s\n", colorGreen(result.Repo), colorCyan(result.File))

				currentFile = fileKey
				if !seenFiles[fileKey] {
					seenFiles[fileKey] = true
					fileCount++
				}
			}

			// Buffer this result
			pendingResult = result
			pendingRanges = [][2]int{{result.MatchStart, result.MatchEnd}}

		case "done":
			// Flush any remaining buffered line
			flushPending()

			if !headerPrinted {
				fmt.Println("No results found")
				return nil
			}

			fmt.Println()
			fmt.Println(strings.Repeat("─", 60))
			fmt.Printf(
				"%s %d matches in %d files (took %s)\n",
				colorYellow("Summary:"),
				event.TotalCount,
				fileCount,
				event.Duration,
			)

			if event.Truncated {
				fmt.Printf(
					"%s Results truncated. Use %s to see more.\n",
					colorCyan("Hint:"),
					colorYellow("--limit"),
				)
			}

		case "error":
			return fmt.Errorf("search error: %s", event.Error)
		}

		return nil
	})
}
