package handlers

import (
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/techquestsdev/code-search/internal/audit"
	"github.com/techquestsdev/code-search/internal/files"
	"github.com/techquestsdev/code-search/internal/middleware"
)

// TreeResponse represents the response for tree listing.
type TreeResponse struct {
	Entries []files.TreeEntry `json:"entries"`
	Path    string            `json:"path"`
	Ref     string            `json:"ref"`
}

// ListTree returns the directory listing for a repository.
// GET /api/v1/repos/by-id/{id}/tree?path=&ref=.
func (h *Handler) ListTree(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid repository ID", http.StatusBadRequest)
		return
	}

	// Get repository from database
	repo, err := h.services.Repos.GetRepository(r.Context(), id)
	if err != nil {
		http.Error(w, "Failed to get repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if repo == nil {
		http.Error(w, "Repository not found", http.StatusNotFound)
		return
	}

	user := middleware.UserFromContext(r.Context())
	if !h.services.Authorizer.CanAccessRepo(r.Context(), user, repo.Name) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	path := r.URL.Query().Get("path")
	ref := r.URL.Query().Get("ref")

	if ref == "" {
		ref = repo.DefaultBranch
		if ref == "" {
			ref = "HEAD"
		}
	}

	var entries []files.TreeEntry

	// Use federated client if available (sharded deployment)
	if h.services.FederatedFiles != nil {
		entries, err = h.services.FederatedFiles.ListTree(r.Context(), repo.Name, path, ref)
		if err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "unreachable") {
				http.Error(w, "Indexer shard unavailable: "+errStr, http.StatusServiceUnavailable)
				return
			}

			if strings.Contains(errStr, "404") || strings.Contains(errStr, "not found") {
				http.Error(w, "Repository not cloned locally", http.StatusNotFound)
				return
			}

			http.Error(w, "Failed to list tree: "+err.Error(), http.StatusInternalServerError)

			return
		}
	} else {
		// Local file access
		if !h.services.Files.RepoExists(repo.Name) {
			http.Error(w, "Repository not cloned locally", http.StatusNotFound)
			return
		}

		entries, err = h.services.Files.ListTree(r.Context(), repo.Name, path, ref)
		if err != nil {
			// Check if it's a "path not found" error
			errStr := err.Error()
			if strings.Contains(errStr, "navigate to path") || strings.Contains(errStr, "not found") || strings.Contains(errStr, "directory not found") {
				http.Error(w, "Path not found: "+path, http.StatusNotFound)
				return
			}

			http.Error(w, "Failed to list tree: "+err.Error(), http.StatusInternalServerError)

			return
		}
	}

	h.services.AuditLogger.LogAccess(r.Context(), audit.AccessEvent{
		UserID:    user.ID,
		RepoName:  repo.Name,
		FilePath:  path,
		Action:    "browse",
		ClientIP:  r.RemoteAddr,
		Timestamp: time.Now(),
	})

	response := TreeResponse{
		Entries: entries,
		Path:    path,
		Ref:     ref,
	}

	writeJSON(w, response)
}

// BlobResponse represents the response for file content.
type BlobAPIResponse struct {
	Content      string `json:"content"`
	Path         string `json:"path"`
	Size         int64  `json:"size"`
	Language     string `json:"language"`
	LanguageMode string `json:"language_mode"` // CodeMirror mode
	Binary       bool   `json:"binary"`
	Ref          string `json:"ref"`
}

