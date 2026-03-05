package search

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	v1 "github.com/sourcegraph/zoekt/grpc/protos/zoekt/webserver/v1"
	"github.com/sourcegraph/zoekt/query"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client communicates with the Zoekt webserver via gRPC.
type Client struct {
	conn   *grpc.ClientConn
	client v1.WebserverServiceClient
}

// NewClient creates a new Zoekt gRPC client.
func NewClient(zoektURL string) *Client {
	// Extract host:port from URL (e.g., "http://localhost:6070" -> "localhost:6070")
	address := strings.TrimPrefix(zoektURL, "http://")
	address = strings.TrimPrefix(address, "https://")
	address = strings.TrimSuffix(address, "/")

	conn, err := grpc.NewClient(address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(500*1024*1024), // 500MB
			grpc.MaxCallSendMsgSize(500*1024*1024), // 500MB
		),
	)
	if err != nil {
		// Log error but don't fail - client will retry on first use
		fmt.Printf("Warning: failed to create gRPC client: %v\n", err)
	}

	return &Client{
		conn:   conn,
		client: v1.NewWebserverServiceClient(conn),
	}
}

// Close closes the gRPC connection.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}

// Query represents a search query.
type Query struct {
	Pattern       string
	IsRegex       bool
	CaseSensitive bool
	Repos         []string
	Languages     []string
	FilePatterns  []string
	MaxResults    int
}

// Result represents a search result.
type Result struct {
	Repo     string   `json:"repo"`
	Branches []string `json:"branches"`
	Files    []File   `json:"files"`
}

// File represents a file with matches.
type File struct {
	Name     string  `json:"name"`
	Language string  `json:"language"`
	Matches  []Match `json:"matches"`
}

// Match represents a single match within a file.
type Match struct {
	LineNum  int    `json:"line_num"`
	Line     string `json:"line"`
	Before   string `json:"before,omitempty"`
	After    string `json:"after,omitempty"`
	Start    int    `json:"start"`
	End      int    `json:"end"`
	Fragment string `json:"fragment"`
}

// matchKey uniquely identifies a match position within a file.
type matchKey struct {
	LineNum int
	Start   int
	End     int
}

// mergeMatches appends newMatches to existing, skipping duplicates
// identified by (LineNum, Start, End).
func mergeMatches(existing, newMatches []Match) []Match {
	seen := make(map[matchKey]struct{}, len(existing))
	for _, m := range existing {
		seen[matchKey{m.LineNum, m.Start, m.End}] = struct{}{}
	}
	for _, m := range newMatches {
		key := matchKey{m.LineNum, m.Start, m.End}
		if _, dup := seen[key]; !dup {
			existing = append(existing, m)
			seen[key] = struct{}{}
		}
	}
	return existing
}

// SearchResponse represents the response from Zoekt.
type SearchResponse struct {
	Results      []Result      `json:"results"`
	Stats        Stats         `json:"stats"`
	Duration     time.Duration `json:"duration"`
	TotalMatches int           `json:"total_matches"`
}

// Stats contains search statistics.
type Stats struct {
	FilesConsidered int           `json:"files_considered"`
	FilesLoaded     int           `json:"files_loaded"`
	FilesSkipped    int           `json:"files_skipped"`
	ShardsScanned   int           `json:"shards_scanned"`
	Duration        time.Duration `json:"duration"`
}

