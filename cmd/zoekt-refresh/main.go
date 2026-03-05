// zoekt-refresh is a lightweight sidecar that triggers zoekt index reloads
// for network filesystems (CephFS, NFS, EFS) that don't propagate inotify events.
//
// It works by periodically "touching" the .zoekt shard files, which triggers
// the IN_ATTRIB inotify event that zoekt monitors for changes.
package main

import (
	"os"
	"path/filepath"
	"time"

	"github.com/techquestsdev/code-search/internal/log"
)

func main() {
	// Initialize logger
	if err := log.InitFromEnv(); err != nil {
		panic("failed to initialize logger: " + err.Error())
	}

	defer log.Sync()

	// INDEX_PATH or CS_INDEXER_INDEX_PATH specifies the Zoekt index directory
	indexPath := os.Getenv("INDEX_PATH")
	if indexPath == "" {
		indexPath = os.Getenv("CS_INDEXER_INDEX_PATH")
	}

	if indexPath == "" {
		indexPath = os.Getenv("CS_ZOEKT_INDEX_PATH")
	}

	if indexPath == "" {
		// Default fallback for container deployments
		indexPath = "/data/index"
	}

	intervalStr := os.Getenv("REFRESH_INTERVAL")
	if intervalStr == "" {
		intervalStr = "30s"
	}

	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		// Invalid interval config - use default and log warning, don't crash
		log.Warn("Invalid REFRESH_INTERVAL, using default",
			log.String("value", intervalStr),
			log.Err(err),
			log.String("default", "30s"),
		)

		interval = 30 * time.Second
	}

	log.Info("Starting zoekt-refresh",
		log.String("index_path", indexPath),
		log.String("interval", interval.String()),
	)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Track file modification times to detect actual changes
	lastMtimes := make(map[string]time.Time)

	// Track iterations for periodic status logging
	iteration := 0

	// Do an initial check immediately
	refreshIndex(indexPath, lastMtimes, iteration)
	iteration++

	for range ticker.C {
		refreshIndex(indexPath, lastMtimes, iteration)
		iteration++

		// Log heartbeat every 10 iterations (5 minutes at 30s interval)
		if iteration%10 == 0 {
			log.Info("Heartbeat",
				log.Int("iteration", iteration),
				log.Int("tracked_shards", len(lastMtimes)),
			)
		}
	}
}

// ensureDir creates the directory if it doesn't exist.
func ensureDir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Info("Index directory does not exist, creating it", log.String("path", path))
		return os.MkdirAll(path, 0o755)
	}

	return nil
}

func refreshIndex(indexPath string, lastMtimes map[string]time.Time, iteration int) {
	// Ensure the index directory exists (may not exist on fresh boot)
	// Do this on every refresh in case it gets deleted
	if err := ensureDir(indexPath); err != nil {
		log.Warn("Failed to ensure index directory exists", log.Err(err))
		return
	}

	entries, err := os.ReadDir(indexPath)
	if err != nil {
		// Log and continue - don't crash the sidecar
		log.Warn("Error reading index directory", log.Err(err))
		return
	}

	// Count total shard files
	totalShards := 0

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".zoekt" {
			totalShards++
		}
	}

	// No shard files yet - this is normal on fresh boot
	if totalShards == 0 {
		log.Debug("No shard files found", log.Int("iteration", iteration))
		return
	}

	changedFiles := 0
	currentFiles := make(map[string]bool)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process .zoekt shard files
		if filepath.Ext(entry.Name()) != ".zoekt" {
			continue
		}

		currentFiles[entry.Name()] = true

		path := filepath.Join(indexPath, entry.Name())

		info, err := entry.Info()
		if err != nil {
			continue
		}

		mtime := info.ModTime()
		lastMtime, seen := lastMtimes[entry.Name()]

		// If file is new or modified since last check, touch it to trigger inotify
		if !seen || mtime.After(lastMtime) {
			changedFiles++
			lastMtimes[entry.Name()] = mtime

			// Touch the file to trigger inotify event
			// We use Chtimes with the current atime but preserve mtime
			now := time.Now()
			if err := os.Chtimes(path, now, mtime); err != nil {
				log.Warn("Error touching shard file",
					log.String("file", entry.Name()),
					log.Err(err),
				)
			}
		}
	}

	// Clean up entries for deleted files
	deletedFiles := 0

	for name := range lastMtimes {
		if !currentFiles[name] {
			delete(lastMtimes, name)

			deletedFiles++
		}
	}

	if changedFiles > 0 || deletedFiles > 0 {
		log.Info("Refresh cycle completed",
			log.Int("iteration", iteration),
			log.Int("changed", changedFiles),
			log.Int("deleted", deletedFiles),
			log.Int("total_shards", totalShards),
		)
	} else {
		log.Debug("Refresh cycle completed, no changes",
			log.Int("iteration", iteration),
			log.Int("total_shards", totalShards),
		)
	}
}