// GetBlob returns the content of a file in a repository.
// GET /api/v1/repos/by-id/{id}/blob?path=&ref=.
func (h *Handler) GetBlob(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid repository ID", http.StatusBadRequest)
		return
	}

	// Get repository from database
	repo, err := h.services.Repos.GetRepository(r.Context(), id)
	if err != nil {
		http.Error(w, "Failed to get repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if repo == nil {
		http.Error(w, "Repository not found", http.StatusNotFound)
		return
	}

	user := middleware.UserFromContext(r.Context())
	if !h.services.Authorizer.CanAccessRepo(r.Context(), user, repo.Name) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "Path is required", http.StatusBadRequest)
		return
	}

	ref := r.URL.Query().Get("ref")
	if ref == "" {
		ref = repo.DefaultBranch
		if ref == "" {
			ref = "HEAD"
		}
	}

	var blob *files.BlobResponse

	// Use federated client if available (sharded deployment)
	if h.services.FederatedFiles != nil {
		blob, err = h.services.FederatedFiles.GetBlob(r.Context(), repo.Name, path, ref)
		if err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "unreachable") {
				http.Error(w, "Indexer shard unavailable: "+errStr, http.StatusServiceUnavailable)
				return
			}

			if strings.Contains(errStr, "404") || strings.Contains(errStr, "not found") {
				http.Error(w, "File not found", http.StatusNotFound)
				return
			}

			http.Error(w, "Failed to get file: "+err.Error(), http.StatusInternalServerError)

			return
		}
	} else {
		// Local file access
		if !h.services.Files.RepoExists(repo.Name) {
			http.Error(w, "Repository not cloned locally", http.StatusNotFound)
			return
		}

		blob, err = h.services.Files.GetBlob(r.Context(), repo.Name, path, ref)
		if err != nil {
			http.Error(w, "Failed to get file: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	h.services.AuditLogger.LogAccess(r.Context(), audit.AccessEvent{
		UserID:    user.ID,
		RepoName:  repo.Name,
		FilePath:  path,
		Action:    "view_blob",
		ClientIP:  r.RemoteAddr,
		Timestamp: time.Now(),
	})

	response := BlobAPIResponse{
		Content:      blob.Content,
		Path:         blob.Path,
		Size:         blob.Size,
		Language:     blob.Language,
		LanguageMode: files.MapToCodeMirrorMode(blob.Language),
		Binary:       blob.Binary,
		Ref:          ref,
	}

	writeJSON(w, response)
}

