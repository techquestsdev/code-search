// Package indexer provides the indexer HTTP server for federated file access.
package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"github.com/aanogueira/code-search/internal/files"
	"github.com/aanogueira/code-search/internal/queue"
	"github.com/aanogueira/code-search/internal/replace"
	"github.com/aanogueira/code-search/internal/scip"
)

// ServerConfig holds configuration for the indexer HTTP server.
type ServerConfig struct {
	Addr        string
	ReposPath   string
	ShardIndex  int
	TotalShards int
}

// Server provides HTTP endpoints for federated file access in sharded deployments.
// Each indexer runs this server to expose its local repos for browsing and replace operations.
type Server struct {
	cfg            ServerConfig
	logger         *zap.Logger
	filesService   *files.Service
	replaceService *replace.Service
	scipService    *scip.Service
	queue          *queue.Queue
	httpServer     *http.Server
}

// NewServer creates a new indexer HTTP server.
func NewServer(
	cfg ServerConfig,
	logger *zap.Logger,
	filesService *files.Service,
	replaceService *replace.Service,
	scipService *scip.Service,
	q *queue.Queue,
) *Server {
	return &Server{
		cfg:            cfg,
		logger:         logger.With(zap.String("component", "indexer-server")),
		filesService:   filesService,
		replaceService: replaceService,
		scipService:    scipService,
		queue:          q,
	}
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Health check
	r.Get("/health", s.handleHealth)

	// Shard info
	r.Get("/shard", s.handleShardInfo)

	// File browsing endpoints
	r.Get("/files/{repo}/tree", s.handleTree)
	r.Get("/files/{repo}/blob", s.handleBlob)
	r.Get("/files/{repo}/raw", s.handleRaw)
	r.Get("/files/{repo}/branches", s.handleBranches)
	r.Get("/files/{repo}/tags", s.handleTags)
	r.Get("/files/{repo}/exists", s.handleExists)

	// SCIP code intelligence endpoints (federation)
	if s.scipService != nil {
		r.Get("/scip/repos/{repoID}/status", s.handleSCIPStatus)
		r.Post("/scip/repos/{repoID}/definition", s.handleSCIPDefinition)
		r.Post("/scip/repos/{repoID}/references", s.handleSCIPReferences)
		r.Post("/scip/repos/{repoID}/symbols/search", s.handleSCIPSymbolSearch)
		r.Get("/scip/repos/{repoID}/files", s.handleSCIPListFiles)
	}

	// Replace endpoint for federated execution
	r.Post("/replace/execute", s.handleReplaceExecute)

	s.httpServer = &http.Server{
		Addr:         s.cfg.Addr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	s.logger.Info("Starting indexer HTTP server",
		zap.String("addr", s.cfg.Addr),
		zap.Int("shard_index", s.cfg.ShardIndex),
		zap.Int("total_shards", s.cfg.TotalShards),
	)

	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}

	return nil
}

// handleHealth returns server health status.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"shard":  strconv.Itoa(s.cfg.ShardIndex),
	})
}

// handleShardInfo returns information about this shard.
func (s *Server) handleShardInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"shard_index":  s.cfg.ShardIndex,
		"total_shards": s.cfg.TotalShards,
		"repos_path":   s.cfg.ReposPath,
	})
}

// handleTree returns directory listing for a repository.
func (s *Server) handleTree(w http.ResponseWriter, r *http.Request) {
	repo := chi.URLParam(r, "repo")
	if repo == "" {
		writeError(w, http.StatusBadRequest, "repository name required")
		return
	}

	// URL decode the repo name (handles %2F for /)
	repo, _ = decodeRepoName(repo)

	// Verify this repo belongs to our shard
	if s.cfg.TotalShards > 1 {
		targetShard := GetShardForRepo(repo, s.cfg.TotalShards)
		if targetShard != s.cfg.ShardIndex {
			writeError(
				w,
				http.StatusMisdirectedRequest,
				fmt.Sprintf("repo belongs to shard %d, not %d", targetShard, s.cfg.ShardIndex),
			)

			return
		}
	}

	path := r.URL.Query().Get("path")
	ref := r.URL.Query().Get("ref")

	entries, err := s.filesService.ListTree(r.Context(), repo, path, ref)
	if err != nil {
		s.logger.Debug("Failed to list tree", zap.String("repo", repo), zap.Error(err))
		writeError(w, http.StatusNotFound, err.Error())

		return
	}

	writeJSON(w, http.StatusOK, entries)
}

