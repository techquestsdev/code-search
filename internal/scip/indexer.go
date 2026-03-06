package scip

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/techquestsdev/code-search/internal/languages"
)

// IndexerConfig holds configuration for SCIP indexers.
type IndexerConfig struct {
	// Paths to SCIP indexer binaries (optional, will look in PATH if not set)
	SCIPTypeScript string `yaml:"scip_typescript"`
	SCIPGo         string `yaml:"scip_go"`
	SCIPJava       string `yaml:"scip_java"`
	SCIPRust       string `yaml:"scip_rust"`
	SCIPPython     string `yaml:"scip_python"`
	// Note: PHP uses per-project installation via Composer (davidrjenni/scip-php)

	// Timeout for indexing operations
	Timeout time.Duration `yaml:"timeout"`

	// Working directory for temporary files
	WorkDir string `yaml:"work_dir"`
}

// DefaultIndexerConfig returns default configuration.
func DefaultIndexerConfig() IndexerConfig {
	return IndexerConfig{
		Timeout: 10 * time.Minute,
		WorkDir: os.TempDir(),
	}
}

// Indexer runs SCIP indexers for different languages.
type Indexer struct {
	config IndexerConfig
	store  *Store
	parser *Parser
	logger *zap.Logger
}

// NewIndexer creates a new SCIP indexer.
func NewIndexer(config IndexerConfig, store *Store, logger *zap.Logger) *Indexer {
	return &Indexer{
		config: config,
		store:  store,
		parser: NewParser(store),
		logger: logger,
	}
}

// IndexOptions contains optional parameters for indexing.
type IndexOptions struct {
	// CodeHostURL is the URL of the code host (e.g., https://gitlab.example.com)
	CodeHostURL string
	// CodeHostType is the type of code host (github, gitlab, gitea, bitbucket)
	CodeHostType string
	// Token is the authentication token for the code host
	Token string
	// ClearFirst controls whether to clear existing index data before importing.
	// When true (default behavior for single-language calls), the index is cleared
	// before importing new data. When false, new data is appended to the existing index.
	// This is used by the worker for multi-language indexing: clear once before the
	// first language, then append for subsequent languages.
	ClearFirst bool
}

// IndexResult contains the result of an indexing operation.
type IndexResult struct {
	Success       bool           `json:"success"`
	Language      string         `json:"language"`
	Duration      time.Duration  `json:"duration"`
	Files         int            `json:"files"`
	Symbols       int            `json:"symbols"`
	Occurrences   int            `json:"occurrences"`
	Error         string         `json:"error,omitempty"`
	IndexerOutput string         `json:"indexerOutput,omitempty"`
	Stats         map[string]any `json:"stats,omitempty"`
}

// checkoutToWorkDir checks out a bare git repo to a temporary working directory.
func (i *Indexer) checkoutToWorkDir(ctx context.Context, bareRepoPath string) (string, error) {
	// Create temp directory
	workDir, err := os.MkdirTemp(i.config.WorkDir, "scip-checkout-")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Clone from bare repo to working directory (local clone is fast)
	cmd := exec.CommandContext(ctx, "git", "clone", "--local", bareRepoPath, workDir)

	var stderr bytes.Buffer

	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		os.RemoveAll(workDir)
		return "", fmt.Errorf("failed to clone: %w: %s", err, stderr.String())
	}

	return workDir, nil
}

// SupportedLanguages returns the list of languages with SCIP indexer support.
func (i *Indexer) SupportedLanguages() []string {
	return []string{"go", "typescript", "javascript", "java", "rust", "python", "php"}
}

// DetectLanguage detects the primary language of a repository.
// It returns the highest-priority language that has at least one project directory.
func (i *Indexer) DetectLanguage(repoPath string) (string, error) {
	langs := i.DetectLanguages(repoPath)
	if len(langs) == 0 {
		return "", errors.New("could not detect language")
	}

	return langs[0], nil
}

