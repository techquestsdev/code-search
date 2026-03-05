package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/techquestsdev/code-search/internal/audit"
	"github.com/techquestsdev/code-search/internal/middleware"
	"github.com/techquestsdev/code-search/internal/queue"
	"github.com/techquestsdev/code-search/internal/replace"
	"github.com/techquestsdev/code-search/internal/search"
)

// ReplacePreviewRequest represents a request to preview replacements.
type ReplacePreviewRequest struct {
	SearchPattern string   `json:"search_pattern"`
	ReplaceWith   string   `json:"replace_with"`
	IsRegex       bool     `json:"is_regex"`
	CaseSensitive bool     `json:"case_sensitive"`
	FilePatterns  []string `json:"file_patterns,omitempty"`
	Repos         []string `json:"repos,omitempty"`
	Languages     []string `json:"languages,omitempty"`
	Limit         *int     `json:"limit,omitempty"`         // Max results: nil/omit for default (1000), 0 for unlimited
	ContextLines  int      `json:"context_lines,omitempty"` // Lines of context around matches (default: 2)
}

// PreviewMatch represents a single match in the preview response.
type PreviewMatch struct {
	RepositoryID       int64         `json:"repository_id"`
	Repo               string        `json:"repo"`
	File               string        `json:"file"`
	Language           string        `json:"language"`
	Line               int           `json:"line"`
	Column             int           `json:"column"`
	Content            string        `json:"content"`
	MatchStart         int           `json:"match_start"`
	MatchEnd           int           `json:"match_end"`
	Context            ResultContext `json:"context"`
	ConnectionID       *int64        `json:"connection_id,omitempty"`
	ConnectionName     string        `json:"connection_name,omitempty"`
	ConnectionHasToken bool          `json:"connection_has_token"`
}

// ResultContext represents lines around a match.
type ResultContext struct {
	Before []string `json:"before"`
	After  []string `json:"after"`
}

