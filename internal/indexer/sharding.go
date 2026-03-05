package indexer

import (
	"github.com/aanogueira/code-search/internal/sharding"
)

// ShardConfig holds sharding configuration.
// Delegates to the shared sharding package.
type ShardConfig = sharding.Config

// GetShardConfig reads sharding configuration from environment.
func GetShardConfig() ShardConfig {
	return sharding.GetConfig()
}

// GetShardForRepo returns the shard index for a repository name.
func GetShardForRepo(repoName string, totalShards int) int {
	return sharding.GetShardForRepo(repoName, totalShards)
}