// handleBlob returns file content.
func (s *Server) handleBlob(w http.ResponseWriter, r *http.Request) {
	repo := chi.URLParam(r, "repo")
	if repo == "" {
		writeError(w, http.StatusBadRequest, "repository name required")
		return
	}

	repo, _ = decodeRepoName(repo)

	// Verify shard ownership
	if s.cfg.TotalShards > 1 {
		targetShard := GetShardForRepo(repo, s.cfg.TotalShards)
		if targetShard != s.cfg.ShardIndex {
			writeError(
				w,
				http.StatusMisdirectedRequest,
				fmt.Sprintf("repo belongs to shard %d", targetShard),
			)

			return
		}
	}

	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, "path parameter required")
		return
	}

	ref := r.URL.Query().Get("ref")

	blob, err := s.filesService.GetBlob(r.Context(), repo, path, ref)
	if err != nil {
		s.logger.Debug(
			"Failed to get blob",
			zap.String("repo", repo),
			zap.String("path", path),
			zap.Error(err),
		)
		writeError(w, http.StatusNotFound, err.Error())

		return
	}

	writeJSON(w, http.StatusOK, blob)
}

// handleExists checks if a repository exists locally.
func (s *Server) handleExists(w http.ResponseWriter, r *http.Request) {
	repo := chi.URLParam(r, "repo")
	if repo == "" {
		writeError(w, http.StatusBadRequest, "repository name required")
		return
	}

	repo, _ = decodeRepoName(repo)

	exists := s.filesService.RepoExists(repo)

	writeJSON(w, http.StatusOK, map[string]bool{
		"exists": exists,
	})
}

// handleRaw returns raw file content.
func (s *Server) handleRaw(w http.ResponseWriter, r *http.Request) {
	repo := chi.URLParam(r, "repo")
	if repo == "" {
		writeError(w, http.StatusBadRequest, "repository name required")
		return
	}

	repo, _ = decodeRepoName(repo)

	if s.cfg.TotalShards > 1 {
		targetShard := GetShardForRepo(repo, s.cfg.TotalShards)
		if targetShard != s.cfg.ShardIndex {
			writeError(w, http.StatusMisdirectedRequest,
				fmt.Sprintf("repo belongs to shard %d", targetShard))
			return
		}
	}

	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, "path parameter required")
		return
	}

	ref := r.URL.Query().Get("ref")

	body, size, err := s.filesService.GetRawBlob(r.Context(), repo, path, ref)
	if err != nil {
		s.logger.Debug("Failed to get raw blob",
			zap.String("repo", repo), zap.String("path", path), zap.Error(err))
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	defer body.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	if size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	}

	io.Copy(w, body)
}

// handleBranches returns all branches for a repository.
func (s *Server) handleBranches(w http.ResponseWriter, r *http.Request) {
	repo := chi.URLParam(r, "repo")
	if repo == "" {
		writeError(w, http.StatusBadRequest, "repository name required")
		return
	}

	repo, _ = decodeRepoName(repo)

	if s.cfg.TotalShards > 1 {
		targetShard := GetShardForRepo(repo, s.cfg.TotalShards)
		if targetShard != s.cfg.ShardIndex {
			writeError(w, http.StatusMisdirectedRequest,
				fmt.Sprintf("repo belongs to shard %d", targetShard))
			return
		}
	}

	branches, err := s.filesService.GetBranches(r.Context(), repo)
	if err != nil {
		s.logger.Debug("Failed to get branches", zap.String("repo", repo), zap.Error(err))
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, branches)
}

// handleTags returns all tags for a repository.
func (s *Server) handleTags(w http.ResponseWriter, r *http.Request) {
	repo := chi.URLParam(r, "repo")
	if repo == "" {
		writeError(w, http.StatusBadRequest, "repository name required")
		return
	}

	repo, _ = decodeRepoName(repo)

	if s.cfg.TotalShards > 1 {
		targetShard := GetShardForRepo(repo, s.cfg.TotalShards)
		if targetShard != s.cfg.ShardIndex {
			writeError(w, http.StatusMisdirectedRequest,
				fmt.Sprintf("repo belongs to shard %d", targetShard))
			return
		}
	}

	tags, err := s.filesService.GetTags(r.Context(), repo)
	if err != nil {
		s.logger.Debug("Failed to get tags", zap.String("repo", repo), zap.Error(err))
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, tags)
}

// parseRepoID extracts and validates the repoID URL parameter.
func parseRepoID(r *http.Request) (int64, error) {
	idStr := chi.URLParam(r, "repoID")
	return strconv.ParseInt(idStr, 10, 64)
}

// handleSCIPStatus returns SCIP index status for a repository.
func (s *Server) handleSCIPStatus(w http.ResponseWriter, r *http.Request) {
	repoID, err := parseRepoID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid repo ID")
		return
	}

	hasIndex, err := s.scipService.HasIndex(r.Context(), repoID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	result := map[string]any{"has_index": hasIndex}

	if hasIndex {
		stats, err := s.scipService.GetStats(r.Context(), repoID)
		if err == nil {
			result["stats"] = stats
		}
	}

	writeJSON(w, http.StatusOK, result)
}

