package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/techquestsdev/code-search/internal/audit"
	"github.com/techquestsdev/code-search/internal/metrics"
	"github.com/techquestsdev/code-search/internal/middleware"
	"github.com/techquestsdev/code-search/internal/regexutil"
	"github.com/techquestsdev/code-search/internal/search"
	"github.com/techquestsdev/code-search/internal/tracing"
)

// Search handles synchronous search requests.
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracing.StartSpan(r.Context(), "search.execute")
	defer span.End()

	var req search.SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate regex patterns to prevent ReDoS
	if req.IsRegex {
		if err := regexutil.ValidatePattern(req.Query); err != nil {
			http.Error(w, "Invalid regex pattern: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	// Set defaults (limit=0 means unlimited, handled by service layer)
	if req.ContextLines == 0 {
		req.ContextLines = 2
	}

	// Add search attributes to span
	searchType := "text"
	if req.IsRegex {
		searchType = "regex"
	}

	tracing.SetAttributes(ctx,
		tracing.AttrSearchQuery.String(req.Query),
		tracing.AttrSearchType.String(searchType),
		attribute.Int("search.limit", req.Limit),
		attribute.Bool("search.case_sensitive", req.CaseSensitive),
	)

	// Execute search via Zoekt
	start := time.Now()
	results, err := h.services.Search.Search(ctx, req)
	duration := time.Since(start)

	if err != nil {
		metrics.RecordSearch(searchType, duration, 0)
		metrics.RecordError("search", "search_failed")
		tracing.RecordError(ctx, err)
		http.Error(w, "Search failed: "+err.Error(), http.StatusInternalServerError)

		return
	}

	// Apply enterprise search result filtering if configured
	if h.services.SearchResultFilter != nil {
		results.Results = h.services.SearchResultFilter(ctx, results.Results)
		results.TotalCount = len(results.Results)
	}

	// Record metrics
	metrics.RecordSearch(searchType, duration, len(results.Results))
	tracing.SetAttributes(ctx, tracing.AttrResultCount.Int(len(results.Results)))
	tracing.SetOK(ctx)

	// Audit log the search
	user := middleware.UserFromContext(ctx)
	h.services.AuditLogger.LogSearch(ctx, audit.SearchEvent{
		UserID:    user.ID,
		Query:     req.Query,
		Results:   len(results.Results),
		Duration:  duration,
		ClientIP:  r.RemoteAddr,
		Timestamp: start,
	})

	// Create response with formatted duration
	response := map[string]any{
		"results":     results.Results,
		"total_count": results.TotalCount,
		"truncated":   results.Truncated,
		"duration":    results.Duration.String(),
		"stats":       results.Stats,
	}

	writeJSON(w, response)
}

// SearchStream handles streaming search requests via SSE.
func (h *Handler) SearchStream(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req search.SearchRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate regex patterns to prevent ReDoS
	if req.IsRegex {
		if err := regexutil.ValidatePattern(req.Query); err != nil {
			http.Error(w, "Invalid regex pattern: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	// Check if search service is available
	if h.services == nil || h.services.Search == nil {
		http.Error(w, "Search service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Set up SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering for SSE

	// Use ResponseController (Go 1.20+) to get Flush capability
	// This works through middleware wrappers that break direct type assertion
	rc := http.NewResponseController(w)

	// Helper to flush - uses ResponseController
	flush := func() {
		if err := rc.Flush(); err != nil {
			// Flush failed, but we can continue - client may have disconnected
			_ = err
		}
	}

	// Send progress event
	event := map[string]any{
		"type":    "progress",
		"message": "Searching...",
	}
	data, _ := json.Marshal(event)

	w.Write([]byte("data: "))
	w.Write(data)
	w.Write([]byte("\n\n"))
	flush()

	// Use true streaming if enabled, otherwise fall back to batch-then-stream
	start := time.Now()

	var totalResults int

	if h.searchConfig.EnableStreaming {
		totalResults = h.streamSearchTrueRC(ctx, w, flush, req)
	} else {
		totalResults = h.streamSearchBatchRC(ctx, w, flush, req)
	}

	// Audit log the streaming search
	duration := time.Since(start)
	user := middleware.UserFromContext(ctx)
	h.services.AuditLogger.LogSearch(ctx, audit.SearchEvent{
		UserID:    user.ID,
		Query:     req.Query,
		Results:   totalResults,
		Duration:  duration,
		ClientIP:  r.RemoteAddr,
		Timestamp: start,
	})
}

// streamSearchTrueRC uses Zoekt's native streaming with ResponseController flush.
// Returns the total number of results streamed.
func (h *Handler) streamSearchTrueRC(
	ctx context.Context,
	w http.ResponseWriter,
	flush func(),
	req search.SearchRequest,
) int {
	streamCh := h.services.Search.StreamSearch(ctx, req)

	var (
		totalCount                   int
		filesSearched, reposSearched int
	)

	startTime := time.Now()

	for event := range streamCh {
		// Check if client disconnected
		select {
		case <-ctx.Done():
			return totalCount
		default:
		}

		if event.Error != nil {
			errEvent := map[string]any{
				"type":  "error",
				"error": event.Error.Error(),
			}
			errData, _ := json.Marshal(errEvent)

			w.Write([]byte("data: "))
			w.Write(errData)
			w.Write([]byte("\n\n"))
			flush()

			continue
		}

		// Accumulate stats from stream events
		if event.Stats != nil {
			filesSearched = event.Stats.FilesSearched
			reposSearched = event.Stats.ReposSearched
		}

		if event.Result != nil {
			// Apply enterprise search result filtering if configured
			if h.services.SearchResultFilter != nil {
				filtered := h.services.SearchResultFilter(ctx, []search.SearchResult{*event.Result})
				if len(filtered) == 0 {
					continue
				}
			}

			totalCount++
			resultEvent := map[string]any{
				"type":   "result",
				"result": event.Result,
			}
			resultData, _ := json.Marshal(resultEvent)

			w.Write([]byte("data: "))
			w.Write(resultData)
			w.Write([]byte("\n\n"))
			flush()
		}

		if event.Done {
			// Determine if results were truncated
			limit := req.Limit
			if limit == 0 {
				limit = 10000 // Default limit
			}

			truncated := totalCount >= limit

			doneEvent := map[string]any{
				"type":        "done",
				"total_count": totalCount,
				"truncated":   truncated,
				"duration":    time.Since(startTime).String(),
				"stats": map[string]any{
					"files_searched": filesSearched,
					"repos_searched": reposSearched,
				},
			}
			doneData, _ := json.Marshal(doneEvent)

			w.Write([]byte("data: "))
			w.Write(doneData)
			w.Write([]byte("\n\n"))
			flush()

			return totalCount
		}
	}

	return totalCount
}

// streamSearchBatchRC fetches all results first, then streams them with ResponseController flush.
// Returns the total number of results streamed.
func (h *Handler) streamSearchBatchRC(
	ctx context.Context,
	w http.ResponseWriter,
	flush func(),
	req search.SearchRequest,
) int {
	// Execute search
	results, err := h.services.Search.Search(ctx, req)

	// Apply enterprise search result filtering if configured
	if err == nil && h.services.SearchResultFilter != nil {
		results.Results = h.services.SearchResultFilter(ctx, results.Results)
		results.TotalCount = len(results.Results)
	}

	if err != nil {
		errEvent := map[string]any{
			"type":  "error",
			"error": err.Error(),
		}
		errData, _ := json.Marshal(errEvent)

		w.Write([]byte("data: "))
		w.Write(errData)
		w.Write([]byte("\n\n"))
		flush()

		return 0
	}

	// Stream results - check context between writes to handle client disconnects
	for i, result := range results.Results {
		// Check if client disconnected
		select {
		case <-ctx.Done():
			return i
		default:
		}

		resultEvent := map[string]any{
			"type":   "result",
			"result": result,
		}
		resultData, _ := json.Marshal(resultEvent)

		w.Write([]byte("data: "))
		w.Write(resultData)
		w.Write([]byte("\n\n"))
		flush()
	}

	// Send done event
	doneEvent := map[string]any{
		"type":        "done",
		"total_count": results.TotalCount,
		"truncated":   results.Truncated,
		"duration":    results.Duration.String(),
		"stats": map[string]any{
			"files_searched": results.Stats.FilesSearched,
			"repos_searched": results.Stats.ReposSearched,
		},
	}
	doneData, _ := json.Marshal(doneEvent)

	w.Write([]byte("data: "))
	w.Write(doneData)
	w.Write([]byte("\n\n"))
	flush()

	return len(results.Results)
}

// escapeRepoName escapes special regex characters and adds anchors for exact matching.
func escapeRepoName(name string) string {
	// Characters that need escaping in regex
	special := []string{"\\", ".", "+", "*", "?", "^", "$", "(", ")", "[", "]", "{", "}", "|"}

	escaped := name
	for _, char := range special {
		escaped = strings.ReplaceAll(escaped, char, "\\"+char)
	}
	// Add anchors for exact match
	return "^" + escaped + "$"
}

// buildFullRepoName extracts the host from clone_url and combines with repo name
// e.g., clone_url="https://github.com/org/repo.git", name="org/repo" -> "github.com/org/repo".
func buildFullRepoName(cloneURL, name string) string {
	// Remove protocol prefix
	url := strings.TrimPrefix(cloneURL, "https://")
	url = strings.TrimPrefix(url, "http://")

	// Extract host (everything before the first /)
	parts := strings.SplitN(url, "/", 2)
	if len(parts) < 1 {
		return name
	}

	host := parts[0]

	return host + "/" + name
}

// SearchSuggestionsResponse contains search suggestions for autocomplete.
type SearchSuggestionsResponse struct {
	Repos     []RepoSuggestion     `json:"repos"`
	Languages []LanguageSuggestion `json:"languages"`
	Filters   []FilterSuggestion   `json:"filters"`
}

// RepoSuggestion represents a repository suggestion.
type RepoSuggestion struct {
	Name        string `json:"name"`         // Escaped regex pattern like "^github\\.com/org/repo$"
	DisplayName string `json:"display_name"` // Human-readable name like "github.com/org/repo"
	FullName    string `json:"full_name"`    // Full name from clone_url
	Status      string `json:"status"`       // Index status
}

// LanguageSuggestion represents a language suggestion.
type LanguageSuggestion struct {
	Name string `json:"name"`
}

// FilterSuggestion represents a filter keyword suggestion.
type FilterSuggestion struct {
	Keyword     string `json:"keyword"`
	Description string `json:"description"`
	Example     string `json:"example"`
}

// SearchSuggestions returns autocomplete suggestions for the search bar.
func (h *Handler) SearchSuggestions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get all repos
	repos, err := h.services.Repos.ListRepositories(ctx, nil)
	if err != nil {
		http.Error(w, "Failed to fetch repos: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Build repo suggestions with escaped regex patterns for exact matching
	repoSuggestions := make([]RepoSuggestion, 0, len(repos))
	for _, repo := range repos {
		// Build full repo name including host (e.g., github.com/org/repo)
		fullRepoName := buildFullRepoName(repo.CloneURL, repo.Name)
		// Escape special regex characters and add anchors for exact match
		escapedName := escapeRepoName(fullRepoName)
		repoSuggestions = append(repoSuggestions, RepoSuggestion{
			Name:        escapedName,
			DisplayName: fullRepoName,
			FullName:    repo.CloneURL,
			Status:      repo.IndexStatus,
		})
	}

	// Common languages in code search
	// Note: These are Zoekt language names, NOT file extensions
	// e.g., "go" (not "*.go"), "terraform" (not "tf")
	languages := []LanguageSuggestion{
		{Name: "go"},
		{Name: "typescript"},
		{Name: "javascript"},
		{Name: "python"},
		{Name: "java"},
		{Name: "c"},
		{Name: "cpp"},
		{Name: "csharp"},
		{Name: "rust"},
		{Name: "ruby"},
		{Name: "php"},
		{Name: "swift"},
		{Name: "kotlin"},
		{Name: "scala"},
		{Name: "haskell"},
		{Name: "lua"},
		{Name: "perl"},
		{Name: "r"},
		{Name: "elixir"},
		{Name: "erlang"},
		{Name: "clojure"},
		{Name: "ocaml"},
		{Name: "fsharp"},
		{Name: "dart"},
		{Name: "groovy"},
		{Name: "terraform"},
		{Name: "hcl"},
		{Name: "markdown"},
		{Name: "yaml"},
		{Name: "json"},
		{Name: "toml"},
		{Name: "xml"},
		{Name: "html"},
		{Name: "css"},
		{Name: "scss"},
		{Name: "sass"},
		{Name: "less"},
		{Name: "sql"},
		{Name: "graphql"},
		{Name: "protobuf"},
		{Name: "thrift"},
		{Name: "shell"},
		{Name: "bash"},
		{Name: "powershell"},
		{Name: "dockerfile"},
		{Name: "makefile"},
		{Name: "cmake"},
		{Name: "nix"},
		{Name: "vim"},
		{Name: "zig"},
		{Name: "assembly"},
		{Name: "objective-c"},
		{Name: "vue"},
		{Name: "svelte"},
		{Name: "jsx"},
		{Name: "tsx"},
	}

	// Filter keywords - only operators that work with zoekt-git-index
	filters := []FilterSuggestion{
		{Keyword: "repo:", Description: "Filter by repository", Example: "repo:org/repo"},
		{Keyword: "file:", Description: "Filter by file path pattern", Example: "file:*.go"},
		{Keyword: "lang:", Description: "Filter by language name", Example: "lang:typescript"},
		{Keyword: "case:yes", Description: "Case sensitive search", Example: "case:yes func"},
		{
			Keyword:     "case:no",
			Description: "Case insensitive search (default)",
			Example:     "case:no FOO",
		},
		{Keyword: "sym:", Description: "Search for symbols/definitions", Example: "sym:main"},
		{
			Keyword:     "content:",
			Description: "Search content only (not file names)",
			Example:     "content:FOO",
		},
		{Keyword: "branch:", Description: "Filter by branch/tag", Example: "branch:main"},
		{Keyword: "type:", Description: "Result type: filematch, filename, or repo", Example: "type:filename main"},
		{Keyword: "regex:", Description: "Treat pattern as regex", Example: "regex:func\\s+main"},
		{Keyword: "-repo:", Description: "Exclude repository", Example: "-repo:test"},
		{Keyword: "-file:", Description: "Exclude file pattern", Example: "-file:*_test.go"},
		{Keyword: "-lang:", Description: "Exclude language", Example: "-lang:markdown"},
		{Keyword: "-content:", Description: "Exclude content pattern", Example: "-content:TODO"},
	}

	response := SearchSuggestionsResponse{
		Repos:     repoSuggestions,
		Languages: languages,
		Filters:   filters,
	}

	writeJSON(w, response)
}