// DetectLanguages detects all languages present in a repository.
// It returns languages in priority order (Go, TypeScript, JavaScript, ...).
func (i *Indexer) DetectLanguages(repoPath string) []string {
	var detected []string

	langs := scipLanguages()
	for _, lang := range languages.Priority() {
		cfg := langs[lang]
		locs := i.findAllProjectDirs(repoPath, cfg.Markers, cfg.IndependentDirs)
		// findAllProjectDirs returns a root fallback when nothing is found;
		// we only count a language as detected if a real marker was found.
		if len(locs) == 0 {
			continue
		}
		// A single entry with empty PathPrefix AND no marker at root means
		// the fallback was returned — no real project detected.
		if len(locs) == 1 && locs[0].PathPrefix == "" {
			// Check if root actually has a marker
			found := false

			for _, marker := range cfg.Markers {
				if _, err := os.Stat(filepath.Join(repoPath, marker)); err == nil {
					found = true
					break
				}
			}

			if !found {
				continue
			}
		}

		detected = append(detected, lang)
	}

	return detected
}

// Index runs the appropriate SCIP indexer for a repository.
func (i *Indexer) Index(
	ctx context.Context,
	repoID int64,
	repoPath string,
	language string,
) (*IndexResult, error) {
	return i.IndexWithOptions(ctx, repoID, repoPath, language, nil)
}

// IndexWithOptions runs the appropriate SCIP indexer for a repository with optional credentials.
func (i *Indexer) IndexWithOptions(
	ctx context.Context,
	repoID int64,
	repoPath string,
	language string,
	opts *IndexOptions,
) (*IndexResult, error) {
	start := time.Now()
	result := &IndexResult{
		Language: language,
	}

	// Check if this is a bare repo (ends in .git) and checkout to a working directory
	workDir := repoPath
	if strings.HasSuffix(repoPath, ".git") {
		var err error

		workDir, err = i.checkoutToWorkDir(ctx, repoPath)
		if err != nil {
			result.Error = fmt.Sprintf("failed to checkout repo: %v", err)
			return result, err
		}
		defer os.RemoveAll(workDir) // Clean up temp directory

		i.logger.Info("Checked out bare repo to working directory", zap.String("workDir", workDir))
	}

	// Detect language if not specified
	if language == "" {
		detected, err := i.DetectLanguage(workDir)
		if err != nil {
			result.Error = fmt.Sprintf("failed to detect language: %v", err)
			return result, err
		}

		language = detected
		result.Language = language
	}

	// Find all project directories for this language
	allLangs := scipLanguages()

	langCfg, ok := allLangs[language]
	if !ok {
		result.Error = "unsupported language: " + language
		return result, fmt.Errorf("unsupported language: %s", language)
	}

	projectDirs := i.findAllProjectDirs(workDir, langCfg.Markers, langCfg.IndependentDirs)

	i.logger.Info("Starting SCIP indexing",
		zap.Int64("repoID", repoID),
		zap.String("language", language),
		zap.String("path", workDir),
		zap.Int("projects", len(projectDirs)),
	)

	// Clear existing index before processing projects (unless caller opted out).
	// When opts is nil or ClearFirst is true, clear the index. The worker sets
	// ClearFirst=false for the second and subsequent languages in multi-language
	// indexing so that results accumulate across languages.
	clearFirst := opts == nil || opts.ClearFirst
	if clearFirst {
		if err := i.store.ClearIndex(ctx, repoID); err != nil {
			result.Error = fmt.Sprintf("failed to clear existing index: %v", err)
			result.Duration = time.Since(start)

			return result, err
		}
	}

	// Index each project directory and append results
	var allOutput []string

	for _, proj := range projectDirs {
		if proj.PathPrefix != "" {
			i.logger.Info("Indexing project in subdirectory",
				zap.String("language", language),
				zap.String("projectDir", proj.Dir),
				zap.String("pathPrefix", proj.PathPrefix),
			)
		}

		var (
			scipData      []byte
			indexerOutput string
			err           error
		)

		switch language {
		case "go":
			scipData, indexerOutput, err = i.indexGoProject(ctx, proj.Dir)
		case "typescript", "javascript":
			scipData, indexerOutput, err = i.indexTypeScriptProject(ctx, proj.Dir)
		case "java":
			scipData, indexerOutput, err = i.indexJavaProject(ctx, proj.Dir)
		case "rust":
			scipData, indexerOutput, err = i.indexRustProject(ctx, proj.Dir)
		case "python":
			scipData, indexerOutput, err = i.indexPythonProject(ctx, proj.Dir)
		case "php":
			scipData, indexerOutput, err = i.indexPHPProject(ctx, proj.Dir, opts)
		}

		if indexerOutput != "" {
			allOutput = append(allOutput, indexerOutput)
		}

		if err != nil {
			i.logger.Warn("Failed to index project, skipping",
				zap.Error(err),
				zap.String("language", language),
				zap.String("projectDir", proj.Dir),
				zap.String("pathPrefix", proj.PathPrefix),
			)

			// If this is the only project, fail the whole operation
			if len(projectDirs) == 1 {
				result.Error = err.Error()
				result.IndexerOutput = strings.Join(allOutput, "\n---\n")
				result.Duration = time.Since(start)

				return result, err
			}

			continue
		}

		// Append the SCIP data (without clearing — index was cleared once above)
		if err := i.parser.AppendFromBytesWithPrefix(ctx, repoID, scipData, proj.PathPrefix); err != nil {
			result.Error = fmt.Sprintf("failed to import SCIP data: %v", err)
			result.IndexerOutput = strings.Join(allOutput, "\n---\n")
			result.Duration = time.Since(start)

			return result, err
		}
	}

	result.IndexerOutput = strings.Join(allOutput, "\n---\n")

	// Get stats
	stats, err := i.store.Stats(ctx, repoID)
	if err == nil {
		result.Stats = stats
		if v, ok := stats["total_files"].(int); ok {
			result.Files = v
		}

		if v, ok := stats["total_symbols"].(int); ok {
			result.Symbols = v
		}

		if v, ok := stats["total_occurrences"].(int); ok {
			result.Occurrences = v
		}
	}

	result.Success = true
	result.Duration = time.Since(start)

	i.logger.Info("SCIP indexing completed",
		zap.Int64("repoID", repoID),
		zap.String("language", language),
		zap.Int("projects", len(projectDirs)),
		zap.Int("files", result.Files),
		zap.Int("symbols", result.Symbols),
		zap.Int("occurrences", result.Occurrences),
		zap.Duration("duration", result.Duration),
	)

	return result, nil
}