// GetRaw returns the raw content of a file (including binary files).
// GET /api/v1/repos/by-id/{id}/raw?path=&ref=.
func (h *Handler) GetRaw(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid repository ID", http.StatusBadRequest)
		return
	}

	// Get repository from database
	repo, err := h.services.Repos.GetRepository(r.Context(), id)
	if err != nil {
		http.Error(w, "Failed to get repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if repo == nil {
		http.Error(w, "Repository not found", http.StatusNotFound)
		return
	}

	user := middleware.UserFromContext(r.Context())
	if !h.services.Authorizer.CanAccessRepo(r.Context(), user, repo.Name) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "Path is required", http.StatusBadRequest)
		return
	}

	ref := r.URL.Query().Get("ref")
	if ref == "" {
		ref = repo.DefaultBranch
		if ref == "" {
			ref = "HEAD"
		}
	}

	var (
		reader io.ReadCloser
		size   int64
	)

	// Use federated client if available (sharded deployment)
	if h.services.FederatedFiles != nil {
		reader, size, err = h.services.FederatedFiles.GetRawBlob(r.Context(), repo.Name, path, ref)
		if err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "unreachable") {
				http.Error(w, "Indexer shard unavailable: "+errStr, http.StatusServiceUnavailable)
				return
			}

			if strings.Contains(errStr, "404") || strings.Contains(errStr, "not found") {
				http.Error(w, "File not found", http.StatusNotFound)
				return
			}

			http.Error(w, "Failed to get file: "+err.Error(), http.StatusInternalServerError)

			return
		}
	} else {
		// Local file access
		if !h.services.Files.RepoExists(repo.Name) {
			http.Error(w, "Repository not cloned locally", http.StatusNotFound)
			return
		}

		reader, size, err = h.services.Files.GetRawBlob(r.Context(), repo.Name, path, ref)
		if err != nil {
			http.Error(w, "Failed to get file: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	defer reader.Close()

	h.services.AuditLogger.LogAccess(r.Context(), audit.AccessEvent{
		UserID:    user.ID,
		RepoName:  repo.Name,
		FilePath:  path,
		Action:    "raw_download",
		ClientIP:  r.RemoteAddr,
		Timestamp: time.Now(),
	})

	// Determine content type from extension
	ext := filepath.Ext(path)

	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Set headers
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("Content-Disposition", "inline; filename=\""+filepath.Base(path)+"\"")

	// Stream the content
	io.Copy(w, reader)
}

// BranchesResponse represents the response for branches listing.
type BranchesResponse struct {
	Branches      []string `json:"branches"`
	Tags          []string `json:"tags"`
	DefaultBranch string   `json:"default_branch"`
}

// FileSymbol represents a symbol found in a file.
type FileSymbol struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	Signature string `json:"signature,omitempty"`
	Parent    string `json:"parent,omitempty"`
}

// FileSymbolsResponse represents the response for symbols in a file.
type FileSymbolsResponse struct {
	Symbols []FileSymbol `json:"symbols"`
	Path    string       `json:"path"`
}

// GetBranchesAndTags returns branches and tags for a repository.
// GET /api/v1/repos/by-id/{id}/refs.
func (h *Handler) GetBranchesAndTags(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid repository ID", http.StatusBadRequest)
		return
	}

	// Get repository from database
	repo, err := h.services.Repos.GetRepository(r.Context(), id)
	if err != nil {
		http.Error(w, "Failed to get repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if repo == nil {
		http.Error(w, "Repository not found", http.StatusNotFound)
		return
	}

	if !h.services.Authorizer.CanAccessRepo(
		r.Context(),
		middleware.UserFromContext(r.Context()),
		repo.Name,
	) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var (
		branches []string
		tags     []string
	)

	// Use federated client if available (sharded deployment)

	if h.services.FederatedFiles != nil {
		branches, err = h.services.FederatedFiles.GetBranches(r.Context(), repo.Name)
		if err != nil {
			branches = []string{}
		}

		tags, err = h.services.FederatedFiles.GetTags(r.Context(), repo.Name)
		if err != nil {
			tags = []string{}
		}
	} else {
		// Local file access
		if !h.services.Files.RepoExists(repo.Name) {
			http.Error(w, "Repository not cloned locally", http.StatusNotFound)
			return
		}

		branches, err = h.services.Files.GetBranches(r.Context(), repo.Name)
		if err != nil {
			branches = []string{}
		}

		tags, err = h.services.Files.GetTags(r.Context(), repo.Name)
		if err != nil {
			tags = []string{}
		}
	}

	response := BranchesResponse{
		Branches:      branches,
		Tags:          tags,
		DefaultBranch: repo.DefaultBranch,
	}

	writeJSON(w, response)
}

// GetFileSymbols returns symbols in a specific file using Tree-sitter.
// GET /api/v1/repos/by-id/{id}/symbols?path=&ref=.
func (h *Handler) GetFileSymbols(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid repository ID", http.StatusBadRequest)
		return
	}

	// Get repository from database
	repo, err := h.services.Repos.GetRepository(r.Context(), id)
	if err != nil {
		http.Error(w, "Failed to get repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if repo == nil {
		http.Error(w, "Repository not found", http.StatusNotFound)
		return
	}

	if !h.services.Authorizer.CanAccessRepo(
		r.Context(),
		middleware.UserFromContext(r.Context()),
		repo.Name,
	) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "Path is required", http.StatusBadRequest)
		return
	}

	ref := r.URL.Query().Get("ref")
	if ref == "" {
		ref = repo.DefaultBranch
		if ref == "" {
			ref = "HEAD"
		}
	}

	// Get file content first (federated or local)
	var blob *files.BlobResponse

	if h.services.FederatedFiles != nil {
		blob, err = h.services.FederatedFiles.GetBlob(r.Context(), repo.Name, path, ref)
		if err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "unreachable") {
				http.Error(w, "Indexer shard unavailable: "+errStr, http.StatusServiceUnavailable)
				return
			}

			http.Error(w, "Failed to get file: "+err.Error(), http.StatusInternalServerError)

			return
		}
	} else {
		blob, err = h.services.Files.GetBlob(r.Context(), repo.Name, path, ref)
		if err != nil {
			http.Error(w, "Failed to get file: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if blob.Binary {
		// Return empty symbols for binary files
		response := FileSymbolsResponse{
			Symbols: []FileSymbol{},
			Path:    path,
		}
		writeJSON(w, response)

		return
	}

	// Get commit SHA for caching (use ref as approximation)
	commitSHA := ref

	// Extract symbols using Tree-sitter with caching
	symbols, err := h.services.Symbols.ExtractSymbols(
		r.Context(),
		id,
		path,
		commitSHA,
		[]byte(blob.Content),
		blob.Language,
	)
	if err != nil || len(symbols) == 0 {
		// Fall back to regex-based extraction for unsupported languages or errors
		regexSymbols := files.ExtractSymbols(blob.Content, blob.Language)

		fileSymbols := make([]FileSymbol, 0, len(regexSymbols))
		for _, s := range regexSymbols {
			fileSymbols = append(fileSymbols, FileSymbol{
				Name:      s.Name,
				Kind:      s.Kind,
				Line:      s.Line,
				Column:    s.Column,
				Signature: s.Signature,
				Parent:    s.Parent,
			})
		}

		response := FileSymbolsResponse{
			Symbols: fileSymbols,
			Path:    path,
		}
		writeJSON(w, response)

		return
	}

	// Convert to API response format
	fileSymbols := make([]FileSymbol, 0, len(symbols))
	for _, s := range symbols {
		fileSymbols = append(fileSymbols, FileSymbol{
			Name:      s.Name,
			Kind:      s.Kind,
			Line:      s.Line,
			Column:    s.Column,
			Signature: s.Signature,
			Parent:    s.Parent,
		})
	}

	response := FileSymbolsResponse{
		Symbols: fileSymbols,
		Path:    path,
	}

	writeJSON(w, response)
}