// Search performs a search query against Zoekt via gRPC.
func (c *Client) Search(ctx context.Context, q Query) (*SearchResponse, error) {
	// Check if client is properly initialized
	if c.client == nil || c.conn == nil {
		return nil, errors.New("search client not initialized (Zoekt may not be ready)")
	}

	// Add timeout if context doesn't already have one
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
	}

	// Build the query
	query := c.buildQuery(q)

	req := &v1.SearchRequest{
		Query: query,
		Opts: &v1.SearchOptions{
			MaxMatchDisplayCount: int64(q.MaxResults),
			NumContextLines:      3,
			ChunkMatches:         true,
			TotalMaxMatchCount:   int64(q.MaxResults * 10),
		},
	}

	// Retry with backoff for connection issues (e.g., sidecar not ready yet)
	var (
		resp *v1.SearchResponse
		err  error
	)

	maxRetries := 3
	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err = c.client.Search(ctx, req)
		if err == nil {
			break
		}
		// Check if it's a connection error worth retrying
		if attempt < maxRetries && (strings.Contains(err.Error(), "connection") ||
			strings.Contains(err.Error(), "unavailable") ||
			strings.Contains(err.Error(), "refused")) {
			backoff := time.Duration(attempt+1) * 2 * time.Second
			fmt.Printf(
				"Zoekt search attempt %d failed, retrying in %v: %v\n",
				attempt+1,
				backoff,
				err,
			)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				continue
			}
		}

		return nil, fmt.Errorf("gRPC search failed: %w", err)
	}

	return c.convertResponse(resp), nil
}

// buildQuery builds a Zoekt query from the Query struct.
func (c *Client) buildQuery(q Query) *v1.Q {
	var children []*v1.Q

	// Check if user explicitly specified a branch filter
	hasBranchFilter := strings.Contains(q.Pattern, "branch:")

	// Add the main pattern query
	if q.Pattern != "" {
		if q.IsRegex {
			// Use regex query when explicitly requested
			children = append(children, &v1.Q{
				Query: &v1.Q_Regexp{
					Regexp: &v1.Regexp{
						Regexp:        q.Pattern,
						CaseSensitive: q.CaseSensitive,
						Content:       true,
					},
				},
			})
		} else if hasQuerySyntax(q.Pattern) {
			// Use zoekt's native query parser for patterns with operators
			// Supports: sym:main, branch:HEAD, lang:go, etc.
			parsedQuery, err := query.Parse(q.Pattern)
			if err == nil {
				// Apply case sensitivity to the parsed query if needed
				if q.CaseSensitive {
					parsedQuery = query.NewAnd(parsedQuery, &query.Const{Value: true})
				}
				// Convert to protobuf and use as main query
				protoQuery := query.QToProto(parsedQuery)
				children = append(children, protoQuery)
			} else {
				// Fallback to simple substring search if parsing fails
				children = append(children, &v1.Q{
					Query: &v1.Q_Substring{
						Substring: &v1.Substring{
							Pattern:       q.Pattern,
							CaseSensitive: q.CaseSensitive,
							Content:       true,
						},
					},
				})
			}
		} else {
			// Use exact substring matching for plain text patterns
			// This ensures "func main()" matches the exact string
			children = append(children, &v1.Q{
				Query: &v1.Q_Substring{
					Substring: &v1.Substring{
						Pattern:       q.Pattern,
						CaseSensitive: q.CaseSensitive,
						Content:       true,
					},
				},
			})
		}
	}

	// Add branch filter if user didn't explicitly specify one
	// This is needed for symbol searches (sym:) and other queries to work properly
	// Only skip branch filter if user explicitly specified branch: in their query
	if !hasBranchFilter {
		children = append(children, &v1.Q{
			Query: &v1.Q_Branch{
				Branch: &v1.Branch{
					Pattern: "HEAD",
				},
			},
		})
	}

	// Add repo filters - multiple repos should be OR'd together
	if len(q.Repos) > 0 {
		var repoChildren []*v1.Q

		for _, repo := range q.Repos {
			// If repo already looks like a regex (has anchors or regex chars), use as-is
			// Otherwise, treat it as a substring match
			repoPattern := repo
			if !strings.HasPrefix(repo, "^") && !strings.HasSuffix(repo, "$") &&
				!strings.ContainsAny(repo, "*+?()[]{}|\\") {
				// Escape dots and make it a substring match
				repoPattern = strings.ReplaceAll(repo, ".", "\\.")
				repoPattern = ".*" + repoPattern + ".*"
			}

			repoChildren = append(repoChildren, &v1.Q{
				Query: &v1.Q_Repo{
					Repo: &v1.Repo{
						Regexp: repoPattern,
					},
				},
			})
		}
		// If multiple repos, OR them together; otherwise just add the single repo
		if len(repoChildren) == 1 {
			children = append(children, repoChildren[0])
		} else {
			children = append(children, &v1.Q{
				Query: &v1.Q_Or{
					Or: &v1.Or{
						Children: repoChildren,
					},
				},
			})
		}
	}

	// Add language filters - multiple languages should be OR'd together
	if len(q.Languages) > 0 {
		var langChildren []*v1.Q
		for _, lang := range q.Languages {
			langChildren = append(langChildren, &v1.Q{
				Query: &v1.Q_Language{
					Language: &v1.Language{
						Language: lang,
					},
				},
			})
		}

		if len(langChildren) == 1 {
			children = append(children, langChildren[0])
		} else {
			children = append(children, &v1.Q{
				Query: &v1.Q_Or{
					Or: &v1.Or{
						Children: langChildren,
					},
				},
			})
		}
	}

	// Add file pattern filters - multiple patterns should be OR'd together
	if len(q.FilePatterns) > 0 {
		var fileChildren []*v1.Q

		for _, pattern := range q.FilePatterns {
			// Convert glob patterns to regex (e.g., *.go -> \.go$, **/*.ts -> .*\.ts$)
			regexPattern := globToRegex(pattern)
			fileChildren = append(fileChildren, &v1.Q{
				Query: &v1.Q_Regexp{
					Regexp: &v1.Regexp{
						Regexp:   regexPattern,
						FileName: true,
						Content:  false,
					},
				},
			})
		}

		if len(fileChildren) == 1 {
			children = append(children, fileChildren[0])
		} else {
			children = append(children, &v1.Q{
				Query: &v1.Q_Or{
					Or: &v1.Or{
						Children: fileChildren,
					},
				},
			})
		}
	}

	// Combine all queries with AND
	if len(children) == 1 {
		return children[0]
	}

	return &v1.Q{
		Query: &v1.Q_And{
			And: &v1.And{
				Children: children,
			},
		},
	}
}