// projectLocation represents a discovered project directory within a repository.
type projectLocation struct {
	Dir        string // Absolute path to the project directory
	PathPrefix string // Relative path from repo root ("" if at root)
}

// scipLanguageConfig adds SCIP-specific project-discovery settings on top of
// the shared language markers.
type scipLanguageConfig struct {
	// IndependentDirs means each marker represents an independent project.
	// When true, all directories with markers are returned (even if root has one).
	// When false, a root-level marker short-circuits: the tool handles the whole
	// tree (e.g. TypeScript workspaces, Cargo workspaces, Maven multi-module).
	IndependentDirs bool
}

// scipLanguageOverrides stores the IndependentDirs flag per language.
// Languages not listed here default to IndependentDirs=false.
var scipLanguageOverrides = map[string]scipLanguageConfig{
	"go":  {IndependentDirs: true},
	"php": {IndependentDirs: true},
}

// scipLanguages builds the full map used by the indexer: shared markers +
// per-language IndependentDirs override.
func scipLanguages() map[string]struct {
	Markers         []string
	IndependentDirs bool
} {
	markers := languages.MarkersByLanguage()

	result := make(map[string]struct {
		Markers         []string
		IndependentDirs bool
	}, len(markers))
	for lang, m := range markers {
		override := scipLanguageOverrides[lang]
		result[lang] = struct {
			Markers         []string
			IndependentDirs bool
		}{Markers: m, IndependentDirs: override.IndependentDirs}
	}

	return result
}

