// Package files provides file browsing and content retrieval for Git repositories.
package files

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-enry/go-enry/v2"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Service provides file browsing operations for repositories.
type Service struct {
	reposPath string
}

// NewService creates a new file service.
func NewService(reposPath string) *Service {
	return &Service{
		reposPath: reposPath,
	}
}

// TreeEntry represents a file or directory in a repository tree.
type TreeEntry struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // "file" or "dir"
	Path     string `json:"path"`
	Size     int64  `json:"size,omitempty"`
	Language string `json:"language,omitempty"`
}

// BlobResponse represents file content.
type BlobResponse struct {
	Content  string `json:"content"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Language string `json:"language"`
	Binary   bool   `json:"binary"`
}

// getRepoPath constructs the local path to a cloned repository.
// Repository names like "apps/adr/registry" are stored as "apps_adr_registry.git"
// (matching the indexer's naming convention).
func (s *Service) getRepoPath(repoName string) string {
	// Convert slashes to underscores and add .git suffix (same as indexer)
	safeName := strings.ReplaceAll(repoName, "/", "_")
	return filepath.Join(s.reposPath, safeName+".git")
}

// GetRepoPath returns the local path to a cloned repository (public version).
func (s *Service) GetRepoPath(repoName string) string {
	return s.getRepoPath(repoName)
}

// RepoExists checks if a repository exists locally.
func (s *Service) RepoExists(repoName string) bool {
	repoPath := s.getRepoPath(repoName)
	// For bare repos (.git suffix), check for HEAD file directly
	_, err := os.Stat(filepath.Join(repoPath, "HEAD"))
	if err == nil {
		return true
	}
	// For non-bare repos, check for .git directory
	_, err = os.Stat(filepath.Join(repoPath, ".git"))

	return err == nil
}

// resolveRef resolves a ref string to a commit.
// Accepts: branch names, tag names, commit SHAs, "HEAD".
func (s *Service) resolveRef(repo *git.Repository, ref string) (*object.Commit, error) {
	if ref == "" || ref == "HEAD" {
		head, err := repo.Head()
		if err != nil {
			return nil, fmt.Errorf("get HEAD: %w", err)
		}

		return repo.CommitObject(head.Hash())
	}

	// Try as branch
	branchRef, err := repo.Reference(plumbing.NewBranchReferenceName(ref), true)
	if err == nil {
		return repo.CommitObject(branchRef.Hash())
	}

	// Try as tag
	tagRef, err := repo.Reference(plumbing.NewTagReferenceName(ref), true)
	if err == nil {
		// Could be lightweight or annotated tag
		tagObj, err := repo.TagObject(tagRef.Hash())
		if err == nil {
			// Annotated tag - get the commit it points to
			return tagObj.Commit()
		}
		// Lightweight tag - hash is the commit directly
		return repo.CommitObject(tagRef.Hash())
	}

	// Try as commit SHA
	hash := plumbing.NewHash(ref)
	if !hash.IsZero() {
		commit, err := repo.CommitObject(hash)
		if err == nil {
			return commit, nil
		}
	}

	// Try as remote branch (origin/main)
	if strings.Contains(ref, "/") {
		remoteRef, err := repo.Reference(
			plumbing.NewRemoteReferenceName("origin", strings.TrimPrefix(ref, "origin/")),
			true,
		)
		if err == nil {
			return repo.CommitObject(remoteRef.Hash())
		}
	}

	return nil, fmt.Errorf("could not resolve ref %q", ref)
}

// ListTree returns the directory listing for a given path in a repository.
func (s *Service) ListTree(ctx context.Context, repoName, path, ref string) ([]TreeEntry, error) {
	repoPath := s.getRepoPath(repoName)

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}

	// Resolve the reference
	commit, err := s.resolveRef(repo, ref)
	if err != nil {
		return nil, fmt.Errorf("resolve ref %q: %w", ref, err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("get tree: %w", err)
	}

	// If path is specified, navigate to that subtree
	if path != "" && path != "/" {
		path = strings.TrimPrefix(path, "/")
		path = strings.TrimSuffix(path, "/")

		subtree, err := tree.Tree(path)
		if err != nil {
			return nil, fmt.Errorf("navigate to path %q: %w", path, err)
		}

		tree = subtree
	}

	var entries []TreeEntry

	for _, entry := range tree.Entries {
		entryPath := entry.Name
		if path != "" && path != "/" {
			entryPath = path + "/" + entry.Name
		}

		te := TreeEntry{
			Name: entry.Name,
			Path: entryPath,
		}

		if entry.Mode.IsFile() {
			te.Type = "file"
			// Get file size
			file, err := tree.TreeEntryFile(&entry)
			if err == nil {
				te.Size = file.Size
			}
			// Detect language
			te.Language = DetectLanguage(entry.Name, nil)
		} else {
			te.Type = "dir"
		}

		entries = append(entries, te)
	}

	// Sort: directories first, then alphabetically
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Type != entries[j].Type {
			return entries[i].Type == "dir"
		}

		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	return entries, nil
}

// GetBlob returns the content of a file in a repository.
func (s *Service) GetBlob(ctx context.Context, repoName, path, ref string) (*BlobResponse, error) {
	repoPath := s.getRepoPath(repoName)

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}

	// Resolve the reference
	commit, err := s.resolveRef(repo, ref)
	if err != nil {
		return nil, fmt.Errorf("resolve ref %q: %w", ref, err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("get tree: %w", err)
	}

	// Navigate to the file
	path = strings.TrimPrefix(path, "/")

	file, err := tree.File(path)
	if err != nil {
		return nil, fmt.Errorf("get file %q: %w", path, err)
	}

	// Check if binary
	isBinary, err := file.IsBinary()
	if err != nil {
		return nil, fmt.Errorf("check binary: %w", err)
	}

	resp := &BlobResponse{
		Path: path,
		Size: file.Size,
	}

	if isBinary {
		resp.Binary = true
		resp.Content = "" // Don't return binary content
		resp.Language = "binary"

		return resp, nil
	}

	// Read content
	reader, err := file.Reader()
	if err != nil {
		return nil, fmt.Errorf("get reader: %w", err)
	}
	defer reader.Close()

	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read content: %w", err)
	}

	resp.Content = string(content)
	resp.Language = DetectLanguage(path, content)

	return resp, nil
}

// GetRawBlob returns a reader for raw file content (including binary files).
func (s *Service) GetRawBlob(
	ctx context.Context,
	repoName, path, ref string,
) (io.ReadCloser, int64, error) {
	repoPath := s.getRepoPath(repoName)

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, 0, fmt.Errorf("open repository: %w", err)
	}

	// Resolve the reference
	commit, err := s.resolveRef(repo, ref)
	if err != nil {
		return nil, 0, fmt.Errorf("resolve ref %q: %w", ref, err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, 0, fmt.Errorf("get tree: %w", err)
	}

	// Navigate to the file
	path = strings.TrimPrefix(path, "/")

	file, err := tree.File(path)
	if err != nil {
		return nil, 0, fmt.Errorf("get file %q: %w", path, err)
	}

	reader, err := file.Reader()
	if err != nil {
		return nil, 0, fmt.Errorf("get reader: %w", err)
	}

	return reader, file.Size, nil
}

// GetBranches returns all branches for a repository.
func (s *Service) GetBranches(ctx context.Context, repoName string) ([]string, error) {
	repoPath := s.getRepoPath(repoName)

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}

	refs, err := repo.Branches()
	if err != nil {
		return nil, fmt.Errorf("get branches: %w", err)
	}

	var branches []string

	err = refs.ForEach(func(ref *plumbing.Reference) error {
		branches = append(branches, ref.Name().Short())
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("iterate branches: %w", err)
	}

	sort.Strings(branches)

	return branches, nil
}

// GetTags returns all tags for a repository.
func (s *Service) GetTags(ctx context.Context, repoName string) ([]string, error) {
	repoPath := s.getRepoPath(repoName)

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}

	refs, err := repo.Tags()
	if err != nil {
		return nil, fmt.Errorf("get tags: %w", err)
	}

	var tags []string

	err = refs.ForEach(func(ref *plumbing.Reference) error {
		tags = append(tags, ref.Name().Short())
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("iterate tags: %w", err)
	}

	sort.Strings(tags)

	return tags, nil
}

// DetectLanguage detects the programming language of a file.
// Uses go-enry which is GitHub's linguist library ported to Go.
func DetectLanguage(filename string, content []byte) string {
	// Always pass content when available for accurate detection.
	// Some extensions like .rs are ambiguous (Rust, RenderScript, XML)
	// and enry needs content to disambiguate.
	lang := enry.GetLanguage(filename, content)
	if lang != "" {
		return lang
	}

	return "Text"
}

// MapToCodeMirrorMode maps enry language names to CodeMirror mode names.
func MapToCodeMirrorMode(language string) string {
	// Map common languages to their CodeMirror mode names
	modeMap := map[string]string{
		"JavaScript":      "javascript",
		"TypeScript":      "typescript",
		"JSX":             "jsx",
		"TSX":             "tsx",
		"Python":          "python",
		"Go":              "go",
		"Rust":            "rust",
		"Java":            "java",
		"C":               "cpp",
		"C++":             "cpp",
		"C#":              "csharp",
		"Ruby":            "ruby",
		"PHP":             "php",
		"Hack":            "php", // Hack is Facebook's PHP variant
		"Swift":           "swift",
		"Kotlin":          "kotlin",
		"Scala":           "scala",
		"SQL":             "sql",
		"HTML":            "html",
		"CSS":             "css",
		"SCSS":            "css",
		"Sass":            "css",
		"Less":            "css",
		"JSON":            "json",
		"YAML":            "yaml",
		"XML":             "xml",
		"Markdown":        "markdown",
		"Shell":           "shell",
		"Bash":            "shell",
		"PowerShell":      "powershell",
		"Dockerfile":      "dockerfile",
		"Makefile":        "text",
		"TOML":            "toml",
		"INI":             "properties",
		"Text":            "text",
		"Plain Text":      "text",
		"Objective-C":     "objectivec",
		"Objective-C++":   "objectivec",
		"Lua":             "lua",
		"Perl":            "perl",
		"R":               "r",
		"Haskell":         "haskell",
		"Elixir":          "elixir",
		"Erlang":          "erlang",
		"Clojure":         "clojure",
		"F#":              "fsharp",
		"OCaml":           "ocaml",
		"Dart":            "dart",
		"Vue":             "vue",
		"Svelte":          "svelte",
		"GraphQL":         "graphql",
		"Protocol Buffer": "protobuf",
		"Protobuf":        "protobuf",
	}

	if mode, ok := modeMap[language]; ok {
		return mode
	}

	// Default to text for unknown languages
	return "text"
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