// ReplacePreview shows what would be replaced without making changes.
func (h *Handler) ReplacePreview(w http.ResponseWriter, r *http.Request) {
	var req ReplacePreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Limit behavior:
	// - nil/omitted: default to 1000
	// - 0: unlimited (search service defaults to 10000)
	// - >0: explicit limit
	limit := 1000 // Default
	if req.Limit != nil {
		limit = *req.Limit
	}

	// Context lines default
	contextLines := req.ContextLines
	if contextLines == 0 {
		contextLines = 2
	}

	opts := replace.ReplaceOptions{
		SearchPattern: req.SearchPattern,
		ReplaceWith:   req.ReplaceWith,
		IsRegex:       req.IsRegex,
		CaseSensitive: req.CaseSensitive,
		FilePatterns:  req.FilePatterns,
		Languages:     req.Languages,
		ContextLines:  contextLines,
		DryRun:        true,
		Limit:         limit,
	}

	results, err := h.services.Replace.Preview(r.Context(), opts)
	if err != nil {
		http.Error(w, "Preview failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// RBAC: filter results to only repos the user can access
	user := middleware.UserFromContext(r.Context())
	repoNameSet := make(map[string]struct{})
	for _, res := range results.Results {
		repoNameSet[res.Repo] = struct{}{}
	}
	repoNames := make([]string, 0, len(repoNameSet))
	for name := range repoNameSet {
		repoNames = append(repoNames, name)
	}
	allowedNames := h.services.Authorizer.FilterRepos(r.Context(), user, repoNames)
	allowedSet := make(map[string]bool, len(allowedNames))
	for _, name := range allowedNames {
		allowedSet[name] = true
	}
	filteredResults := make([]search.SearchResult, 0, len(results.Results))
	for _, res := range results.Results {
		if allowedSet[res.Repo] {
			filteredResults = append(filteredResults, res)
		}
	}
	results.Results = filteredResults

	// Build preview response with match details including repository_id
	matches := make([]PreviewMatch, 0, len(results.Results))
	// Cache connection info by connection ID
	type connInfo struct {
		name     string
		hasToken bool
	}

	connectionCache := make(map[int64]connInfo)

	for _, res := range results.Results {
		// Look up repository ID by name
		var (
			repoID         int64
			connectionID   *int64
			connectionName string
			hasToken       bool
		)

		repo, err := h.services.Repos.FindRepositoryByZoektName(r.Context(), res.Repo)
		if err == nil && repo != nil {
			repoID = repo.ID
			connectionID = &repo.ConnectionID
			// Check if connection info is cached
			if cached, ok := connectionCache[repo.ConnectionID]; ok {
				connectionName = cached.name
				hasToken = cached.hasToken
			} else {
				conn, err := h.services.Repos.GetConnection(r.Context(), repo.ConnectionID)
				if err == nil && conn != nil {
					connectionName = conn.Name
					// In read-only mode, DB tokens cannot be used for replace operations
					// (they're reserved for indexing only), so report hasToken as false
					// to require user-provided tokens
					if h.reposReadOnly {
						hasToken = false
					} else {
						hasToken = conn.Token != ""
					}
				}

				connectionCache[repo.ConnectionID] = connInfo{name: connectionName, hasToken: hasToken}
			}
		}

		matches = append(matches, PreviewMatch{
			RepositoryID: repoID,
			Repo:         res.Repo,
			File:         res.File,
			Language:     res.Language,
			Line:         res.Line,
			Column:       res.Column,
			Content:      res.Content,
			MatchStart:   res.MatchStart,
			MatchEnd:     res.MatchEnd,
			Context: ResultContext{
				Before: res.Context.Before,
				After:  res.Context.After,
			},
			ConnectionID:       connectionID,
			ConnectionName:     connectionName,
			ConnectionHasToken: hasToken,
		})
	}

	response := map[string]any{
		"matches":     matches,
		"total_count": results.TotalCount,
		"truncated":   results.Truncated,
		"limit":       limit,
		"duration":    results.Duration.String(),
		"stats": map[string]int{
			"files_searched": results.Stats.FilesSearched,
			"repos_searched": results.Stats.ReposSearched,
		},
	}

	writeJSON(w, response)
}

// ReplaceMatch represents a pre-computed match from preview.
type ReplaceMatch struct {
	RepositoryID   int64  `json:"repository_id"`
	RepositoryName string `json:"repository_name"`
	FilePath       string `json:"file_path"`
}

// ReplaceExecuteRequest represents a request to execute replacements.
type ReplaceExecuteRequest struct {
	// Search parameters (kept for reference/logging, not used for search)
	SearchPattern string   `json:"search_pattern"`
	ReplaceWith   string   `json:"replace_with"`
	IsRegex       bool     `json:"is_regex"`
	CaseSensitive bool     `json:"case_sensitive"`
	FilePatterns  []string `json:"file_patterns,omitempty"`

	// Matches from preview (required - Execute uses these directly)
	Matches []ReplaceMatch `json:"matches"`

	// MR options (MR is always created to avoid direct main branch changes)
	BranchName    string `json:"branch_name,omitempty"`
	MRTitle       string `json:"mr_title,omitempty"`
	MRDescription string `json:"mr_description,omitempty"`

	// User-provided tokens for repos without server-side authentication
	// Map of connection_id -> token (allows different tokens per code host)
	UserTokens map[string]string `json:"user_tokens,omitempty"`
}

// ReplaceExecute executes replacements across repositories.
func (h *Handler) ReplaceExecute(w http.ResponseWriter, r *http.Request) {
	var req ReplaceExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate: matches are required (must call preview first)
	if len(req.Matches) == 0 {
		http.Error(
			w,
			"'matches' is required - call preview endpoint first to get matches",
			http.StatusBadRequest,
		)

		return
	}

	// RBAC: filter matches to only repos the user can access
	user := middleware.UserFromContext(r.Context())
	repoNameSet := make(map[string]struct{})
	for _, m := range req.Matches {
		repoNameSet[m.RepositoryName] = struct{}{}
	}
	repoNames := make([]string, 0, len(repoNameSet))
	for name := range repoNameSet {
		repoNames = append(repoNames, name)
	}
	allowedNames := h.services.Authorizer.FilterRepos(r.Context(), user, repoNames)
	allowedSet := make(map[string]bool, len(allowedNames))
	for _, name := range allowedNames {
		allowedSet[name] = true
	}
	filteredMatches := make([]ReplaceMatch, 0, len(req.Matches))
	for _, m := range req.Matches {
		if allowedSet[m.RepositoryName] {
			filteredMatches = append(filteredMatches, m)
		}
	}
	if len(filteredMatches) == 0 {
		http.Error(w, "Forbidden: no accessible repos in matches", http.StatusForbidden)
		return
	}
	req.Matches = filteredMatches

	// Convert matches to queue format
	var queueMatches []queue.ReplaceMatch
	for _, m := range req.Matches {
		queueMatches = append(queueMatches, queue.ReplaceMatch{
			RepositoryID:   m.RepositoryID,
			RepositoryName: m.RepositoryName,
			FilePath:       m.FilePath,
		})
	}

	// Queue the replace job
	payload := queue.ReplacePayload{
		SearchPattern: req.SearchPattern,
		ReplaceWith:   req.ReplaceWith,
		IsRegex:       req.IsRegex,
		CaseSensitive: req.CaseSensitive,
		FilePatterns:  req.FilePatterns,
		Matches:       queueMatches,
		BranchName:    req.BranchName,
		MRTitle:       req.MRTitle,
		MRDescription: req.MRDescription,
		UserTokens:    req.UserTokens,
		ReposReadOnly: h.reposReadOnly,
	}

	job, err := h.services.Queue.Enqueue(r.Context(), queue.JobTypeReplace, payload)
	if err != nil {
		http.Error(w, "Failed to queue replace job: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Audit log the replace operation
	uniqueRepos := make(map[string]struct{})
	for _, m := range req.Matches {
		uniqueRepos[m.RepositoryName] = struct{}{}
	}
	h.services.AuditLogger.LogReplace(r.Context(), audit.ReplaceEvent{
		UserID:     user.ID,
		Query:      req.SearchPattern,
		ReposCount: len(uniqueRepos),
		FilesCount: len(req.Matches),
		ClientIP:   r.RemoteAddr,
		Timestamp:  time.Now(),
	})

	response := map[string]any{
		"job_id":  job.ID,
		"status":  job.Status,
		"message": "Replace job queued",
	}

	writeJSONWithStatus(w, http.StatusAccepted, response)
}

// ListReplaceJobs lists all replace jobs.
func (h *Handler) ListReplaceJobs(w http.ResponseWriter, r *http.Request) {
	result, err := h.services.Queue.ListJobsWithOptions(r.Context(), queue.JobListOptions{
		Type:  queue.JobTypeReplace,
		Limit: 100,
	})
	if err != nil {
		http.Error(w, "Failed to list jobs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, result.Jobs)
}

// GetReplaceJob returns a single replace job.
func (h *Handler) GetReplaceJob(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	job, err := h.services.Queue.GetJob(r.Context(), id)
	if err != nil {
		http.Error(w, "Failed to get job: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if job == nil {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	if job.Type != queue.JobTypeReplace {
		http.Error(w, "Job is not a replace job", http.StatusBadRequest)
		return
	}

	writeJSON(w, job)
}

// ListJobs lists all jobs with pagination and filtering.
func (h *Handler) ListJobs(w http.ResponseWriter, r *http.Request) {
	// Parse query params
	typeFilter := r.URL.Query().Get("type")
	statusFilter := r.URL.Query().Get("status")
	excludeStatusFilter := r.URL.Query().Get("exclude_status")
	repoFilter := r.URL.Query().Get("repo")
	createdAfterFilter := r.URL.Query().Get("created_after")

	// Parse pagination params
	limit := 50
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Build options
	opts := queue.JobListOptions{
		Limit:    limit,
		Offset:   offset,
		RepoName: repoFilter,
	}

	if typeFilter != "" {
		opts.Type = queue.JobType(strings.ToLower(typeFilter))
	}

	if statusFilter != "" {
		opts.Status = queue.JobStatus(strings.ToLower(statusFilter))
	}

	if excludeStatusFilter != "" {
		opts.ExcludeStatus = queue.JobStatus(strings.ToLower(excludeStatusFilter))
	}

	// Parse created_after as RFC3339 or Unix timestamp
	if createdAfterFilter != "" {
		if t, err := time.Parse(time.RFC3339, createdAfterFilter); err == nil {
			opts.CreatedAfter = &t
		} else if unix, err := strconv.ParseInt(createdAfterFilter, 10, 64); err == nil {
			t := time.Unix(unix, 0)
			opts.CreatedAfter = &t
		}
	}

	result, err := h.services.Queue.ListJobsWithOptions(r.Context(), opts)
	if err != nil {
		http.Error(w, "Failed to list jobs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, result)
}

// GetJob returns a single job.
func (h *Handler) GetJob(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	job, err := h.services.Queue.GetJob(r.Context(), id)
	if err != nil {
		http.Error(w, "Failed to get job: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if job == nil {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	writeJSON(w, job)
}

// CancelJob cancels a pending job.
func (h *Handler) CancelJob(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	job, err := h.services.Queue.GetJob(r.Context(), id)
	if err != nil {
		http.Error(w, "Failed to get job: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if job == nil {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	if job.Status != queue.JobStatusPending {
		http.Error(w, "Only pending jobs can be canceled", http.StatusBadRequest)
		return
	}

	if err := h.services.Queue.UpdateStatus(r.Context(), id, queue.JobStatusFailed, "Canceled by user"); err != nil {
		http.Error(w, "Failed to cancel job: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{
		"status":  "canceled",
		"message": "Job canceled",
	})
}

// CleanupJobs removes old completed/failed jobs.
func (h *Handler) CleanupJobs(w http.ResponseWriter, r *http.Request) {
	// Default to 1 hour retention, allow override via query param
	maxAgeHours := 1

	if hoursStr := r.URL.Query().Get("max_age_hours"); hoursStr != "" {
		if hours, err := strconv.Atoi(hoursStr); err == nil && hours > 0 {
			maxAgeHours = hours
		}
	}

	result, err := h.services.Queue.CleanupOldJobs(
		r.Context(),
		time.Duration(maxAgeHours)*time.Hour,
	)
	if err != nil {
		http.Error(w, "Cleanup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"status":        "ok",
		"deleted_count": result.DeletedCount,
		"scanned_count": result.ScannedCount,
		"max_age_hours": maxAgeHours,
		"message":       "Cleanup completed",
	})
}

// BulkCancelJobs cancels all jobs matching the given filters.
func (h *Handler) BulkCancelJobs(w http.ResponseWriter, r *http.Request) {
	jobType := queue.JobType(r.URL.Query().Get("type"))
	status := queue.JobStatus(r.URL.Query().Get("status"))

	result, err := h.services.Queue.BulkCancelJobs(r.Context(), jobType, status)
	if err != nil {
		http.Error(w, "Bulk cancel failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"status":    "ok",
		"processed": result.Processed,
		"succeeded": result.Succeeded,
		"failed":    result.Failed,
		"message":   "Bulk cancel completed",
	})
}

// BulkDeleteJobs deletes all completed/failed jobs matching the given filters.
func (h *Handler) BulkDeleteJobs(w http.ResponseWriter, r *http.Request) {
	jobType := queue.JobType(r.URL.Query().Get("type"))
	status := queue.JobStatus(r.URL.Query().Get("status"))

	result, err := h.services.Queue.BulkDeleteJobs(r.Context(), jobType, status)
	if err != nil {
		http.Error(w, "Bulk delete failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"status":    "ok",
		"processed": result.Processed,
		"succeeded": result.Succeeded,
		"failed":    result.Failed,
		"message":   "Bulk delete completed",
	})
}

// DeleteAllJobs deletes ALL jobs (use with caution).
func (h *Handler) DeleteAllJobs(w http.ResponseWriter, r *http.Request) {
	// Require confirmation parameter for safety
	confirm := r.URL.Query().Get("confirm")
	if confirm != "true" {
		http.Error(w, "Must pass ?confirm=true to delete all jobs", http.StatusBadRequest)
		return
	}

	result, err := h.services.Queue.DeleteAllJobs(r.Context())
	if err != nil {
		http.Error(w, "Delete all failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"status":    "ok",
		"processed": result.Processed,
		"succeeded": result.Succeeded,
		"failed":    result.Failed,
		"message":   "All jobs deleted",
	})
}

// RebuildJobIndexes rebuilds the Redis sorted set indexes for jobs.
// This fixes stale data from jobs that were created before the index feature
// or when indexes get out of sync due to bugs.
func (h *Handler) RebuildJobIndexes(w http.ResponseWriter, r *http.Request) {
	processed, err := h.services.Queue.RebuildJobIndexes(r.Context())
	if err != nil {
		http.Error(w, "Rebuild indexes failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"status":    "ok",
		"processed": processed,
		"message":   "Job indexes rebuilt successfully",
	})
}