// findAllProjectDirs searches for all project root directories by looking for marker
// files. It checks the repo root first, then walks subdirectories up to 3
// levels deep. Returns all discovered project locations.
//
// When independentDirs is false (e.g. TypeScript, Java, Rust): if a marker is
// found at root, only the root is returned — the tool handles the whole tree.
// When independentDirs is true (e.g. Go, PHP): all directories with markers are
// returned, including root, because each marker is an independent project.
//
// If no markers are found, returns a single entry with the repo root and empty prefix.
func (i *Indexer) findAllProjectDirs(repoPath string, markerFiles []string, independentDirs bool) []projectLocation {
	// Check if any marker exists at the root
	rootHasMarker := false

	for _, marker := range markerFiles {
		if _, err := os.Stat(filepath.Join(repoPath, marker)); err == nil {
			rootHasMarker = true
			break
		}
	}

	// For non-independent languages, root marker short-circuits
	if rootHasMarker && !independentDirs {
		return []projectLocation{{Dir: repoPath, PathPrefix: ""}}
	}

	// Build a set for fast lookup
	markerSet := make(map[string]bool, len(markerFiles))
	for _, m := range markerFiles {
		markerSet[m] = true
	}

	// Track which directories we've already found to avoid duplicates
	// (e.g., a directory containing both pyproject.toml and requirements.txt)
	seen := make(map[string]bool)

	var results []projectLocation

	// Include root if it has a marker (for independent-dir languages)
	if rootHasMarker && independentDirs {
		seen[repoPath] = true
		results = append(results, projectLocation{Dir: repoPath, PathPrefix: ""})
	}

	// Search subdirectories (up to 3 levels deep)
	_ = filepath.WalkDir(repoPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			// Skip common non-source directories
			name := d.Name()
			if name == ".git" || name == "vendor" || name == "node_modules" ||
				name == "testdata" || name == ".next" || name == "__pycache__" || name == "target" {
				return filepath.SkipDir
			}
			// Check depth - don't go too deep
			rel, _ := filepath.Rel(repoPath, path)
			if strings.Count(rel, string(os.PathSeparator)) > 3 {
				return filepath.SkipDir
			}
		}

		if !d.IsDir() && markerSet[d.Name()] {
			dir := filepath.Dir(path)
			if !seen[dir] {
				seen[dir] = true
				prefix, _ := filepath.Rel(repoPath, dir)
				results = append(results, projectLocation{Dir: dir, PathPrefix: prefix})
			}
		}

		return nil
	})

	if len(results) == 0 {
		return []projectLocation{{Dir: repoPath, PathPrefix: ""}}
	}

	return results
}

// indexGoProject indexes a single Go project at the given directory.
func (i *Indexer) indexGoProject(ctx context.Context, projectDir string) ([]byte, string, error) {
	// Check for scip-go
	scipGo := i.config.SCIPGo
	if scipGo == "" {
		scipGo = "scip-go"
	}

	if _, err := exec.LookPath(scipGo); err != nil {
		return nil, "", errors.New(
			"scip-go not found in PATH. Install with: go install github.com/sourcegraph/scip-go@latest",
		)
	}

	outputFile := filepath.Join(
		i.config.WorkDir,
		fmt.Sprintf("index-%d.scip", time.Now().UnixNano()),
	)
	defer os.Remove(outputFile)

	ctx, cancel := context.WithTimeout(ctx, i.config.Timeout)
	defer cancel()

	// Use explicit paths to avoid "project root is outside the repository" error
	cmd := exec.CommandContext(ctx, scipGo,
		"--output", outputFile,
		"--project-root", projectDir,
		"--module-root", projectDir,
		"--repository-root", projectDir,
	)
	cmd.Dir = projectDir

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		output := stderr.String() + stdout.String()
		return nil, output, fmt.Errorf("scip-go failed: %w\n%s", err, output)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		return nil, stdout.String(), fmt.Errorf("failed to read SCIP output: %w", err)
	}

	return data, stdout.String() + stderr.String(), nil
}