// hasQuerySyntax checks if the pattern contains Zoekt query operators.
// Only includes operators that work with zoekt-git-index.
// Excludes: archived:, fork:, public: (require RawConfig metadata not set by zoekt-git-index)
// Excludes: short forms like r:, f:, b:, c:, t: (treated as literal text for predictable search)
func hasQuerySyntax(pattern string) bool {
	// Quoted phrases should go through Zoekt's native parser
	// which interprets quotes as exact phrase delimiters
	if strings.Contains(pattern, "\"") {
		return true
	}

	operators := []string{
		"sym:", "branch:", "lang:", "file:", "repo:",
		"case:", "content:", "regex:", "type:",
	}
	lower := strings.ToLower(pattern)
	for _, op := range operators {
		if strings.HasPrefix(lower, op) || strings.Contains(lower, " "+op) ||
			strings.HasPrefix(lower, "-"+op) || strings.Contains(lower, " -"+op) {
			return true
		}
	}
	return false
}

// globToRegex converts a glob pattern to a regex pattern
// Examples:
//   - "*.go" -> "\.go$"
//   - "**/*.ts" -> ".*\.ts$"
//   - "src/*.js" -> "src/[^/]*\.js$"
//   - "test" -> "test"
func globToRegex(glob string) string {
	// If it looks like it's already a regex, return as-is
	if strings.ContainsAny(glob, "^$()[]{}|+\\") && !strings.Contains(glob, "*") {
		return glob
	}

	var result strings.Builder

	i := 0
	for i < len(glob) {
		c := glob[i]
		switch c {
		case '*':
			if i+1 < len(glob) && glob[i+1] == '*' {
				// ** matches any path including /
				result.WriteString(".*")

				i += 2
				// Skip optional trailing /
				if i < len(glob) && glob[i] == '/' {
					i++
				}
			} else {
				// * matches any character except /
				result.WriteString("[^/]*")

				i++
			}
		case '?':
			result.WriteString("[^/]")

			i++
		case '.':
			result.WriteString("\\.")

			i++
		default:
			result.WriteByte(c)

			i++
		}
	}

	// Add end anchor if the pattern ends with a file extension
	pattern := result.String()
	if strings.HasSuffix(glob, "*") || strings.Contains(glob, ".") {
		if !strings.HasSuffix(pattern, "$") {
			pattern += "$"
		}
	}

	return pattern
}

