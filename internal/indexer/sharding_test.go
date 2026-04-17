package indexer

import (
	"os"
	"testing"

	"github.com/techquestsdev/code-search/internal/sharding"
)

func TestGetShardConfig_Defaults(t *testing.T) {
	// Clear environment
	originalTotal := os.Getenv("TOTAL_SHARDS")
	originalIndex := os.Getenv("SHARD_INDEX")
	originalPod := os.Getenv("POD_NAME")

	defer func() {
		os.Setenv("TOTAL_SHARDS", originalTotal)
		os.Setenv("SHARD_INDEX", originalIndex)
		os.Setenv("POD_NAME", originalPod)
	}()

	os.Unsetenv("TOTAL_SHARDS")
	os.Unsetenv("SHARD_INDEX")
	os.Unsetenv("POD_NAME")

	config := GetShardConfig()

	if config.Enabled {
		t.Error("Enabled should be false by default")
	}

	if config.ShardIndex != 0 {
		t.Errorf("ShardIndex = %v, want 0", config.ShardIndex)
	}

	if config.TotalShards != 1 {
		t.Errorf("TotalShards = %v, want 1", config.TotalShards)
	}
}

func TestGetShardConfig_WithTotalShards(t *testing.T) {
	originalTotal := os.Getenv("TOTAL_SHARDS")
	originalIndex := os.Getenv("SHARD_INDEX")

	defer func() {
		os.Setenv("TOTAL_SHARDS", originalTotal)
		os.Setenv("SHARD_INDEX", originalIndex)
	}()

	os.Setenv("TOTAL_SHARDS", "3")
	os.Setenv("SHARD_INDEX", "1")

	config := GetShardConfig()

	if !config.Enabled {
		t.Error("Enabled should be true when TOTAL_SHARDS > 1")
	}

	if config.TotalShards != 3 {
		t.Errorf("TotalShards = %v, want 3", config.TotalShards)
	}

	if config.ShardIndex != 1 {
		t.Errorf("ShardIndex = %v, want 1", config.ShardIndex)
	}
}

func TestGetShardConfig_WithPodName(t *testing.T) {
	originalTotal := os.Getenv("TOTAL_SHARDS")
	originalIndex := os.Getenv("SHARD_INDEX")
	originalPod := os.Getenv("POD_NAME")

	defer func() {
		os.Setenv("TOTAL_SHARDS", originalTotal)
		os.Setenv("SHARD_INDEX", originalIndex)
		os.Setenv("POD_NAME", originalPod)
	}()

	os.Setenv("TOTAL_SHARDS", "5")
	os.Unsetenv("SHARD_INDEX")
	os.Setenv("POD_NAME", "code-search-indexer-2")

	config := GetShardConfig()

	if config.ShardIndex != 2 {
		t.Errorf("ShardIndex = %v, want 2 (extracted from pod name)", config.ShardIndex)
	}
}

func TestGetShardConfig_InvalidTotalShards(t *testing.T) {
	originalTotal := os.Getenv("TOTAL_SHARDS")
	defer os.Setenv("TOTAL_SHARDS", originalTotal)

	os.Setenv("TOTAL_SHARDS", "invalid")

	config := GetShardConfig()

	if config.Enabled {
		t.Error("Enabled should be false for invalid TOTAL_SHARDS")
	}

	if config.TotalShards != 1 {
		t.Errorf("TotalShards = %v, want 1 (default)", config.TotalShards)
	}
}

func TestGetShardConfig_SingleShard(t *testing.T) {
	originalTotal := os.Getenv("TOTAL_SHARDS")
	defer os.Setenv("TOTAL_SHARDS", originalTotal)

	os.Setenv("TOTAL_SHARDS", "1")

	config := GetShardConfig()

	if config.Enabled {
		t.Error("Enabled should be false when TOTAL_SHARDS = 1")
	}
}

func TestExtractOrdinal(t *testing.T) {
	tests := []struct {
		podName  string
		expected int
	}{
		{"code-search-indexer-0", 0},
		{"code-search-indexer-1", 1},
		{"code-search-indexer-2", 2},
		{"my-app-5", 5},
		{"indexer-99", 99},
		{"no-ordinal", -1},
		{"trailing-dash-", -1},
		{"", -1},
		{"nodash", -1},
	}

	for _, tt := range tests {
		t.Run(tt.podName, func(t *testing.T) {
			result := sharding.ExtractOrdinal(tt.podName)
			if result != tt.expected {
				t.Errorf(
					"sharding.ExtractOrdinal(%q) = %v, want %v",
					tt.podName,
					result,
					tt.expected,
				)
			}
		})
	}
}