// indexTypeScriptProject indexes a single TypeScript/JavaScript project at the given directory.
func (i *Indexer) indexTypeScriptProject(ctx context.Context, projectDir string) ([]byte, string, error) {
	// Check for scip-typescript via npx
	scipTS := i.config.SCIPTypeScript
	if scipTS == "" {
		scipTS = "scip-typescript"
	}

	outputFile := filepath.Join(
		i.config.WorkDir,
		fmt.Sprintf("index-%d.scip", time.Now().UnixNano()),
	)
	defer os.Remove(outputFile)

	ctx, cancel := context.WithTimeout(ctx, i.config.Timeout)
	defer cancel()

	// Try npx first
	var cmd *exec.Cmd
	if _, err := exec.LookPath("npx"); err == nil {
		cmd = exec.CommandContext(
			ctx,
			"npx",
			"-y",
			"@sourcegraph/scip-typescript",
			"index",
			"--output",
			outputFile,
		)
	} else if _, err := exec.LookPath(scipTS); err == nil {
		cmd = exec.CommandContext(ctx, scipTS, "index", "--output", outputFile)
	} else {
		return nil, "", errors.New("scip-typescript not found. Install with: npm install -g @sourcegraph/scip-typescript")
	}

	cmd.Dir = projectDir

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		output := stderr.String() + stdout.String()
		return nil, output, fmt.Errorf("scip-typescript failed: %w\n%s", err, output)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		return nil, stdout.String(), fmt.Errorf("failed to read SCIP output: %w", err)
	}

	return data, stdout.String() + stderr.String(), nil
}

// indexJavaProject indexes a single Java project at the given directory.
func (i *Indexer) indexJavaProject(ctx context.Context, projectDir string) ([]byte, string, error) {
	// Check for scip-java
	scipJava := i.config.SCIPJava
	if scipJava == "" {
		scipJava = "scip-java"
	}

	if _, err := exec.LookPath(scipJava); err != nil {
		return nil, "", errors.New(
			"scip-java not found in PATH. Install from: https://sourcegraph.github.io/scip-java/",
		)
	}

	outputFile := filepath.Join(
		i.config.WorkDir,
		fmt.Sprintf("index-%d.scip", time.Now().UnixNano()),
	)
	defer os.Remove(outputFile)

	ctx, cancel := context.WithTimeout(ctx, i.config.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, scipJava, "index", "--output", outputFile)
	cmd.Dir = projectDir

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		output := stderr.String() + stdout.String()
		return nil, output, fmt.Errorf("scip-java failed: %w\n%s", err, output)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		return nil, stdout.String(), fmt.Errorf("failed to read SCIP output: %w", err)
	}

	return data, stdout.String() + stderr.String(), nil
}

// indexRustProject indexes a single Rust project at the given directory.
func (i *Indexer) indexRustProject(ctx context.Context, projectDir string) ([]byte, string, error) {
	// rust-analyzer can generate SCIP output
	if _, err := exec.LookPath("rust-analyzer"); err != nil {
		return nil, "", errors.New(
			"rust-analyzer not found in PATH. Install with: rustup component add rust-analyzer",
		)
	}

	outputFile := filepath.Join(
		i.config.WorkDir,
		fmt.Sprintf("index-%d.scip", time.Now().UnixNano()),
	)
	defer os.Remove(outputFile)

	ctx, cancel := context.WithTimeout(ctx, i.config.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "rust-analyzer", "scip", ".", "--output", outputFile)
	cmd.Dir = projectDir

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		output := stderr.String() + stdout.String()
		return nil, output, fmt.Errorf("rust-analyzer scip failed: %w\n%s", err, output)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		return nil, stdout.String(), fmt.Errorf("failed to read SCIP output: %w", err)
	}

	return data, stdout.String() + stderr.String(), nil
}