// convertResponse converts gRPC response to our internal format.
func (c *Client) convertResponse(resp *v1.SearchResponse) *SearchResponse {
	results := make([]Result, 0)

	// Group files by repository and filename (to deduplicate cross-branch results)
	repoFiles := make(map[string]map[string]File) // repo -> filename -> File
	repoBranches := make(map[string][]string)

	for _, file := range resp.GetFiles() {
		repoName := file.GetRepository()
		if repoName == "" {
			continue
		}

		// Collect branches
		for _, branch := range file.GetBranches() {
			found := slices.Contains(repoBranches[repoName], branch)

			if !found {
				repoBranches[repoName] = append(repoBranches[repoName], branch)
			}
		}

		// Convert matches
		matches := make([]Match, 0)

		// Handle chunk matches
		for _, chunk := range file.GetChunkMatches() {
			content := string(chunk.GetContent())
			contentLines := strings.Split(content, "\n")
			// Remove trailing empty line from split
			if len(contentLines) > 0 && contentLines[len(contentLines)-1] == "" {
				contentLines = contentLines[:len(contentLines)-1]
			}

			contentStartLine := int(chunk.GetContentStart().GetLineNumber())

			for _, r := range chunk.GetRanges() {
				startLine := int(r.GetStart().GetLineNumber())
				// Find the line within the chunk
				lineIdx := startLine - contentStartLine

				line := ""
				if lineIdx >= 0 && lineIdx < len(contentLines) {
					line = contentLines[lineIdx]
				} else if len(contentLines) > 0 {
					line = contentLines[0]
				}

				// Extract before/after context from chunk content
				var beforeLines, afterLines []string

				// Collect before context (up to 3 lines before the match line)
				beforeStart := max(lineIdx-3, 0)

				for i := beforeStart; i < lineIdx && i < len(contentLines); i++ {
					beforeLines = append(beforeLines, contentLines[i])
				}

				// Collect after context (up to 3 lines after the match line)
				afterEnd := min(
					// +1 to skip match line, +3 for context
					lineIdx+4, len(contentLines))

				for i := lineIdx + 1; i < afterEnd && i < len(contentLines); i++ {
					afterLines = append(afterLines, contentLines[i])
				}

				// Zoekt provides 1-based column numbers
				startCol := int(r.GetStart().GetColumn())
				endCol := int(r.GetEnd().GetColumn())

				// If columns are 0, calculate position from byte offsets within the chunk
				if startCol == 0 || endCol == 0 {
					// Calculate the byte offset of the start of the current line within the chunk
					lineStartByteOffset := 0
					for i := 0; i < lineIdx && i < len(contentLines); i++ {
						lineStartByteOffset += len(contentLines[i]) + 1 // +1 for newline
					}

					chunkStartByte := int(chunk.GetContentStart().GetByteOffset())

					// Get the match byte offsets
					matchStartByte := int(r.GetStart().GetByteOffset())
					matchEndByte := int(r.GetEnd().GetByteOffset())

					// Calculate position relative to the line
					startCol = matchStartByte - chunkStartByte - lineStartByteOffset + 1 // +1 for 1-based
					endCol = matchEndByte - chunkStartByte - lineStartByteOffset + 1
				}

				// Ensure column values are valid for the line content
				lineLen := len(line)
				matchStart := startCol - 1
				matchEnd := endCol - 1

				// Clamp to valid range
				if matchStart < 0 {
					matchStart = 0
				}

				if matchStart > lineLen {
					matchStart = lineLen
				}

				if matchEnd < matchStart {
					matchEnd = matchStart
				}

				if matchEnd > lineLen {
					matchEnd = lineLen
				}

				match := Match{
					LineNum: startLine,
					Line:    line,
					Before:  strings.Join(beforeLines, "\n"),
					After:   strings.Join(afterLines, "\n"),
					Start:   matchStart,
					End:     matchEnd,
				}
				matches = append(matches, match)
			}
		}

		// Handle line matches (legacy format)
		for _, line := range file.GetLineMatches() {
			for _, frag := range line.GetLineFragments() {
				match := Match{
					LineNum: int(line.GetLineNumber()),
					Line:    string(line.GetLine()),
					Before:  string(line.GetBefore()),
					After:   string(line.GetAfter()),
					Start:   int(frag.GetLineOffset()),
					End:     int(frag.GetLineOffset()) + int(frag.GetMatchLength()),
				}
				matches = append(matches, match)
			}
		}

		if len(matches) > 0 {
			fileName := string(file.GetFileName())

			// Initialize map for this repo if needed
			if repoFiles[repoName] == nil {
				repoFiles[repoName] = make(map[string]File)
			}

			// Check if we already have this file (from another branch)
			if existingFile, exists := repoFiles[repoName][fileName]; exists {
				// Merge matches from different branches, deduplicating by position
				existingFile.Matches = mergeMatches(existingFile.Matches, matches)
				repoFiles[repoName][fileName] = existingFile
			} else {
				// First time seeing this file
				f := File{
					Name:     fileName,
					Language: file.GetLanguage(),
					Matches:  matches,
				}
				repoFiles[repoName][fileName] = f
			}
		}
	}

	// Build results - convert map of files to slice
	for repo, fileMap := range repoFiles {
		files := make([]File, 0, len(fileMap))
		for _, file := range fileMap {
			files = append(files, file)
		}

		results = append(results, Result{
			Repo:     repo,
			Branches: repoBranches[repo],
			Files:    files,
		})
	}

	stats := resp.GetStats()

	var duration time.Duration
	if stats.GetDuration() != nil {
		duration = stats.GetDuration().AsDuration()
	}

	return &SearchResponse{
		Results:      results,
		TotalMatches: int(stats.GetMatchCount()),
		Duration:     duration,
		Stats: Stats{
			FilesConsidered: int(stats.GetFilesConsidered()),
			FilesLoaded:     int(stats.GetFilesLoaded()),
			FilesSkipped:    int(stats.GetFilesSkipped()),
			ShardsScanned:   int(stats.GetShardsScanned()),
		},
	}
}

