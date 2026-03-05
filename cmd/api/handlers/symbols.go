package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/techquestsdev/code-search/internal/audit"
	"github.com/techquestsdev/code-search/internal/codehost"
	"github.com/techquestsdev/code-search/internal/middleware"
	"github.com/techquestsdev/code-search/internal/queue"
	"github.com/techquestsdev/code-search/internal/search"
)

// Symbol represents a code symbol.
type Symbol struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Repo      string `json:"repo"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	Signature string `json:"signature,omitempty"`
	Language  string `json:"language"`
}

// FindSymbolsRequest represents a symbol search request.
type FindSymbolsRequest struct {
	Name     string   `json:"name"`
	Kind     string   `json:"kind,omitempty"`
	Repos    []string `json:"repos,omitempty"`
	Language string   `json:"language,omitempty"`
	Limit    int      `json:"limit"`
}

// FindSymbols searches for symbol definitions using Zoekt's sym: filter.
func (h *Handler) FindSymbols(w http.ResponseWriter, r *http.Request) {
	var req FindSymbolsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Limit == 0 {
		req.Limit = 100
	}

	if req.Name == "" {
		http.Error(w, "Symbol name is required", http.StatusBadRequest)
		return
	}

	// RBAC: filter requested repos to only those the user can access
	if len(req.Repos) > 0 {
		req.Repos = h.services.Authorizer.FilterRepos(r.Context(), middleware.UserFromContext(r.Context()), req.Repos)
	}

	// Build Zoekt query with sym: filter for symbol search
	queryParts := []string{"sym:" + req.Name}

	// Add repo filter if specified - use OR pattern for multiple repos
	// Multiple repo: filters are ANDed in Zoekt, so we need a single regex
	if len(req.Repos) == 1 {
		queryParts = append(queryParts, "repo:"+req.Repos[0])
	} else if len(req.Repos) > 1 {
		// Escape special regex characters and join with |
		escaped := make([]string, len(req.Repos))
		for i, repo := range req.Repos {
			escaped[i] = regexp.QuoteMeta(repo)
		}

		queryParts = append(queryParts, "repo:("+strings.Join(escaped, "|")+")")
	}

	// Add language filter if specified
	if req.Language != "" {
		queryParts = append(queryParts, "lang:"+req.Language)
	}

	// Perform search using the search service
	searchReq := search.SearchRequest{
		Query: strings.Join(queryParts, " "),
		Limit: req.Limit,
	}

	results, err := h.services.Search.Search(r.Context(), searchReq)
	if err != nil {
		http.Error(w, "Search failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert search results to symbols
	symbols := make([]Symbol, 0)

	for _, result := range results.Results {
		symbol := Symbol{
			Name:     req.Name,
			Kind:     req.Kind,
			Repo:     result.Repo,
			File:     result.File,
			Line:     result.Line,
			Column:   result.Column,
			Language: result.Language,
		}

		// Try to extract signature from context
		if result.Content != "" {
			symbol.Signature = strings.TrimSpace(result.Content)
		}

		symbols = append(symbols, symbol)

		if len(symbols) >= req.Limit {
			break
		}
	}

	writeJSON(w, symbols)
}

// Reference represents a reference to a symbol.
type Reference struct {
	Repo    string `json:"repo"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Context string `json:"context"`
}

// FindRefsRequest represents a reference search request.
type FindRefsRequest struct {
	Symbol   string   `json:"symbol"`
	Repos    []string `json:"repos,omitempty"`
	Language string   `json:"language,omitempty"`
	Limit    int      `json:"limit"`
}