// indexPythonProject indexes a single Python project at the given directory.
func (i *Indexer) indexPythonProject(ctx context.Context, projectDir string) ([]byte, string, error) {
	// Check for scip-python
	scipPython := i.config.SCIPPython
	if scipPython == "" {
		scipPython = "scip-python"
	}

	if _, err := exec.LookPath(scipPython); err != nil {
		// Try pip install
		return nil, "", errors.New(
			"scip-python not found in PATH. Install with: pip install scip-python",
		)
	}

	outputFile := filepath.Join(
		i.config.WorkDir,
		fmt.Sprintf("index-%d.scip", time.Now().UnixNano()),
	)
	defer os.Remove(outputFile)

	ctx, cancel := context.WithTimeout(ctx, i.config.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, scipPython, "index", ".", "--output", outputFile)
	cmd.Dir = projectDir

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		output := stderr.String() + stdout.String()
		return nil, output, fmt.Errorf("scip-python failed: %w\n%s", err, output)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		return nil, stdout.String(), fmt.Errorf("failed to read SCIP output: %w", err)
	}

	return data, stdout.String() + stderr.String(), nil
}

// indexPHPProject indexes a single PHP project at the given directory using scip-php.
// See: https://github.com/davidrjenni/scip-php
//
// IMPORTANT: scip-php must be installed as a dev dependency in the project itself.
// Due to strict version requirements (nikic/php-parser ^4.15), it cannot be
// automatically installed - it must be added by the project maintainers:
//
//	composer require --dev davidrjenni/scip-php
//
// Requirements:
// 1. PHP runtime in PATH
// 2. Composer in PATH
// 3. davidrjenni/scip-php installed in the project's vendor
//
// If opts contains a Token, it will be used for Composer authentication.
func (i *Indexer) indexPHPProject(
	ctx context.Context,
	projectDir string,
	opts *IndexOptions,
) ([]byte, string, error) {
	i.logger.Info("Starting PHP SCIP indexing", zap.String("projectDir", projectDir))

	// List files in the project path for debugging
	entries, _ := os.ReadDir(projectDir)

	var files []string
	for _, e := range entries {
		files = append(files, e.Name())
	}

	i.logger.Debug("Files in project path", zap.Strings("files", files), zap.String("projectDir", projectDir))

	// scip-php requires composer.json - check if it exists
	composerJSON := filepath.Join(projectDir, "composer.json")
	if _, err := os.Stat(composerJSON); os.IsNotExist(err) {
		return nil, "", fmt.Errorf(
			"composer.json not found at %s - scip-php requires a Composer-managed PHP project. Files found: %v",
			composerJSON,
			files,
		)
	}

	// Build composer auth environment if token provided
	var composerEnv []string

	if opts != nil && opts.Token != "" && opts.CodeHostURL != "" {
		// Extract host from URL for COMPOSER_AUTH
		host := opts.CodeHostURL
		host = strings.TrimPrefix(host, "https://")
		host = strings.TrimPrefix(host, "http://")
		host = strings.TrimSuffix(host, "/")

		// Build COMPOSER_AUTH JSON based on code host type
		var composerAuth string

		switch opts.CodeHostType {
		case "gitlab":
			// GitLab uses gitlab-token auth
			composerAuth = fmt.Sprintf(`{"gitlab-token": {"%s": "%s"}}`, host, opts.Token)
		case "github":
			// GitHub uses github-oauth
			composerAuth = fmt.Sprintf(`{"github-oauth": {"%s": "%s"}}`, host, opts.Token)
		default:
			// Generic http-basic auth
			composerAuth = fmt.Sprintf(
				`{"http-basic": {"%s": {"username": "token", "password": "%s"}}}`,
				host,
				opts.Token,
			)
		}

		composerEnv = append(os.Environ(), "COMPOSER_AUTH="+composerAuth)

		i.logger.Debug("Using Composer auth for PHP indexing",
			zap.String("host", host),
			zap.String("type", opts.CodeHostType),
		)
	} else {
		composerEnv = os.Environ()
	}

	// Run composer install to get project dependencies
	i.logger.Info("Running composer install for PHP SCIP indexing", zap.String("projectDir", projectDir))

	installCmd := exec.CommandContext(
		ctx,
		"composer",
		"install",
		"--no-interaction",
		"--no-progress",
		"--ignore-platform-reqs",
	)
	installCmd.Dir = projectDir
	installCmd.Env = composerEnv

	var installOut bytes.Buffer

	installCmd.Stdout = &installOut

	installCmd.Stderr = &installOut
	if err := installCmd.Run(); err != nil {
		i.logger.Warn("composer install failed", zap.Error(err), zap.String("output", installOut.String()))
	}

	// Check if scip-php is installed in the project
	scipPHP := filepath.Join(projectDir, "vendor/bin/scip-php")
	if _, err := os.Stat(scipPHP); os.IsNotExist(err) {
		return nil, "", errors.New(
			"scip-php not found in project. PHP SCIP indexing requires the project to have davidrjenni/scip-php as a dev dependency. Add it with: composer require --dev davidrjenni/scip-php",
		)
	}

	ctx, cancel := context.WithTimeout(ctx, i.config.Timeout)
	defer cancel()

	// Run scip-php from the project directory
	cmd := exec.CommandContext(ctx, scipPHP)
	cmd.Dir = projectDir
	// Suppress PHP deprecation warnings
	cmd.Env = append(os.Environ(), "PHP_CLI_SERVER_WORKERS=1")

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	i.logger.Info("Running scip-php", zap.String("projectDir", projectDir), zap.String("scipPHP", scipPHP))

	if err := cmd.Run(); err != nil {
		output := stderr.String() + stdout.String()
		return nil, output, fmt.Errorf("scip-php failed: %w\n%s", err, output)
	}

	// scip-php outputs to index.scip in the working directory
	defaultOutput := filepath.Join(projectDir, "index.scip")

	data, err := os.ReadFile(defaultOutput)
	if err != nil {
		return nil, stdout.String(), fmt.Errorf(
			"failed to read SCIP output from %s: %w",
			defaultOutput,
			err,
		)
	}

	// Clean up the index.scip file from the project
	os.Remove(defaultOutput)

	return data, stdout.String() + stderr.String(), nil
}

