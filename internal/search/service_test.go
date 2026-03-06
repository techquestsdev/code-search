package search

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewService_SingleURL(t *testing.T) {
	svc := NewService("http://localhost:6070")

	if svc.useSharding {
		t.Error("expected single URL to not use sharding")
	}

	if svc.client == nil {
		t.Error("expected client to be set")
	}
}

func TestNewShardedService_SingleShard(t *testing.T) {
	// Single URL should use regular client
	svc := NewShardedService("http://localhost:6070")

	if svc.useSharding {
		t.Error("expected single shard to use regular client")
	}

	if svc.client == nil {
		t.Error("expected client to be set")
	}
}

func TestNewShardedService_MultipleShards(t *testing.T) {
	// Multiple URLs should use sharded client
	svc := NewShardedService("http://localhost:6070,http://localhost:6071")

	if !svc.useSharding {
		t.Error("expected multiple shards to use sharded client")
	}

	if svc.shardedClient == nil {
		t.Error("expected shardedClient to be set")
	}
}

func TestSearchRequest_JSON(t *testing.T) {
	req := SearchRequest{
		Query:         "test pattern",
		IsRegex:       true,
		CaseSensitive: true,
		Repos:         []string{"repo1"},
		Languages:     []string{"Go"},
		FilePatterns:  []string{"*.go"},
		Limit:         100,
		ContextLines:  3,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed SearchRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Query != "test pattern" {
		t.Errorf("expected query 'test pattern', got %q", parsed.Query)
	}

	if !parsed.IsRegex {
		t.Error("expected is_regex to be true")
	}

	if !parsed.CaseSensitive {
		t.Error("expected case_sensitive to be true")
	}

	if len(parsed.Repos) != 1 || parsed.Repos[0] != "repo1" {
		t.Errorf("expected repos [repo1], got %v", parsed.Repos)
	}
}

func TestSearchResult_JSON(t *testing.T) {
	result := SearchResult{
		Repo:       "github.com/test/repo",
		File:       "main.go",
		Line:       10,
		Column:     5,
		Content:    "func main() {",
		Language:   "Go",
		MatchStart: 5,
		MatchEnd:   9,
		Context: ResultContext{
			Before: []string{"package main", ""},
			After:  []string{"    fmt.Println()", "}"},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed SearchResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Repo != "github.com/test/repo" {
		t.Errorf("expected repo 'github.com/test/repo', got %q", parsed.Repo)
	}

	if parsed.Line != 10 {
		t.Errorf("expected line 10, got %d", parsed.Line)
	}

	if len(parsed.Context.Before) != 2 {
		t.Errorf("expected 2 before lines, got %d", len(parsed.Context.Before))
	}

	if len(parsed.Context.After) != 2 {
		t.Errorf("expected 2 after lines, got %d", len(parsed.Context.After))
	}
}

func TestSearchResults_JSON(t *testing.T) {
	results := SearchResults{
		Results: []SearchResult{
			{
				Repo: "test/repo",
				File: "main.go",
				Line: 1,
			},
		},
		TotalCount: 1,
		Truncated:  false,
		Duration:   100 * time.Millisecond,
		Stats: SearchStats{
			FilesSearched: 100,
			ReposSearched: 5,
		},
	}

	data, err := json.Marshal(results)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed["total_count"].(float64) != 1 {
		t.Errorf("expected total_count 1, got %v", parsed["total_count"])
	}

	if parsed["truncated"].(bool) != false {
		t.Error("expected truncated to be false")
	}
}

func TestResultContext_JSON(t *testing.T) {
	ctx := ResultContext{
		Before: []string{"line 1", "line 2"},
		After:  []string{"line 4", "line 5"},
	}

	data, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed ResultContext
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(parsed.Before) != 2 {
		t.Errorf("expected 2 before lines, got %d", len(parsed.Before))
	}

	if len(parsed.After) != 2 {
		t.Errorf("expected 2 after lines, got %d", len(parsed.After))
	}
}

func TestSearchStats_JSON(t *testing.T) {
	stats := SearchStats{
		FilesSearched: 1000,
		ReposSearched: 50,
	}

	data, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed SearchStats
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.FilesSearched != 1000 {
		t.Errorf("expected files_searched 1000, got %d", parsed.FilesSearched)
	}

	if parsed.ReposSearched != 50 {
		t.Errorf("expected repos_searched 50, got %d", parsed.ReposSearched)
	}
}

func TestSearchRequest_Defaults(t *testing.T) {
	req := SearchRequest{}

	// Verify zero values
	if req.Query != "" {
		t.Errorf("expected empty query, got %q", req.Query)
	}

	if req.IsRegex {
		t.Error("expected is_regex to be false by default")
	}

	if req.CaseSensitive {
		t.Error("expected case_sensitive to be false by default")
	}

	if req.Limit != 0 {
		t.Errorf("expected limit 0 (unlimited), got %d", req.Limit)
	}
}

func TestSearchRequest_OmitEmpty(t *testing.T) {
	req := SearchRequest{
		Query: "test",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Check that empty fields are omitted
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// repos should be omitted when empty
	if _, ok := parsed["repos"]; ok {
		t.Error("expected repos to be omitted when empty")
	}
	// languages should be omitted when empty
	if _, ok := parsed["languages"]; ok {
		t.Error("expected languages to be omitted when empty")
	}
}

func TestService_Close(t *testing.T) {
	svc := NewService("http://localhost:6070")

	// Close should not panic on valid service
	err := svc.Close()
	if err != nil {
		t.Errorf("expected no error on close, got %v", err)
	}
}

func TestShardedService_Close(t *testing.T) {
	svc := NewShardedService("http://localhost:6070,http://localhost:6071")

	// Close should not panic on sharded service
	err := svc.Close()
	if err != nil {
		t.Errorf("expected no error on close, got %v", err)
	}
}
