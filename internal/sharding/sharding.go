// Package sharding provides consistent hashing for distributing repositories across shards.
package sharding

import (
	"hash/fnv"
	"os"
	"strconv"
	"strings"
)

// GetShardForRepo returns the shard index for a repository name
// using FNV-1a consistent hashing. This must be identical across
// all consumers (indexer, queue, federation, search) to ensure
// the same repo always maps to the same shard.
func GetShardForRepo(repoName string, totalShards int) int {
	if totalShards <= 1 {
		return 0
	}

	h := fnv.New32a()
	h.Write([]byte(repoName))

	return int(h.Sum32() % uint32(totalShards))
}

// Config holds sharding configuration read from environment.
type Config struct {
	ShardIndex  int
	TotalShards int
	Enabled     bool
}

// GetConfig reads sharding configuration from environment variables.
func GetConfig() Config {
	cfg := Config{
		ShardIndex:  0,
		TotalShards: 1,
		Enabled:     false,
	}

	if totalStr := os.Getenv("TOTAL_SHARDS"); totalStr != "" {
		if total, err := strconv.Atoi(totalStr); err == nil && total > 1 {
			cfg.TotalShards = total
			cfg.Enabled = true
		}
	}

	if indexStr := os.Getenv("SHARD_INDEX"); indexStr != "" {
		if idx, err := strconv.Atoi(indexStr); err == nil {
			cfg.ShardIndex = idx
		}
	} else {
		// Extract ordinal from POD_NAME (e.g., "code-search-indexer-2" -> 2)
		podName := os.Getenv("POD_NAME")
		if podName != "" {
			if idx := ExtractOrdinal(podName); idx >= 0 {
				cfg.ShardIndex = idx
			}
		}
	}

	return cfg
}

// ShouldHandleRepo returns true if the given shard should handle the repo.
func (c Config) ShouldHandleRepo(repoName string) bool {
	if !c.Enabled || c.TotalShards <= 1 {
		return true
	}

	return GetShardForRepo(repoName, c.TotalShards) == c.ShardIndex
}

// ExtractOrdinal extracts the ordinal index from a StatefulSet pod name.
func ExtractOrdinal(podName string) int {
	lastDash := strings.LastIndex(podName, "-")
	if lastDash == -1 || lastDash == len(podName)-1 {
		return -1
	}

	ordinal, err := strconv.Atoi(podName[lastDash+1:])
	if err != nil {
		return -1
	}

	return ordinal
}
