package config

import (
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"

	"github.com/aanogueira/code-search/internal/log"
)

// LoadSecretsFromDir loads environment variables from files in a directory.
// Each file name becomes the environment variable name, and the file content becomes the value.
// This is useful for Kubernetes secrets mounted as files.
//
// Example:
//
//	/etc/secrets/GITHUB_TOKEN contains "ghp_xxxx"
//	After calling LoadSecretsFromDir("/etc/secrets"), os.Getenv("GITHUB_TOKEN") returns "ghp_xxxx"
//
// Options:
//   - prefix: Optional prefix to add to env var names (e.g., "CS_" makes GITHUB_TOKEN -> CS_GITHUB_TOKEN)
//   - overwrite: If true, overwrites existing env vars; if false, skips if already set
func LoadSecretsFromDir(dir string, prefix string, overwrite bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			// Directory doesn't exist - this is not an error, just skip
			log.Debug("Secrets directory does not exist, skipping", zap.String("path", dir))
			return nil
		}

		return err
	}

	loaded := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Skip hidden files and common non-secret files
		name := entry.Name()
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "..") {
			continue
		}

		filePath := filepath.Join(dir, name)

		content, err := os.ReadFile(filePath)
		if err != nil {
			log.Warn("Failed to read secret file", zap.String("path", filePath), zap.Error(err))
			continue
		}

		// Trim whitespace (common issue with mounted secrets)
		value := strings.TrimSpace(string(content))

		// Build env var name
		envName := prefix + name

		// Check if already set
		if !overwrite && os.Getenv(envName) != "" {
			log.Debug("Env var already set, skipping", zap.String("name", envName))
			continue
		}

		if err := os.Setenv(envName, value); err != nil {
			log.Warn(
				"Failed to set env var from secret",
				zap.String("name", envName),
				zap.Error(err),
			)

			continue
		}

		loaded++

		log.Debug(
			"Loaded secret as env var",
			zap.String("name", envName),
			zap.String("source", filePath),
		)
	}

	if loaded > 0 {
		log.Info("Loaded secrets from directory", zap.String("path", dir), zap.Int("count", loaded))
	}

	return nil
}

// LoadSecretsFromPaths loads secrets from multiple directories.
// This is useful when you have secrets in multiple locations.
// Directories are processed in order, so later directories can override earlier ones if overwrite=true.
func LoadSecretsFromPaths(paths []string, prefix string, overwrite bool) error {
	for _, path := range paths {
		if path == "" {
			continue
		}

		err := LoadSecretsFromDir(path, prefix, overwrite)
		if err != nil {
			return err
		}
	}

	return nil
}

// DefaultSecretsPaths returns the default paths to check for secrets.
// These are common locations for Kubernetes secrets and Docker secrets.
func DefaultSecretsPaths() []string {
	return []string{
		"/etc/secrets", // Common Kubernetes secrets mount
		"/run/secrets", // Docker secrets
	}
}