// RepoInfo represents information about an indexed repository.
type RepoInfo struct {
	Name     string   `json:"name"`
	Branches []string `json:"branches"`
	Files    int      `json:"files"`
	Shard    int      `json:"shard,omitempty"` // Which shard this repo is on (for sharded setups)
}

// ListRepos returns a list of indexed repositories.
func (c *Client) ListRepos(ctx context.Context) ([]RepoInfo, error) {
	// Add timeout if context doesn't already have one
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	req := &v1.ListRequest{
		Query: &v1.Q{
			Query: &v1.Q_Const{
				Const: true,
			},
		},
		Opts: &v1.ListOptions{
			Field: v1.ListOptions_REPO_LIST_FIELD_REPOS,
		},
	}

	resp, err := c.client.List(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("gRPC list failed: %w", err)
	}

	repos := make([]RepoInfo, 0)

	for _, entry := range resp.GetRepos() {
		repo := entry.GetRepository()
		if repo == nil {
			continue
		}

		branches := make([]string, 0)
		for _, branch := range repo.GetBranches() {
			branches = append(branches, branch.GetName())
		}

		stats := entry.GetStats()

		files := 0
		if stats != nil {
			files = int(stats.GetDocuments())
		}

		repos = append(repos, RepoInfo{
			Name:     repo.GetName(),
			Branches: branches,
			Files:    files,
		})
	}

	return repos, nil
}

// Health checks if the Zoekt service is healthy.
func (c *Client) Health(ctx context.Context) error {
	_, err := c.ListRepos(ctx)
	return err
}

