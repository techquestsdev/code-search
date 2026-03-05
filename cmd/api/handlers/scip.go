package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/aanogueira/code-search/internal/files"
	"github.com/aanogueira/code-search/internal/middleware"
	"github.com/aanogueira/code-search/internal/scip"
)

// SCIPHandler handles SCIP-related endpoints.
type SCIPHandler struct {
	services *Services
	scipSvc  *scip.Service
}

// NewSCIPHandler creates a new SCIP handler.
func NewSCIPHandler(services *Services, scipSvc *scip.Service) *SCIPHandler {
	return &SCIPHandler{
		services: services,
		scipSvc:  scipSvc,
	}
}

// getBlob fetches file content using federated access when available.
func (h *SCIPHandler) getBlob(ctx context.Context, repoName, path, ref string) (*files.BlobResponse, error) {
	if h.services.FederatedFiles != nil {
		return h.services.FederatedFiles.GetBlob(ctx, repoName, path, ref)
	}

	return h.services.Files.GetBlob(ctx, repoName, path, ref)
}

// enrichOccurrenceWithContext fetches the source line for an occurrence.
func (h *SCIPHandler) enrichOccurrenceWithContext(
	ctx context.Context,
	repoName string,
	occ *scip.Occurrence,
) {
	if occ == nil || occ.FilePath == "" {
		return
	}

	// Fetch the file content (federated or local)
	blob, err := h.getBlob(ctx, repoName, occ.FilePath, "HEAD")
	if err != nil || blob == nil || blob.Binary {
		return
	}

	// Split into lines and get the relevant line
	lines := strings.Split(blob.Content, "\n")
	if occ.StartLine >= 0 && occ.StartLine < len(lines) {
		occ.Context = strings.TrimRight(lines[occ.StartLine], "\r")
	}
}

// enrichOccurrencesWithContext fetches source lines for multiple occurrences.
func (h *SCIPHandler) enrichOccurrencesWithContext(
	ctx context.Context,
	repoName string,
	occs []scip.Occurrence,
) {
	// Group by file to minimize file reads
	fileCache := make(map[string][]string)

	for i := range occs {
		occ := &occs[i]
		if occ.FilePath == "" {
			continue
		}

		// Check cache first
		lines, ok := fileCache[occ.FilePath]
		if !ok {
			blob, err := h.getBlob(ctx, repoName, occ.FilePath, "HEAD")
			if err != nil || blob == nil || blob.Binary {
				fileCache[occ.FilePath] = nil // Cache the failure
				continue
			}

			lines = strings.Split(blob.Content, "\n")
			fileCache[occ.FilePath] = lines
		}

		if lines != nil && occ.StartLine >= 0 && occ.StartLine < len(lines) {
			occ.Context = strings.TrimRight(lines[occ.StartLine], "\r")
		}
	}
}

// SCIPStatusResponse contains SCIP index status.
type SCIPStatusResponse struct {
	HasIndex          bool            `json:"has_index"`
	AvailableIndexers map[string]bool `json:"available_indexers"`
	Stats             map[string]any  `json:"stats,omitempty"`
}

// checkRepoAccess fetches a repository by ID and checks RBAC access.
// Returns the repo on success, or nil if access was denied (error already written to w).
func (h *SCIPHandler) checkRepoAccess(w http.ResponseWriter, r *http.Request, repoID int64) bool {
	repo, err := h.services.Repos.GetRepository(r.Context(), repoID)
	if err != nil {
		http.Error(w, "Failed to get repository: "+err.Error(), http.StatusInternalServerError)
		return false
	}
	if repo == nil {
		http.Error(w, "Repository not found", http.StatusNotFound)
		return false
	}
	if !h.services.Authorizer.CanAccessRepo(r.Context(), middleware.UserFromContext(r.Context()), repo.Name) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return false
	}
	return true
}