// handleSCIPDefinition handles go-to-definition requests.
func (s *Server) handleSCIPDefinition(w http.ResponseWriter, r *http.Request) {
	repoID, err := parseRepoID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid repo ID")
		return
	}

	var req struct {
		FilePath string `json:"filePath"`
		Line     int    `json:"line"`
		Column   int    `json:"column"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := s.scipService.GoToDefinition(r.Context(), repoID, req.FilePath, req.Line, req.Column)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if result == nil {
		writeJSON(w, http.StatusOK, map[string]any{"found": false})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// handleSCIPReferences handles find-references requests.
func (s *Server) handleSCIPReferences(w http.ResponseWriter, r *http.Request) {
	repoID, err := parseRepoID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid repo ID")
		return
	}

	var req struct {
		FilePath string `json:"filePath"`
		Line     int    `json:"line"`
		Column   int    `json:"column"`
		Limit    int    `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Limit <= 0 {
		req.Limit = 100
	}

	result, err := s.scipService.FindReferences(r.Context(), repoID, req.FilePath, req.Line, req.Column, req.Limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if result == nil {
		writeJSON(w, http.StatusOK, map[string]any{"references": []any{}})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// handleSCIPSymbolSearch handles symbol search requests.
func (s *Server) handleSCIPSymbolSearch(w http.ResponseWriter, r *http.Request) {
	repoID, err := parseRepoID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid repo ID")
		return
	}

	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Limit <= 0 {
		req.Limit = 50
	}

	results, err := s.scipService.SearchSymbols(r.Context(), repoID, req.Query, req.Limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"symbols": results})
}

// handleSCIPListFiles returns indexed files for a repository.
func (s *Server) handleSCIPListFiles(w http.ResponseWriter, r *http.Request) {
	repoID, err := parseRepoID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid repo ID")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 1000
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	files, err := s.scipService.ListFiles(r.Context(), repoID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"files": files})
}

// ReplaceExecuteRequest is the request body for federated replace execution.
type ReplaceExecuteRequest struct {
	SearchPattern string               `json:"search_pattern"`
	ReplaceWith   string               `json:"replace_with"`
	IsRegex       bool                 `json:"is_regex"`
	CaseSensitive bool                 `json:"case_sensitive"`
	Matches       []queue.ReplaceMatch `json:"matches"`
	BranchName    string               `json:"branch_name,omitempty"`
	MRTitle       string               `json:"mr_title,omitempty"`
	MRDescription string               `json:"mr_description,omitempty"`
	UserTokens    map[string]string    `json:"user_tokens,omitempty"`
	ReposReadOnly bool                 `json:"repos_readonly"`
}

// handleReplaceExecute handles replace execution for repos owned by this shard.
func (s *Server) handleReplaceExecute(w http.ResponseWriter, r *http.Request) {
	var req ReplaceExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if len(req.Matches) == 0 {
		writeError(w, http.StatusBadRequest, "no matches provided")
		return
	}

	// Filter matches to only those belonging to this shard
	var myMatches []queue.ReplaceMatch

	for _, m := range req.Matches {
		if s.cfg.TotalShards <= 1 {
			myMatches = append(myMatches, m)
		} else {
			targetShard := GetShardForRepo(m.RepositoryName, s.cfg.TotalShards)
			if targetShard == s.cfg.ShardIndex {
				myMatches = append(myMatches, m)
			}
		}
	}

	if len(myMatches) == 0 {
		writeJSON(w, http.StatusOK, &replace.ExecuteResult{
			TotalFiles:    0,
			ModifiedFiles: 0,
			RepoResults:   []replace.RepoResult{},
		})

		return
	}

	s.logger.Info("Executing replace on shard",
		zap.Int("shard", s.cfg.ShardIndex),
		zap.Int("matches", len(myMatches)),
	)

	// Execute replace for our matches
	opts := replace.ExecuteOptions{
		SearchPattern: req.SearchPattern,
		ReplaceWith:   req.ReplaceWith,
		IsRegex:       req.IsRegex,
		CaseSensitive: req.CaseSensitive,
		Matches:       myMatches,
		BranchName:    req.BranchName,
		MRTitle:       req.MRTitle,
		MRDescription: req.MRDescription,
		UserTokens:    req.UserTokens,
		ReposReadOnly: req.ReposReadOnly,
	}

	result, err := s.replaceService.ExecuteFromOptions(r.Context(), opts)
	if err != nil {
		s.logger.Error("Replace execution failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, err.Error())

		return
	}

	writeJSON(w, http.StatusOK, result)
}

// Helper functions

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func decodeRepoName(encoded string) (string, error) {
	// Handle URL-encoded repo names (e.g., "owner%2Frepo" -> "owner/repo")
	// Chi already decodes URL params, but handle manual encoding too
	return encoded, nil
}