// StreamSearchResult represents a single result or event from streaming search.
type StreamSearchResult struct {
	Result *Result // A single repository result (may contain multiple files/matches)
	Stats  *Stats  // Partial stats update (sent periodically)
	Done   bool    // Indicates stream is complete
	Error  error   // Error if something went wrong
}

// StreamSearch performs a streaming search using Zoekt's native gRPC streaming.
// Results are sent on the returned channel as they arrive from Zoekt.
// The channel is closed when the search is complete or an error occurs.
func (c *Client) StreamSearch(ctx context.Context, q Query) <-chan StreamSearchResult {
	results := make(chan StreamSearchResult, 100)

	go func() {
		defer close(results)

		// Check if client is properly initialized
		if c.client == nil || c.conn == nil {
			results <- StreamSearchResult{Error: errors.New("search client not initialized (Zoekt may not be ready)")}
			return
		}

		// Add timeout if context doesn't already have one
		searchCtx := ctx
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc

			searchCtx, cancel = context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()
		}

		_ = searchCtx // Used in gRPC call below

		// Build the query
		query := c.buildQuery(q)

		req := &v1.StreamSearchRequest{
			Request: &v1.SearchRequest{
				Query: query,
				Opts: &v1.SearchOptions{
					MaxMatchDisplayCount: int64(q.MaxResults),
					NumContextLines:      3,
					ChunkMatches:         true,
					TotalMaxMatchCount:   int64(q.MaxResults * 10),
				},
			},
		}

		// Start streaming search
		stream, err := c.client.StreamSearch(ctx, req)
		if err != nil {
			results <- StreamSearchResult{Error: fmt.Errorf("failed to start stream search: %w", err)}
			return
		}

		// Track total matches for stats
		var (
			totalMatches int
			totalStats   Stats
		)

		for {
			resp, err := stream.Recv()
			if err != nil {
				if err.Error() == "EOF" || strings.Contains(err.Error(), "EOF") {
					// Stream completed normally
					results <- StreamSearchResult{
						Stats: &totalStats,
						Done:  true,
					}

					return
				}

				results <- StreamSearchResult{Error: fmt.Errorf("stream recv error: %w", err)}

				return
			}

			// Get the response chunk
			chunk := resp.GetResponseChunk()
			if chunk == nil {
				continue
			}

			// Update stats
			if stats := chunk.GetStats(); stats != nil {
				totalStats.FilesConsidered += int(stats.GetFilesConsidered())
				totalStats.FilesLoaded += int(stats.GetFilesLoaded())
				totalStats.FilesSkipped += int(stats.GetFilesSkipped())
				totalStats.ShardsScanned += int(stats.GetShardsScanned())

				totalMatches += int(stats.GetMatchCount())
				if stats.GetDuration() != nil {
					totalStats.Duration = stats.GetDuration().AsDuration()
				}
			}

			// Process file matches from this chunk
			files := chunk.GetFiles()
			if len(files) == 0 {
				// Stats-only update
				continue
			}

			// Convert to our Result format - group by repo and deduplicate by filename
			repoFiles := make(map[string]map[string]File) // repo -> filename -> File
			repoBranches := make(map[string][]string)

			for _, file := range files {
				repoName := file.GetRepository()
				if repoName == "" {
					continue
				}

				// Collect branches
				for _, branch := range file.GetBranches() {
					if !slices.Contains(repoBranches[repoName], branch) {
						repoBranches[repoName] = append(repoBranches[repoName], branch)
					}
				}

				// Convert matches using existing logic
				matches := c.convertFileMatches(file)
				if len(matches) > 0 {
					fileName := string(file.GetFileName())

					// Initialize map for this repo if needed
					if repoFiles[repoName] == nil {
						repoFiles[repoName] = make(map[string]File)
					}

					// Check if we already have this file (from another branch)
					if existingFile, exists := repoFiles[repoName][fileName]; exists {
						// Merge matches from different branches
						existingFile.Matches = mergeMatches(existingFile.Matches, matches)
						repoFiles[repoName][fileName] = existingFile
					} else {
						// First time seeing this file
						f := File{
							Name:     fileName,
							Language: file.GetLanguage(),
							Matches:  matches,
						}
						repoFiles[repoName][fileName] = f
					}
				}
			}

			// Send each repo as a separate result for true streaming
			for repo, fileMap := range repoFiles {
				// Convert map to slice
				repoFileList := make([]File, 0, len(fileMap))
				for _, f := range fileMap {
					repoFileList = append(repoFileList, f)
				}

				result := &Result{
					Repo:     repo,
					Branches: repoBranches[repo],
					Files:    repoFileList,
				}
				select {
				case results <- StreamSearchResult{Result: result}:
				case <-ctx.Done():
					results <- StreamSearchResult{Error: ctx.Err()}
					return
				}
			}
		}
	}()

	return results
}