// GetSCIPStatus returns the SCIP index status for a repository.
func (h *SCIPHandler) GetSCIPStatus(w http.ResponseWriter, r *http.Request) {
	repoIDStr := chi.URLParam(r, "id")

	repoID, err := strconv.ParseInt(repoIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid repository ID", http.StatusBadRequest)
		return
	}

	if !h.checkRepoAccess(w, r, repoID) {
		return
	}

	hasIndex, err := h.scipSvc.HasIndex(r.Context(), repoID)
	if err != nil {
		http.Error(w, "Failed to check index status: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := SCIPStatusResponse{
		HasIndex:          hasIndex,
		AvailableIndexers: h.scipSvc.GetAvailableIndexers(),
	}

	if hasIndex {
		stats, err := h.scipSvc.GetStats(r.Context(), repoID)
		if err == nil {
			response.Stats = stats
		}
	}

	writeJSON(w, response)
}

// GoToDefinitionRequest is the request for go-to-definition.
type GoToDefinitionRequest struct {
	FilePath string `json:"filePath"`
	Line     int    `json:"line"`   // 1-indexed (matches editor display)
	Column   int    `json:"column"` // 0-indexed
}

// GoToDefinitionResponse is the response for go-to-definition.
type GoToDefinitionResponse struct {
	Found      bool             `json:"found"`
	Symbol     string           `json:"symbol,omitempty"`
	Definition *scip.Occurrence `json:"definition,omitempty"`
	Info       *scip.SymbolInfo `json:"info,omitempty"`
	External   bool             `json:"external"`
}

// GoToDefinition finds the definition of a symbol using SCIP.
func (h *SCIPHandler) GoToDefinition(w http.ResponseWriter, r *http.Request) {
	repoIDStr := chi.URLParam(r, "id")

	repoID, err := strconv.ParseInt(repoIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid repository ID", http.StatusBadRequest)
		return
	}

	if !h.checkRepoAccess(w, r, repoID) {
		return
	}

	var req GoToDefinitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	result, err := h.scipSvc.GoToDefinition(r.Context(), repoID, req.FilePath, req.Line, req.Column)
	if err != nil {
		http.Error(w, "Failed to find definition: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := GoToDefinitionResponse{
		Found: result != nil && result.Definition != nil,
	}

	if result != nil {
		response.Symbol = result.Symbol
		response.Definition = result.Definition
		response.Info = result.Info
		response.External = result.External

		// Enrich definition with context (source line)
		if response.Definition != nil {
			repo, repoErr := h.services.Repos.GetRepository(r.Context(), repoID)
			if repoErr == nil && repo != nil {
				h.enrichOccurrenceWithContext(r.Context(), repo.Name, response.Definition)
			}
		}
	}

	writeJSON(w, response)
}

// FindReferencesRequest is the request for find-references.
type FindReferencesRequest struct {
	FilePath string `json:"filePath"`
	Line     int    `json:"line"`            // 1-indexed (matches editor display)
	Column   int    `json:"column"`          // 0-indexed
	Limit    int    `json:"limit,omitempty"` // 0 for no limit
}

// FindReferencesResponse is the response for find-references.
type FindReferencesResponse struct {
	Found      bool              `json:"found"`
	Symbol     string            `json:"symbol,omitempty"`
	Definition *scip.Occurrence  `json:"definition,omitempty"`
	References []scip.Occurrence `json:"references"`
	TotalCount int               `json:"totalCount"`
}

// FindReferences finds all references to a symbol using SCIP.
func (h *SCIPHandler) FindReferences(w http.ResponseWriter, r *http.Request) {
	repoIDStr := chi.URLParam(r, "id")

	repoID, err := strconv.ParseInt(repoIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid repository ID", http.StatusBadRequest)
		return
	}

	if !h.checkRepoAccess(w, r, repoID) {
		return
	}

	var req FindReferencesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	limit := req.Limit
	if limit == 0 {
		limit = 100
	}

	result, err := h.scipSvc.FindReferences(
		r.Context(),
		repoID,
		req.FilePath,
		req.Line,
		req.Column,
		limit,
	)
	if err != nil {
		http.Error(w, "Failed to find references: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := FindReferencesResponse{
		Found:      result != nil && len(result.References) > 0,
		References: []scip.Occurrence{}, // Non-nil empty slice
	}

	if result != nil {
		response.Symbol = result.Symbol
		response.Definition = result.Definition
		response.References = result.References

		response.TotalCount = result.TotalCount
		if response.References == nil {
			response.References = []scip.Occurrence{}
		}

		// Enrich with context (source lines) - need repo name
		repo, repoErr := h.services.Repos.GetRepository(r.Context(), repoID)
		if repoErr == nil && repo != nil {
			if response.Definition != nil {
				h.enrichOccurrenceWithContext(r.Context(), repo.Name, response.Definition)
			}

			h.enrichOccurrencesWithContext(r.Context(), repo.Name, response.References)
		}
	}

	writeJSON(w, response)
}

// IndexSCIPRequest is the request for indexing a repository.
type IndexSCIPRequest struct {
	Language string `json:"language,omitempty"` // Auto-detect if empty
}

// IndexSCIPResponse is the response from indexing.
type IndexSCIPResponse struct {
	Success     bool   `json:"success"`
	Language    string `json:"language"`
	Duration    string `json:"duration"`
	Files       int    `json:"files"`
	Symbols     int    `json:"symbols"`
	Occurrences int    `json:"occurrences"`
	Error       string `json:"error,omitempty"`
	Output      string `json:"output,omitempty"`
}

// IndexRepository runs SCIP indexing on a repository.
func (h *SCIPHandler) IndexRepository(w http.ResponseWriter, r *http.Request) {
	repoIDStr := chi.URLParam(r, "id")

	repoID, err := strconv.ParseInt(repoIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid repository ID", http.StatusBadRequest)
		return
	}

	var req IndexSCIPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get repository info
	repo, err := h.services.Repos.GetRepository(r.Context(), repoID)
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

	// Get connection to access code host credentials
	conn, err := h.services.Repos.GetConnection(r.Context(), repo.ConnectionID)
	if err != nil {
		http.Error(w, "Failed to get connection: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Build indexing options with auth credentials
	var opts *scip.IndexOptions
	if conn != nil && conn.Token != "" {
		opts = &scip.IndexOptions{
			CodeHostURL:  conn.URL,
			CodeHostType: conn.Type,
			Token:        conn.Token,
			ClearFirst:   true,
		}
	}

	// Get repository path
	repoPath := h.services.Files.GetRepoPath(repo.Name)

	// Run indexing with options
	result, err := h.scipSvc.IndexWithOptions(r.Context(), repoID, repoPath, req.Language, opts)

	response := IndexSCIPResponse{
		Success: result != nil && result.Success,
	}

	if result != nil {
		response.Language = result.Language
		response.Duration = result.Duration.String()
		response.Files = result.Files
		response.Symbols = result.Symbols
		response.Occurrences = result.Occurrences
		response.Error = result.Error
		response.Output = result.IndexerOutput
	} else if err != nil {
		response.Error = err.Error()
	}

	if !response.Success {
		writeJSONWithStatus(w, http.StatusInternalServerError, response)
	} else {
		writeJSON(w, response)
	}
}

// UploadSCIPIndex uploads a pre-built SCIP index.
func (h *SCIPHandler) UploadSCIPIndex(w http.ResponseWriter, r *http.Request) {
	repoIDStr := chi.URLParam(r, "id")

	repoID, err := strconv.ParseInt(repoIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid repository ID", http.StatusBadRequest)
		return
	}

	if !h.checkRepoAccess(w, r, repoID) {
		return
	}

	// Read the SCIP data from the request body
	data, err := io.ReadAll(io.LimitReader(r.Body, 100*1024*1024)) // 100MB limit
	if err != nil {
		http.Error(w, "Failed to read request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if len(data) == 0 {
		http.Error(w, "Empty request body", http.StatusBadRequest)
		return
	}

	// Import the SCIP data
	if err := h.scipSvc.ImportFromBytes(r.Context(), repoID, data); err != nil {
		http.Error(w, "Failed to import SCIP index: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get stats
	stats, err := h.scipSvc.GetStats(r.Context(), repoID)
	if err != nil {
		http.Error(
			w,
			"Index imported but failed to get stats: "+err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	writeJSON(w, map[string]any{
		"success": true,
		"message": "SCIP index uploaded successfully",
		"stats":   stats,
	})
}

// ClearSCIPIndex removes the SCIP index for a repository.
func (h *SCIPHandler) ClearSCIPIndex(w http.ResponseWriter, r *http.Request) {
	repoIDStr := chi.URLParam(r, "id")

	repoID, err := strconv.ParseInt(repoIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid repository ID", http.StatusBadRequest)
		return
	}

	if err := h.scipSvc.ClearIndex(r.Context(), repoID); err != nil {
		http.Error(w, "Failed to clear index: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"success": true,
		"message": "SCIP index cleared",
	})
}

// GetAvailableIndexers returns which SCIP indexers are available.
func (h *SCIPHandler) GetAvailableIndexers(w http.ResponseWriter, r *http.Request) {
	available := h.scipSvc.GetAvailableIndexers()
	instructions := scip.InstallInstructions()

	writeJSON(w, map[string]any{
		"available":    available,
		"supported":    h.scipSvc.SupportedLanguages(),
		"instructions": instructions,
	})
}

// SearchSymbolsRequest is the request for searching symbols.
type SearchSymbolsRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

// SearchSymbols searches for symbols by name.
func (h *SCIPHandler) SearchSymbols(w http.ResponseWriter, r *http.Request) {
	repoIDStr := chi.URLParam(r, "id")

	repoID, err := strconv.ParseInt(repoIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid repository ID", http.StatusBadRequest)
		return
	}

	var req SearchSymbolsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Query == "" {
		http.Error(w, "Query is required", http.StatusBadRequest)
		return
	}

	results, err := h.scipSvc.SearchSymbols(r.Context(), repoID, req.Query, req.Limit)
	if err != nil {
		http.Error(w, "Search failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"results": results,
		"count":   len(results),
	})
}

// ListFiles lists all indexed files for a repository.
func (h *SCIPHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	repoIDStr := chi.URLParam(r, "id")

	repoID, err := strconv.ParseInt(repoIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid repository ID", http.StatusBadRequest)
		return
	}

	limit := 100

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	files, err := h.scipSvc.ListFiles(r.Context(), repoID, limit)
	if err != nil {
		http.Error(w, "Failed to list files: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"files": files,
		"count": len(files),
	})
}