// GetAvailableIndexers returns which indexers are available on the system.
func (i *Indexer) GetAvailableIndexers() map[string]bool {
	available := make(map[string]bool)

	// Check Go
	scipGo := i.config.SCIPGo
	if scipGo == "" {
		scipGo = "scip-go"
	}

	_, err := exec.LookPath(scipGo)
	available["go"] = err == nil

	// Check TypeScript (via npx)
	_, err = exec.LookPath("npx")
	available["typescript"] = err == nil
	available["javascript"] = err == nil

	// Check Java
	scipJava := i.config.SCIPJava
	if scipJava == "" {
		scipJava = "scip-java"
	}

	_, err = exec.LookPath(scipJava)
	available["java"] = err == nil

	// Check Rust
	_, err = exec.LookPath("rust-analyzer")
	available["rust"] = err == nil

	// Check Python
	scipPython := i.config.SCIPPython
	if scipPython == "" {
		scipPython = "scip-python"
	}

	_, err = exec.LookPath(scipPython)
	available["python"] = err == nil

	// Check PHP - look for scip-php in PATH or via env var
	scipPHP := os.Getenv("SCIP_PHP_PATH")
	if scipPHP == "" {
		scipPHP = "scip-php"
	}

	_, err = exec.LookPath(scipPHP)
	if err != nil {
		// Also check /usr/local/bin
		_, err = os.Stat("/usr/local/bin/scip-php")
	}

	available["php"] = err == nil

	return available
}

// InstallInstructions returns instructions for installing SCIP indexers.
func InstallInstructions() map[string]string {
	return map[string]string{
		"go":         "go install github.com/sourcegraph/scip-go@latest",
		"typescript": "npm install -g @sourcegraph/scip-typescript",
		"javascript": "npm install -g @sourcegraph/scip-typescript",
		"java":       "See https://sourcegraph.github.io/scip-java/",
		"rust":       "rustup component add rust-analyzer",
		"python":     "pip install scip-python",
		"php":        "composer global require davidrjenni/scip-php (or set SCIP_PHP_PATH)",
	}
}

// IsIndexerAvailable checks if an indexer is available for a language.
func (i *Indexer) IsIndexerAvailable(language string) bool {
	available := i.GetAvailableIndexers()
	return available[strings.ToLower(language)]
}
