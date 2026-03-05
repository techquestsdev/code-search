package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/techquestsdev/code-search/internal/audit"
	"github.com/techquestsdev/code-search/internal/log"
	"github.com/techquestsdev/code-search/internal/middleware"
	"github.com/techquestsdev/code-search/internal/queue"
	"github.com/techquestsdev/code-search/internal/repos"
)

// RepositoryResponse represents a repository in API responses.
type RepositoryResponse struct {
	ID            int64    `json:"id"`
	Name          string   `json:"name"`
	CloneURL      string   `json:"clone_url"`
	DefaultBranch string   `json:"default_branch"`
	Branches      []string `json:"branches"`
	Status        string   `json:"status"`
	LastIndexed   string   `json:"last_indexed,omitempty"`
	Excluded      bool     `json:"excluded"`
	Deleted       bool     `json:"deleted"`
}

// ListReposResponse represents the list repos response with pagination.
type ListReposResponse struct {
	Repos      []RepositoryResponse `json:"repos"`
	TotalCount int                  `json:"total_count"`
	Limit      int                  `json:"limit"`
	Offset     int                  `json:"offset"`
	HasMore    bool                 `json:"has_more"`
}

// ListRepos returns repositories with optional pagination and filtering.
func (h *Handler) ListRepos(w http.ResponseWriter, r *http.Request) {
	opts := repos.RepoListOptions{}

	// Parse connection_id filter
	if connIDStr := r.URL.Query().Get("connection_id"); connIDStr != "" {
		id, err := strconv.ParseInt(connIDStr, 10, 64)
		if err == nil {
			opts.ConnectionID = &id
		}
	}

	// Parse search filter
	opts.Search = r.URL.Query().Get("search")

	// Parse status filter
	opts.Status = r.URL.Query().Get("status")

	// Parse pagination params
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			opts.Limit = parsed
		}
	}

	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			opts.Offset = parsed
		}
	}

	result, err := h.services.Repos.ListRepositoriesWithOptions(r.Context(), opts)
	if err != nil {
		http.Error(w, "Failed to list repositories: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// RBAC: filter repos to only those the user can access
	user := middleware.UserFromContext(r.Context())
	repoNames := make([]string, len(result.Repos))
	for i, repo := range result.Repos {
		repoNames[i] = repo.Name
	}
	allowedNames := h.services.Authorizer.FilterRepos(r.Context(), user, repoNames)
	allowedSet := make(map[string]bool, len(allowedNames))
	for _, name := range allowedNames {
		allowedSet[name] = true
	}
	filteredRepos := make([]repos.Repository, 0, len(allowedNames))
	for _, repo := range result.Repos {
		if allowedSet[repo.Name] {
			filteredRepos = append(filteredRepos, repo)
		}
	}

	// When RBAC filters out repos from the current page, the total count
	// should still reflect the database total so pagination works correctly.
	// Only adjust if RBAC actually filtered something on this page.
	totalCount := result.TotalCount
	filteredOut := len(result.Repos) - len(filteredRepos)
	if filteredOut > 0 {
		totalCount = totalCount - filteredOut
		if totalCount < 0 {
			totalCount = 0
		}
	}

	response := ListReposResponse{
		Repos:      make([]RepositoryResponse, 0, len(filteredRepos)),
		TotalCount: totalCount,
		Limit:      result.Limit,
		Offset:     result.Offset,
		HasMore:    result.HasMore,
	}

	for _, repo := range filteredRepos {
		lastIndexed := ""
		if repo.LastIndexed != nil {
			lastIndexed = repo.LastIndexed.Format("2006-01-02T15:04:05Z")
		}
		// Use default_branch as fallback if branches is empty
		branches := repo.Branches
		if len(branches) == 0 && repo.DefaultBranch != "" {
			branches = []string{repo.DefaultBranch}
		}

		response.Repos = append(response.Repos, RepositoryResponse{
			ID:            repo.ID,
			Name:          repo.Name,
			CloneURL:      repo.CloneURL,
			DefaultBranch: repo.DefaultBranch,
			Branches:      branches,
			Status:        repo.IndexStatus,
			LastIndexed:   lastIndexed,
			Excluded:      repo.Excluded,
			Deleted:       repo.Deleted,
		})
	}

	writeJSON(w, response)
}

// AddRepoRequest represents a request to add a repository.
type AddRepoRequest struct {
	ConnectionID  int64    `json:"connection_id"`
	Name          string   `json:"name"`
	CloneURL      string   `json:"clone_url"`
	DefaultBranch string   `json:"default_branch"`
	Branches      []string `json:"branches,omitempty"`
}

// AddRepo adds a new repository.
func (h *Handler) AddRepo(w http.ResponseWriter, r *http.Request) {
	if h.reposReadOnly {
		http.Error(w, "Repositories are read-only. Manage repos via sync.", http.StatusForbidden)
		return
	}

	var req AddRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.DefaultBranch == "" {
		req.DefaultBranch = "main"
	}

	if len(req.Branches) == 0 {
		req.Branches = []string{req.DefaultBranch}
	}

	repo, err := h.services.Repos.CreateRepository(
		r.Context(),
		req.ConnectionID,
		req.Name,
		req.CloneURL,
		req.DefaultBranch,
		req.Branches,
	)
	if err != nil {
		http.Error(w, "Failed to create repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	user := middleware.UserFromContext(r.Context())
	h.services.AuditLogger.LogOperation(r.Context(), audit.OperationEvent{
		UserID: user.ID, Action: "create", ResourceType: "repo",
		ResourceID: strconv.FormatInt(repo.ID, 10), ResourceName: repo.Name,
		Details:  map[string]any{"clone_url": repo.CloneURL},
		ClientIP: r.RemoteAddr, Timestamp: time.Now(),
	})

	response := RepositoryResponse{
		ID:       repo.ID,
		Name:     repo.Name,
		CloneURL: repo.CloneURL,
		Branches: repo.Branches,
		Status:   repo.IndexStatus,
		Excluded: repo.Excluded,
	}

	writeJSONWithStatus(w, http.StatusCreated, response)
}

// GetRepoByID returns a single repository by ID.
func (h *Handler) GetRepoByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid repository ID", http.StatusBadRequest)
		return
	}

	repo, err := h.services.Repos.GetRepository(r.Context(), id)
	if err != nil {
		http.Error(w, "Failed to get repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if repo == nil {
		http.Error(w, "Repository not found", http.StatusNotFound)
		return
	}

	if !h.services.Authorizer.CanAccessRepo(r.Context(), middleware.UserFromContext(r.Context()), repo.Name) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	lastIndexed := ""
	if repo.LastIndexed != nil {
		lastIndexed = repo.LastIndexed.Format("2006-01-02T15:04:05Z")
	}

	// Get branches, with fallback to default_branch for existing repos
	branches := repo.Branches
	if len(branches) == 0 && repo.DefaultBranch != "" {
		branches = []string{repo.DefaultBranch}
	}

	response := RepositoryResponse{
		ID:            repo.ID,
		Name:          repo.Name,
		CloneURL:      repo.CloneURL,
		Branches:      branches,
		DefaultBranch: repo.DefaultBranch,
		Status:        repo.IndexStatus,
		LastIndexed:   lastIndexed,
		Excluded:      repo.Excluded,
	}

	writeJSON(w, response)
}

// SetRepoBranchesRequest represents a request to set branches to index.
type SetRepoBranchesRequest struct {
	Branches []string `json:"branches"`
}

// SetRepoBranches sets which branches to index for a repository.
// PUT /api/v1/repos/by-id/{id}/branches.
func (h *Handler) SetRepoBranches(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid repository ID", http.StatusBadRequest)
		return
	}

	var req SetRepoBranchesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get repo to validate it exists
	repo, err := h.services.Repos.GetRepository(r.Context(), id)
	if err != nil {
		http.Error(w, "Failed to get repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if repo == nil {
		http.Error(w, "Repository not found", http.StatusNotFound)
		return
	}

	// If no branches specified, default to the default branch
	branches := req.Branches
	if len(branches) == 0 {
		branches = []string{repo.DefaultBranch}
	}

	// Update branches in database
	if err := h.services.Repos.UpdateBranches(r.Context(), id, branches); err != nil {
		http.Error(w, "Failed to update branches: "+err.Error(), http.StatusInternalServerError)
		return
	}

	user := middleware.UserFromContext(r.Context())
	h.services.AuditLogger.LogOperation(r.Context(), audit.OperationEvent{
		UserID: user.ID, Action: "update", ResourceType: "repo",
		ResourceID: strconv.FormatInt(id, 10), ResourceName: repo.Name,
		Details:  map[string]any{"branches": branches},
		ClientIP: r.RemoteAddr, Timestamp: time.Now(),
	})

	writeJSON(w, map[string]any{
		"status":   "updated",
		"repo_id":  id,
		"branches": branches,
	})
}

// DeleteRepoByID removes a repository by ID.
func (h *Handler) DeleteRepoByID(w http.ResponseWriter, r *http.Request) {
	if h.reposReadOnly {
		http.Error(w, "Repositories are read-only. Manage repos via sync.", http.StatusForbidden)
		return
	}

	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid repository ID", http.StatusBadRequest)
		return
	}

	repo, err := h.services.Repos.GetRepository(r.Context(), id)
	if err != nil {
		http.Error(w, "Failed to get repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if repo == nil {
		http.Error(w, "Repository not found", http.StatusNotFound)
		return
	}

	if err := h.services.Repos.DeleteRepository(r.Context(), id); err != nil {
		http.Error(w, "Failed to delete repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	user := middleware.UserFromContext(r.Context())
	h.services.AuditLogger.LogOperation(r.Context(), audit.OperationEvent{
		UserID: user.ID, Action: "delete", ResourceType: "repo",
		ResourceID: strconv.FormatInt(id, 10), ResourceName: repo.Name,
		ClientIP: r.RemoteAddr, Timestamp: time.Now(),
	})

	w.WriteHeader(http.StatusNoContent)
}

// ExcludeRepoByID excludes a repository from sync and indexing (soft delete).
func (h *Handler) ExcludeRepoByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid repository ID", http.StatusBadRequest)
		return
	}

	repo, err := h.services.Repos.GetRepository(r.Context(), id)
	if err != nil {
		http.Error(w, "Failed to get repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if repo == nil {
		http.Error(w, "Repository not found", http.StatusNotFound)
		return
	}

	if err := h.services.Repos.ExcludeRepository(r.Context(), id); err != nil {
		http.Error(w, "Failed to exclude repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Clear any active index job marker for this repo (best-effort)
	// This ensures that if the repo was pending, re-including it won't fail due to stale markers
	if err := h.services.Queue.MarkIndexJobInactive(r.Context(), id); err != nil {
		if h.services.Logger != nil {
			h.services.Logger.Debug("failed to clear index job marker for excluded repo",
				zap.Int64("repo_id", id),
				zap.Error(err),
			)
		}
	}

	// Queue a cleanup job to remove from Zoekt index and delete cloned files
	cleanupPayload := queue.CleanupPayload{
		RepositoryID:   id,
		RepositoryName: repo.Name,
		DataDir:        h.services.IndexPath, // Worker uses this to locate index and repos
	}
	if _, err := h.services.Queue.Enqueue(r.Context(), queue.JobTypeCleanup, cleanupPayload); err != nil {
		// Log error but don't fail the request - repo is already excluded
		// The cleanup will be handled on next sync or manually
		if h.services.Logger != nil {
			h.services.Logger.Warn("failed to queue cleanup job for excluded repo",
				zap.Int64("repo_id", id),
				zap.String("repo_name", repo.Name),
				zap.Error(err),
			)
		}
	}

	user := middleware.UserFromContext(r.Context())
	h.services.AuditLogger.LogOperation(r.Context(), audit.OperationEvent{
		UserID: user.ID, Action: "exclude", ResourceType: "repo",
		ResourceID: strconv.FormatInt(id, 10), ResourceName: repo.Name,
		ClientIP: r.RemoteAddr, Timestamp: time.Now(),
	})

	writeJSON(w, map[string]any{
		"message": "Repository excluded from sync and indexing",
		"id":      id,
	})
}

// IncludeRepoByID includes a previously excluded repository and queues an index job.
func (h *Handler) IncludeRepoByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid repository ID", http.StatusBadRequest)
		return
	}

	repo, err := h.services.Repos.GetRepository(r.Context(), id)
	if err != nil {
		http.Error(w, "Failed to get repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if repo == nil {
		http.Error(w, "Repository not found", http.StatusNotFound)
		return
	}

	// Include the repository (sets excluded=false, index_status=pending)
	if err := h.services.Repos.IncludeRepository(r.Context(), id); err != nil {
		http.Error(w, "Failed to include repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Clear any stale index job marker first (in case it wasn't cleaned up properly)
	// This is a best-effort operation to recover from previous failures
	_ = h.services.Queue.MarkIndexJobInactive(r.Context(), repo.ID)

	// Queue an index job immediately so it doesn't wait for scheduler
	// Try to acquire the job slot first to prevent duplicates
	acquired, err := h.services.Queue.TryAcquireIndexJob(r.Context(), repo.ID)
	if err != nil {
		// Log but don't fail - queue check is best-effort
		log.L.Debug("Failed to acquire index job slot", zap.Int64("repo_id", repo.ID), zap.Error(err))
	}

	var jobID string

	if acquired {
		payload := queue.IndexPayload{
			RepositoryID: repo.ID,
			ConnectionID: repo.ConnectionID,
			RepoName:     repo.Name,
			CloneURL:     repo.CloneURL,
			Branch:       repo.DefaultBranch,
			Branches:     repo.Branches,
		}

		job, err := h.services.Queue.Enqueue(r.Context(), queue.JobTypeIndex, payload)
		if err != nil {
			// Rollback active marker on failure
			_ = h.services.Queue.MarkIndexJobInactive(r.Context(), repo.ID)
			log.L.Warn("Failed to queue index job for included repo",
				zap.Int64("repo_id", repo.ID),
				zap.Error(err),
			)
		} else {
			jobID = job.ID
		}
	}

	user := middleware.UserFromContext(r.Context())
	h.services.AuditLogger.LogOperation(r.Context(), audit.OperationEvent{
		UserID: user.ID, Action: "include", ResourceType: "repo",
		ResourceID: strconv.FormatInt(id, 10), ResourceName: repo.Name,
		ClientIP: r.RemoteAddr, Timestamp: time.Now(),
	})

	response := map[string]any{
		"message": "Repository included and index job queued",
		"id":      id,
	}
	if jobID != "" {
		response["job_id"] = jobID
	}

	writeJSON(w, response)
}

// RestoreRepoByID restores a previously deleted repository and queues an index job.
func (h *Handler) RestoreRepoByID(w http.ResponseWriter, r *http.Request) {
	if h.reposReadOnly {
		http.Error(w, "Repositories are read-only. Manage repos via sync.", http.StatusForbidden)
		return
	}

	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid repository ID", http.StatusBadRequest)
		return
	}

	repo, err := h.services.Repos.GetRepository(r.Context(), id)
	if err != nil {
		http.Error(w, "Failed to get repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if repo == nil {
		http.Error(w, "Repository not found", http.StatusNotFound)
		return
	}

	if !repo.Deleted {
		http.Error(w, "Repository is not deleted", http.StatusBadRequest)
		return
	}

	if err := h.services.Repos.RestoreRepository(r.Context(), id); err != nil {
		http.Error(w, "Failed to restore repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Queue an index job immediately so it doesn't wait for scheduler
	acquired, err := h.services.Queue.TryAcquireIndexJob(r.Context(), repo.ID)
	if err != nil {
		log.L.Debug("Failed to acquire index job slot", zap.Int64("repo_id", repo.ID), zap.Error(err))
	}

	var jobID string

	if acquired {
		payload := queue.IndexPayload{
			RepositoryID: repo.ID,
			ConnectionID: repo.ConnectionID,
			RepoName:     repo.Name,
			CloneURL:     repo.CloneURL,
			Branch:       repo.DefaultBranch,
			Branches:     repo.Branches,
		}

		job, err := h.services.Queue.Enqueue(r.Context(), queue.JobTypeIndex, payload)
		if err != nil {
			_ = h.services.Queue.MarkIndexJobInactive(r.Context(), repo.ID)
			log.L.Warn("Failed to queue index job for restored repo",
				zap.Int64("repo_id", repo.ID),
				zap.Error(err),
			)
		} else {
			jobID = job.ID
		}
	}

	user := middleware.UserFromContext(r.Context())
	h.services.AuditLogger.LogOperation(r.Context(), audit.OperationEvent{
		UserID: user.ID, Action: "restore", ResourceType: "repo",
		ResourceID: strconv.FormatInt(id, 10), ResourceName: repo.Name,
		ClientIP: r.RemoteAddr, Timestamp: time.Now(),
	})

	response := map[string]any{
		"message": "Repository restored and index job queued",
		"id":      id,
	}
	if jobID != "" {
		response["job_id"] = jobID
	}

	writeJSON(w, response)
}

// ReposStatus returns the readonly status for repos management.
func (h *Handler) ReposStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"readonly": h.reposReadOnly,
	})
}

// SyncRepoByID triggers a sync/reindex for a repository by ID.
func (h *Handler) SyncRepoByID(w http.ResponseWriter, r *http.Request) {
	if h.reposReadOnly {
		http.Error(w, "Repositories are read-only. Manage repos via sync.", http.StatusForbidden)
		return
	}

	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid repository ID", http.StatusBadRequest)
		return
	}

	repo, err := h.services.Repos.GetRepository(r.Context(), id)
	if err != nil {
		http.Error(w, "Failed to get repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if repo == nil {
		http.Error(w, "Repository not found", http.StatusNotFound)
		return
	}

	// Check if repo is already being synced (pending status)
	if repo.IndexStatus == "pending" {
		active, err := h.services.Queue.HasPendingIndexJob(r.Context(), repo.ID)
		if err != nil {
			log.L.Debug("Failed to check active index job", zap.Int64("repo_id", repo.ID), zap.Error(err))

			active = true
		}

		if active {
			writeJSONWithStatus(w, http.StatusConflict, map[string]string{
				"status":  "already_syncing",
				"message": "Repository sync already in progress",
			})

			return
		}

		log.L.Debug("Repo pending without active job; enqueueing sync",
			zap.Int64("repo_id", repo.ID),
			zap.String("repo", repo.Name),
		)
	}

	// Atomically try to acquire the job slot - this eliminates race conditions
	acquired, err := h.services.Queue.TryAcquireIndexJob(r.Context(), repo.ID)
	if err != nil {
		// Log but don't fail - queue check is a best-effort
	} else if !acquired {
		writeJSONWithStatus(w, http.StatusConflict, map[string]string{
			"status":  "already_syncing",
			"message": "Repository sync job already queued",
		})

		return
	}

	// Queue index job
	payload := queue.IndexPayload{
		RepositoryID: repo.ID,
		ConnectionID: repo.ConnectionID,
		RepoName:     repo.Name,
		CloneURL:     repo.CloneURL,
		Branch:       repo.DefaultBranch,
		Branches:     repo.Branches, // Include configured branches for multi-branch indexing
	}

	job, err := h.services.Queue.Enqueue(r.Context(), queue.JobTypeIndex, payload)
	if err != nil {
		// Rollback active marker on failure (best-effort)
		if markErr := h.services.Queue.MarkIndexJobInactive(r.Context(), repo.ID); markErr != nil {
			log.L.Debug("Failed to mark index job inactive",
				zap.Int64("repo_id", repo.ID),
				zap.Error(markErr),
			)
		}

		http.Error(w, "Failed to queue sync job: "+err.Error(), http.StatusInternalServerError)

		return
	}

	// Update status to pending (best-effort, log but don't fail)
	if err := h.services.Repos.UpdateIndexStatus(r.Context(), repo.ID, "pending", false); err != nil {
		log.L.Debug("Failed to update index status to pending",
			zap.Int64("repo_id", repo.ID),
			zap.Error(err),
		)
	}

	user := middleware.UserFromContext(r.Context())
	h.services.AuditLogger.LogOperation(r.Context(), audit.OperationEvent{
		UserID: user.ID, Action: "sync", ResourceType: "repo",
		ResourceID: strconv.FormatInt(id, 10), ResourceName: repo.Name,
		ClientIP: r.RemoteAddr, Timestamp: time.Now(),
	})

	writeJSON(w, map[string]string{
		"status":  "queued",
		"job_id":  job.ID,
		"message": "Sync job queued",
	})
}

// SetRepoPollIntervalRequest represents a request to set poll interval.
type SetRepoPollIntervalRequest struct {
	IntervalSeconds int `json:"interval_seconds"`
}

// SetRepoPollInterval sets a custom poll interval for a repository.
func (h *Handler) SetRepoPollInterval(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid repository ID", http.StatusBadRequest)
		return
	}

	var req SetRepoPollIntervalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.IntervalSeconds < 0 {
		http.Error(
			w,
			"Interval must be non-negative (use 0 to reset to default)",
			http.StatusBadRequest,
		)

		return
	}

	// Update poll interval via service
	if err := h.services.Repos.SetRepoPollInterval(r.Context(), id, req.IntervalSeconds); err != nil {
		http.Error(
			w,
			"Failed to update poll interval: "+err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	user := middleware.UserFromContext(r.Context())
	h.services.AuditLogger.LogOperation(r.Context(), audit.OperationEvent{
		UserID: user.ID, Action: "update", ResourceType: "repo",
		ResourceID: strconv.FormatInt(id, 10),
		Details:    map[string]any{"poll_interval": req.IntervalSeconds},
		ClientIP:   r.RemoteAddr, Timestamp: time.Now(),
	})

	writeJSON(w, map[string]any{
		"status":           "updated",
		"repo_id":          id,
		"interval_seconds": req.IntervalSeconds,
	})
}

// LookupRepoByName looks up a repository by its Zoekt name (e.g., "gitlab.example.com/org/repo").
// GET /api/v1/repos/lookup?name=...
func (h *Handler) LookupRepoByName(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "Name parameter is required", http.StatusBadRequest)
		return
	}

	// Use FindRepositoryByZoektName which handles the host prefix matching
	repo, err := h.services.Repos.FindRepositoryByZoektName(r.Context(), name)
	if err != nil {
		http.Error(w, "Failed to lookup repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if repo == nil {
		http.Error(w, "Repository not found", http.StatusNotFound)
		return
	}

	lastIndexed := ""
	if repo.LastIndexed != nil {
		lastIndexed = repo.LastIndexed.Format("2006-01-02T15:04:05Z")
	}

	// Get branches, with fallback to default_branch for existing repos
	branches := repo.Branches
	if len(branches) == 0 && repo.DefaultBranch != "" {
		branches = []string{repo.DefaultBranch}
	}

	response := RepositoryResponse{
		ID:            repo.ID,
		Name:          repo.Name,
		CloneURL:      repo.CloneURL,
		Branches:      branches,
		DefaultBranch: repo.DefaultBranch,
		Status:        repo.IndexStatus,
		LastIndexed:   lastIndexed,
		Excluded:      repo.Excluded,
	}

	writeJSON(w, response)
}