// convertFileMatches extracts matches from a file match protobuf.
func (c *Client) convertFileMatches(file *v1.FileMatch) []Match {
	matches := make([]Match, 0)

	// Handle chunk matches
	for _, chunk := range file.GetChunkMatches() {
		content := string(chunk.GetContent())
		contentLines := strings.Split(content, "\n")
		// Remove trailing empty line from split
		if len(contentLines) > 0 && contentLines[len(contentLines)-1] == "" {
			contentLines = contentLines[:len(contentLines)-1]
		}

		contentStartLine := int(chunk.GetContentStart().GetLineNumber())

		for _, r := range chunk.GetRanges() {
			startLine := int(r.GetStart().GetLineNumber())
			lineIdx := startLine - contentStartLine

			line := ""
			if lineIdx >= 0 && lineIdx < len(contentLines) {
				line = contentLines[lineIdx]
			} else if len(contentLines) > 0 {
				line = contentLines[0]
			}

			// Extract before/after context
			var beforeLines, afterLines []string

			beforeStart := max(lineIdx-3, 0)
			for i := beforeStart; i < lineIdx && i < len(contentLines); i++ {
				beforeLines = append(beforeLines, contentLines[i])
			}

			afterEnd := min(lineIdx+4, len(contentLines))
			for i := lineIdx + 1; i < afterEnd && i < len(contentLines); i++ {
				afterLines = append(afterLines, contentLines[i])
			}

			startCol := int(r.GetStart().GetColumn())
			endCol := int(r.GetEnd().GetColumn())

			if startCol == 0 || endCol == 0 {
				lineStartByteOffset := 0
				for i := 0; i < lineIdx && i < len(contentLines); i++ {
					lineStartByteOffset += len(contentLines[i]) + 1
				}

				chunkStartByte := int(chunk.GetContentStart().GetByteOffset())
				matchStartByte := int(r.GetStart().GetByteOffset())
				matchEndByte := int(r.GetEnd().GetByteOffset())
				startCol = matchStartByte - chunkStartByte - lineStartByteOffset + 1
				endCol = matchEndByte - chunkStartByte - lineStartByteOffset + 1
			}

			lineLen := len(line)
			matchStart := startCol - 1
			matchEnd := endCol - 1

			if matchStart < 0 {
				matchStart = 0
			}

			if matchStart > lineLen {
				matchStart = lineLen
			}

			if matchEnd < matchStart {
				matchEnd = matchStart
			}

			if matchEnd > lineLen {
				matchEnd = lineLen
			}

			match := Match{
				LineNum: startLine,
				Line:    line,
				Before:  strings.Join(beforeLines, "\n"),
				After:   strings.Join(afterLines, "\n"),
				Start:   matchStart,
				End:     matchEnd,
			}
			matches = append(matches, match)
		}
	}

	// Handle line matches (legacy format)
	for _, line := range file.GetLineMatches() {
		for _, frag := range line.GetLineFragments() {
			match := Match{
				LineNum: int(line.GetLineNumber()),
				Line:    string(line.GetLine()),
				Before:  string(line.GetBefore()),
				After:   string(line.GetAfter()),
				Start:   int(frag.GetLineOffset()),
				End:     int(frag.GetLineOffset()) + int(frag.GetMatchLength()),
			}
			matches = append(matches, match)
		}
	}

	return matches
}