// FindRefs finds references to a symbol.
func (h *Handler) FindRefs(w http.ResponseWriter, r *http.Request) {
	var req FindRefsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Limit == 0 {
		req.Limit = 100
	}

	if req.Symbol == "" {
		http.Error(w, "Symbol name is required", http.StatusBadRequest)
		return
	}

	// RBAC: filter requested repos to only those the user can access
	if len(req.Repos) > 0 {
		req.Repos = h.services.Authorizer.FilterRepos(r.Context(), middleware.UserFromContext(r.Context()), req.Repos)
	}

	// Build Zoekt query - search for the symbol as a literal string
	// Use quotes for exact phrase matching, Zoekt will handle word boundaries
	queryParts := []string{fmt.Sprintf(`"%s"`, req.Symbol)}

	// Add repo filter if specified - use OR pattern for multiple repos
	// Multiple repo: filters are ANDed in Zoekt, so we need a single regex
	if len(req.Repos) == 1 {
		queryParts = append(queryParts, "repo:"+req.Repos[0])
	} else if len(req.Repos) > 1 {
		// Escape special regex characters and join with |
		escaped := make([]string, len(req.Repos))
		for i, repo := range req.Repos {
			escaped[i] = regexp.QuoteMeta(repo)
		}

		queryParts = append(queryParts, "repo:("+strings.Join(escaped, "|")+")")
	}

	// Add language filter if specified
	if req.Language != "" {
		queryParts = append(queryParts, "lang:"+req.Language)
	}

	// Perform search using the search service
	searchReq := search.SearchRequest{
		Query: strings.Join(queryParts, " "),
		Limit: req.Limit,
	}

	results, err := h.services.Search.Search(r.Context(), searchReq)
	if err != nil {
		http.Error(w, "Search failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert search results to references
	refs := make([]Reference, 0)

	for _, result := range results.Results {
		ref := Reference{
			Repo:    result.Repo,
			File:    result.File,
			Line:    result.Line,
			Column:  result.Column,
			Context: strings.TrimSpace(result.Content),
		}
		refs = append(refs, ref)

		if len(refs) >= req.Limit {
			break
		}
	}

	writeJSON(w, refs)
}

// Connection handlers

// ConnectionResponse represents a connection in API responses.
type ConnectionResponse struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	URL             string `json:"url"`
	TokenSet        bool   `json:"token_set"`
	ExcludeArchived bool   `json:"exclude_archived"`
	CleanupArchived bool   `json:"cleanup_archived"`
	CreatedAt       string `json:"created_at"`
}

// ConnectionsStatusResponse represents the status of connections management.
type ConnectionsStatusResponse struct {
	ReadOnly bool   `json:"readonly"`
	Message  string `json:"message,omitempty"`
}

// GetConnectionsStatus returns the status of connections management (readonly mode, etc.)
func (h *Handler) GetConnectionsStatus(w http.ResponseWriter, r *http.Request) {
	response := ConnectionsStatusResponse{
		ReadOnly: h.connectionsReadOnly,
	}
	if h.connectionsReadOnly {
		response.Message = "Connections are managed via configuration file. Changes through the UI are disabled."
	}

	writeJSON(w, response)
}

// ListConnections returns all code host connections.
func (h *Handler) ListConnections(w http.ResponseWriter, r *http.Request) {
	connections, err := h.services.Repos.ListConnections(r.Context())
	if err != nil {
		http.Error(w, "Failed to list connections: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := make([]ConnectionResponse, 0, len(connections))
	for _, c := range connections {
		response = append(response, ConnectionResponse{
			ID:              c.ID,
			Name:            c.Name,
			Type:            c.Type,
			URL:             c.URL,
			TokenSet:        c.Token != "",
			ExcludeArchived: c.ExcludeArchived,
			CleanupArchived: c.CleanupArchived,
			CreatedAt:       c.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	writeJSON(w, response)
}

// CreateConnectionRequest represents a request to create a connection.
type CreateConnectionRequest struct {
	Name            string `json:"name"`
	Type            string `json:"type"` // github, gitlab, gitea, bitbucket
	URL             string `json:"url"`
	Token           string `json:"token"`
	ExcludeArchived bool   `json:"exclude_archived"`
	CleanupArchived bool   `json:"cleanup_archived"`
}

// CreateConnection creates a new code host connection.
func (h *Handler) CreateConnection(w http.ResponseWriter, r *http.Request) {
	if h.connectionsReadOnly {
		http.Error(
			w,
			"Connections are read-only. Manage connections via config file.",
			http.StatusForbidden,
		)

		return
	}

	var req CreateConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	conn, err := h.services.Repos.CreateConnection(
		r.Context(),
		req.Name,
		req.Type,
		req.URL,
		req.Token,
		req.ExcludeArchived,
		req.CleanupArchived,
	)
	if err != nil {
		http.Error(w, "Failed to create connection: "+err.Error(), http.StatusInternalServerError)
		return
	}

	user := middleware.UserFromContext(r.Context())
	h.services.AuditLogger.LogOperation(r.Context(), audit.OperationEvent{
		UserID: user.ID, Action: "create", ResourceType: "connection",
		ResourceID: strconv.FormatInt(conn.ID, 10), ResourceName: conn.Name,
		Details:  map[string]any{"type": conn.Type},
		ClientIP: r.RemoteAddr, Timestamp: time.Now(),
	})

	response := ConnectionResponse{
		ID:              conn.ID,
		Name:            conn.Name,
		Type:            conn.Type,
		URL:             conn.URL,
		TokenSet:        conn.Token != "",
		ExcludeArchived: conn.ExcludeArchived,
		CleanupArchived: conn.CleanupArchived,
		CreatedAt:       conn.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}

	writeJSONWithStatus(w, http.StatusCreated, response)
}

// GetConnection returns a single connection.
func (h *Handler) GetConnection(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid connection ID", http.StatusBadRequest)
		return
	}

	conn, err := h.services.Repos.GetConnection(r.Context(), id)
	if err != nil {
		http.Error(w, "Failed to get connection: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if conn == nil {
		http.Error(w, "Connection not found", http.StatusNotFound)
		return
	}

	response := ConnectionResponse{
		ID:              conn.ID,
		Name:            conn.Name,
		Type:            conn.Type,
		URL:             conn.URL,
		TokenSet:        conn.Token != "",
		ExcludeArchived: conn.ExcludeArchived,
		CleanupArchived: conn.CleanupArchived,
		CreatedAt:       conn.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}

	writeJSON(w, response)
}

// UpdateConnectionRequest represents a request to update a connection.
type UpdateConnectionRequest struct {
	Name            string `json:"name"`
	Type            string `json:"type"`
	URL             string `json:"url"`
	Token           string `json:"token,omitempty"` // Optional - if empty, keep existing
	ExcludeArchived bool   `json:"exclude_archived"`
	CleanupArchived bool   `json:"cleanup_archived"`
}

// UpdateConnection updates an existing connection.
func (h *Handler) UpdateConnection(w http.ResponseWriter, r *http.Request) {
	if h.connectionsReadOnly {
		http.Error(
			w,
			"Connections are read-only. Manage connections via config file.",
			http.StatusForbidden,
		)

		return
	}

	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid connection ID", http.StatusBadRequest)
		return
	}

	var req UpdateConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Name == "" || req.Type == "" || req.URL == "" {
		http.Error(w, "Name, type, and URL are required", http.StatusBadRequest)
		return
	}

	conn, err := h.services.Repos.UpdateConnection(
		r.Context(),
		id,
		req.Name,
		req.Type,
		req.URL,
		req.Token,
		req.ExcludeArchived,
		req.CleanupArchived,
	)
	if err != nil {
		http.Error(w, "Failed to update connection: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if conn == nil {
		http.Error(w, "Connection not found", http.StatusNotFound)
		return
	}

	user := middleware.UserFromContext(r.Context())
	h.services.AuditLogger.LogOperation(r.Context(), audit.OperationEvent{
		UserID: user.ID, Action: "update", ResourceType: "connection",
		ResourceID: strconv.FormatInt(conn.ID, 10), ResourceName: conn.Name,
		Details:  map[string]any{"type": conn.Type},
		ClientIP: r.RemoteAddr, Timestamp: time.Now(),
	})

	response := ConnectionResponse{
		ID:              conn.ID,
		Name:            conn.Name,
		Type:            conn.Type,
		URL:             conn.URL,
		TokenSet:        conn.Token != "",
		ExcludeArchived: conn.ExcludeArchived,
		CleanupArchived: conn.CleanupArchived,
		CreatedAt:       conn.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}

	writeJSON(w, response)
}

// DeleteConnection removes a connection.
func (h *Handler) DeleteConnection(w http.ResponseWriter, r *http.Request) {
	if h.connectionsReadOnly {
		http.Error(
			w,
			"Connections are read-only. Manage connections via config file.",
			http.StatusForbidden,
		)

		return
	}

	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid connection ID", http.StatusBadRequest)
		return
	}

	// Fetch connection name for audit logging before deletion
	conn, err := h.services.Repos.GetConnection(r.Context(), id)
	if err != nil {
		http.Error(w, "Failed to get connection: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if conn == nil {
		http.Error(w, "Connection not found", http.StatusNotFound)
		return
	}

	if err := h.services.Repos.DeleteConnection(r.Context(), id); err != nil {
		http.Error(w, "Failed to delete connection: "+err.Error(), http.StatusInternalServerError)
		return
	}

	user := middleware.UserFromContext(r.Context())
	h.services.AuditLogger.LogOperation(r.Context(), audit.OperationEvent{
		UserID: user.ID, Action: "delete", ResourceType: "connection",
		ResourceID: strconv.FormatInt(id, 10), ResourceName: conn.Name,
		ClientIP: r.RemoteAddr, Timestamp: time.Now(),
	})

	w.WriteHeader(http.StatusNoContent)
}

// TestConnection tests if a connection is valid.
func (h *Handler) TestConnection(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid connection ID", http.StatusBadRequest)
		return
	}

	conn, err := h.services.Repos.GetConnection(r.Context(), id)
	if err != nil {
		http.Error(w, "Failed to get connection: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if conn == nil {
		http.Error(w, "Connection not found", http.StatusNotFound)
		return
	}

	// Test the connection by validating credentials
	client, err := codehost.NewClient(conn.Type, conn.URL, conn.Token)
	if err != nil {
		writeJSON(w, map[string]string{
			"status": "error",
			"error":  err.Error(),
		})

		return
	}

	// Validate credentials (fast - just fetches current user)
	if err := client.ValidateCredentials(r.Context()); err != nil {
		writeJSON(w, map[string]string{
			"status": "error",
			"error":  err.Error(),
		})

		return
	}

	writeJSON(w, map[string]any{
		"status":  "ok",
		"message": "Connection validated successfully",
	})
}

// SyncConnection queues a job to fetch repositories from a connection
// The actual fetching is done asynchronously by the indexer worker.
func (h *Handler) SyncConnection(w http.ResponseWriter, r *http.Request) {
	if h.connectionsReadOnly {
		http.Error(w, "Connections are read-only. Manage connections via config file.", http.StatusForbidden)
		return
	}

	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid connection ID", http.StatusBadRequest)
		return
	}

	conn, err := h.services.Repos.GetConnection(r.Context(), id)
	if err != nil {
		http.Error(w, "Failed to get connection: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if conn == nil {
		http.Error(w, "Connection not found", http.StatusNotFound)
		return
	}

	// Atomically try to acquire the sync job slot - this eliminates race conditions
	acquired, err := h.services.Queue.TryAcquireSyncJob(r.Context(), id)
	if err != nil {
		// Log but don't fail - queue check is a best-effort
	} else if !acquired {
		writeJSONWithStatus(w, http.StatusConflict, map[string]string{
			"status":  "already_syncing",
			"message": "Connection sync job already running",
		})

		return
	}

	// Queue sync job (async - worker will handle fetching repos from code host)
	payload := queue.SyncPayload{
		ConnectionID: id,
	}

	job, err := h.services.Queue.Enqueue(r.Context(), queue.JobTypeSync, payload)
	if err != nil {
		// Rollback active marker on failure
		_ = h.services.Queue.MarkSyncJobInactive(r.Context(), id)

		http.Error(w, "Failed to queue sync job: "+err.Error(), http.StatusInternalServerError)

		return
	}

	user := middleware.UserFromContext(r.Context())
	h.services.AuditLogger.LogOperation(r.Context(), audit.OperationEvent{
		UserID: user.ID, Action: "sync", ResourceType: "connection",
		ResourceID: strconv.FormatInt(id, 10), ResourceName: conn.Name,
		ClientIP: r.RemoteAddr, Timestamp: time.Now(),
	})

	writeJSON(w, map[string]any{
		"status":  "queued",
		"job_id":  job.ID,
		"message": "Sync job queued - repositories will be fetched in the background",
	})
}

// ListConnectionRepos lists repos that belong to a specific connection.
func (h *Handler) ListConnectionRepos(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid connection ID", http.StatusBadRequest)
		return
	}

	repos, err := h.services.Repos.ListRepositories(r.Context(), &id)
	if err != nil {
		http.Error(w, "Failed to list repositories: "+err.Error(), http.StatusInternalServerError)
		return
	}

	type RepoResponse struct {
		ID            int64    `json:"id"`
		Name          string   `json:"name"`
		CloneURL      string   `json:"clone_url"`
		DefaultBranch string   `json:"default_branch"`
		Branches      []string `json:"branches"`
		Status        string   `json:"status"`
		LastIndexed   *string  `json:"last_indexed,omitempty"`
	}

	response := make([]RepoResponse, 0, len(repos))
	for _, r := range repos {
		resp := RepoResponse{
			ID:            r.ID,
			Name:          r.Name,
			CloneURL:      r.CloneURL,
			DefaultBranch: r.DefaultBranch,
			Branches:      r.Branches,
			Status:        r.IndexStatus,
		}
		if r.LastIndexed != nil {
			t := r.LastIndexed.Format("2006-01-02T15:04:05Z")
			resp.LastIndexed = &t
		}

		response = append(response, resp)
	}

	writeJSON(w, response)
}