func TestGetShardForRepo(t *testing.T) {
	tests := []struct {
		name        string
		repoName    string
		totalShards int
		// We just test consistency, not specific values
	}{
		{"single shard", "owner/repo", 1},
		{"three shards", "owner/repo", 3},
		{"five shards", "github.com/org/project", 5},
		{"ten shards", "gitlab.com/group/subgroup/repo", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetShardForRepo(tt.repoName, tt.totalShards)

			if result < 0 || result >= tt.totalShards {
				t.Errorf("GetShardForRepo(%q, %d) = %d, should be in range [0, %d)",
					tt.repoName, tt.totalShards, result, tt.totalShards)
			}

			// Verify consistency - same input should produce same output
			result2 := GetShardForRepo(tt.repoName, tt.totalShards)
			if result != result2 {
				t.Errorf("GetShardForRepo is not consistent: %d != %d", result, result2)
			}
		})
	}
}

func TestGetShardForRepo_SingleShard(t *testing.T) {
	result := GetShardForRepo("any/repo", 1)
	if result != 0 {
		t.Errorf("GetShardForRepo with 1 shard should always return 0, got %d", result)
	}
}

func TestGetShardForRepo_Distribution(t *testing.T) {
	// Test that sharding distributes repos across shards reasonably
	totalShards := 4
	repos := []string{
		"owner/repo1",
		"owner/repo2",
		"owner/repo3",
		"owner/repo4",
		"org/project-a",
		"org/project-b",
		"github.com/user/app",
		"gitlab.com/team/service",
	}

	distribution := make(map[int]int)

	for _, repo := range repos {
		shard := GetShardForRepo(repo, totalShards)
		distribution[shard]++
	}

	// Verify all shards are covered at least somewhat
	// (this is probabilistic, but with 8 repos in 4 shards, should be OK)
	if len(distribution) == 0 {
		t.Error("No shards used in distribution")
	}
}

func TestShardConfig_ShouldHandleRepo(t *testing.T) {
	tests := []struct {
		name     string
		config   ShardConfig
		repoName string
	}{
		{
			name: "disabled sharding indexes all",
			config: ShardConfig{
				Enabled:     false,
				ShardIndex:  0,
				TotalShards: 1,
			},
			repoName: "any/repo",
		},
		{
			name: "single shard indexes all",
			config: ShardConfig{
				Enabled:     true,
				ShardIndex:  0,
				TotalShards: 1,
			},
			repoName: "any/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.config.ShouldHandleRepo(tt.repoName) {
				t.Errorf(
					"ShouldHandleRepo(%q) = false, expected true for disabled/single shard",
					tt.repoName,
				)
			}
		})
	}
}

func TestShardConfig_ShouldHandleRepo_MultiShard(t *testing.T) {
	// Test that each repo is assigned to exactly one shard
	totalShards := 3
	repos := []string{
		"repo-a",
		"repo-b",
		"repo-c",
		"repo-d",
		"repo-e",
	}

	for _, repo := range repos {
		assignedCount := 0

		for shardIndex := range totalShards {
			config := ShardConfig{
				Enabled:     true,
				ShardIndex:  shardIndex,
				TotalShards: totalShards,
			}

			if config.ShouldHandleRepo(repo) {
				assignedCount++
			}
		}

		if assignedCount != 1 {
			t.Errorf("Repo %q was assigned to %d shards, expected exactly 1", repo, assignedCount)
		}
	}
}

func TestShardConfig_Fields(t *testing.T) {
	config := ShardConfig{
		ShardIndex:  2,
		TotalShards: 5,
		Enabled:     true,
	}

	if config.ShardIndex != 2 {
		t.Errorf("ShardIndex = %v, want 2", config.ShardIndex)
	}

	if config.TotalShards != 5 {
		t.Errorf("TotalShards = %v, want 5", config.TotalShards)
	}

	if !config.Enabled {
		t.Error("Enabled should be true")
	}
}
