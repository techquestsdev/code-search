package search

import (
	"testing"
)

func TestQuery_Defaults(t *testing.T) {
	q := Query{
		Pattern: "test",
	}

	if q.IsRegex {
		t.Error("IsRegex should default to false")
	}

	if q.CaseSensitive {
		t.Error("CaseSensitive should default to false")
	}

	if q.MaxResults != 0 {
		t.Error("MaxResults should default to 0")
	}
}

func TestQuery_WithOptions(t *testing.T) {
	q := Query{
		Pattern:       "func.*main",
		IsRegex:       true,
		CaseSensitive: true,
		Repos:         []string{"org/repo1", "org/repo2"},
		Languages:     []string{"go", "python"},
		FilePatterns:  []string{"*.go", "*.py"},
		MaxResults:    100,
	}

	if !q.IsRegex {
		t.Error("IsRegex should be true")
	}

	if !q.CaseSensitive {
		t.Error("CaseSensitive should be true")
	}

	if len(q.Repos) != 2 {
		t.Errorf("Repos length = %v, want 2", len(q.Repos))
	}

	if len(q.Languages) != 2 {
		t.Errorf("Languages length = %v, want 2", len(q.Languages))
	}

	if len(q.FilePatterns) != 2 {
		t.Errorf("FilePatterns length = %v, want 2", len(q.FilePatterns))
	}

	if q.MaxResults != 100 {
		t.Errorf("MaxResults = %v, want 100", q.MaxResults)
	}
}

func TestGlobToRegex(t *testing.T) {
	tests := []struct {
		glob     string
		expected string
	}{
		{"*.go", "[^/]*\\.go$"},
		{"**/*.ts", ".*[^/]*\\.ts$"},
		{"src/*.js", "src/[^/]*\\.js$"},
		{"test", "test"},
		{"*.test.js", "[^/]*\\.test\\.js$"},
		{"foo?bar", "foo[^/]bar"},
	}

	for _, tt := range tests {
		t.Run(tt.glob, func(t *testing.T) {
			result := globToRegex(tt.glob)
			if result != tt.expected {
				t.Errorf("globToRegex(%q) = %q, want %q", tt.glob, result, tt.expected)
			}
		})
	}
}

func TestResult(t *testing.T) {
	result := Result{
		Repo:     "github.com/org/repo",
		Branches: []string{"main", "develop"},
		Files: []File{
			{
				Name:     "main.go",
				Language: "go",
				Matches: []Match{
					{
						LineNum:  10,
						Line:     "func main() {",
						Before:   "package main",
						After:    "    fmt.Println()",
						Start:    0,
						End:      4,
						Fragment: "func",
					},
				},
			},
		},
	}

	if result.Repo == "" {
		t.Error("Repo should not be empty")
	}

	if len(result.Branches) != 2 {
		t.Errorf("Branches length = %v, want 2", len(result.Branches))
	}

	if len(result.Files) != 1 {
		t.Errorf("Files length = %v, want 1", len(result.Files))
	}

	file := result.Files[0]
	if file.Language != "go" {
		t.Errorf("File.Language = %v, want go", file.Language)
	}

	if len(file.Matches) != 1 {
		t.Errorf("File.Matches length = %v, want 1", len(file.Matches))
	}

	match := file.Matches[0]
	if match.LineNum != 10 {
		t.Errorf("Match.LineNum = %v, want 10", match.LineNum)
	}

	if match.Start >= match.End {
		t.Error("Match.Start should be less than Match.End")
	}
}

func TestSearchResponse(t *testing.T) {
	response := &SearchResponse{
		Results: []Result{
			{Repo: "repo1", Files: []File{{Name: "a.go"}}},
			{Repo: "repo2", Files: []File{{Name: "b.go"}}},
		},
		Stats: Stats{
			FilesConsidered: 1000,
			FilesLoaded:     100,
			FilesSkipped:    50,
			ShardsScanned:   4,
		},
		TotalMatches: 25,
	}

	if len(response.Results) != 2 {
		t.Errorf("Results length = %v, want 2", len(response.Results))
	}

	if response.TotalMatches != 25 {
		t.Errorf("TotalMatches = %v, want 25", response.TotalMatches)
	}

	if response.Stats.FilesConsidered != 1000 {
		t.Errorf("Stats.FilesConsidered = %v, want 1000", response.Stats.FilesConsidered)
	}
}

func TestRepoInfo(t *testing.T) {
	info := RepoInfo{
		Name:     "github.com/org/repo",
		Branches: []string{"main", "feature-x"},
		Files:    5000,
		Shard:    1,
	}

	if info.Name == "" {
		t.Error("Name should not be empty")
	}

	if len(info.Branches) != 2 {
		t.Errorf("Branches length = %v, want 2", len(info.Branches))
	}

	if info.Files != 5000 {
		t.Errorf("Files = %v, want 5000", info.Files)
	}

	if info.Shard != 1 {
		t.Errorf("Shard = %v, want 1", info.Shard)
	}
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{
			name: "http url",
			url:  "http://localhost:6070",
		},
		{
			name: "https url",
			url:  "https://zoekt.example.com:6070",
		},
		{
			name: "url with trailing slash",
			url:  "http://localhost:6070/",
		},
		{
			name: "plain address",
			url:  "localhost:6070",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.url)
			if client == nil {
				t.Error("NewClient should not return nil")
			}

			if client != nil {
				client.Close()
			}
		})
	}
}

func TestClient_Close(t *testing.T) {
	client := NewClient("localhost:6070")

	err := client.Close()
	if err != nil {
		t.Logf("Close returned error (may be expected): %v", err)
	}
}

func TestMatch(t *testing.T) {
	match := Match{
		LineNum:  42,
		Line:     "    result := searchPattern(query)",
		Before:   "func process() {",
		After:    "    return result",
		Start:    14,
		End:      27,
		Fragment: "searchPattern",
	}

	if match.Start < 0 {
		t.Error("Start should not be negative")
	}

	if match.End < match.Start {
		t.Error("End should be >= Start")
	}

	if match.End > len(match.Line) {
		t.Error("End should not exceed line length")
	}

	matchedText := match.Line[match.Start:match.End]
	if matchedText != "searchPattern" {
		t.Errorf("Matched text = %q, want 'searchPattern'", matchedText)
	}
}

func TestFile(t *testing.T) {
	file := File{
		Name:     "pkg/search/client.go",
		Language: "go",
		Matches: []Match{
			{LineNum: 10, Line: "line 1", Start: 0, End: 4},
			{LineNum: 20, Line: "line 2", Start: 5, End: 10},
			{LineNum: 30, Line: "line 3", Start: 0, End: 6},
		},
	}

	if file.Name == "" {
		t.Error("Name should not be empty")
	}

	if file.Language == "" {
		t.Error("Language should not be empty")
	}

	if len(file.Matches) != 3 {
		t.Errorf("Matches length = %v, want 3", len(file.Matches))
	}
}

func TestStats(t *testing.T) {
	stats := Stats{
		FilesConsidered: 10000,
		FilesLoaded:     500,
		FilesSkipped:    100,
		ShardsScanned:   8,
	}

	if stats.FilesLoaded > stats.FilesConsidered {
		t.Error("FilesLoaded should not exceed FilesConsidered")
	}

	if stats.FilesSkipped > stats.FilesConsidered {
		t.Error("FilesSkipped should not exceed FilesConsidered")
	}
}

func TestBuildQuery_SimpleCases(t *testing.T) {
	client := &Client{}

	tests := []struct {
		name  string
		query Query
	}{
		{
			name:  "simple pattern",
			query: Query{Pattern: "main"},
		},
		{
			name:  "regex pattern",
			query: Query{Pattern: "func.*main", IsRegex: true},
		},
		{
			name:  "case sensitive",
			query: Query{Pattern: "Main", CaseSensitive: true},
		},
		{
			name:  "with repos filter",
			query: Query{Pattern: "test", Repos: []string{"repo1", "repo2"}},
		},
		{
			name:  "with language filter",
			query: Query{Pattern: "test", Languages: []string{"go"}},
		},
		{
			name:  "with file patterns",
			query: Query{Pattern: "test", FilePatterns: []string{"*.go"}},
		},
		{
			name: "complex query",
			query: Query{
				Pattern:       "error",
				IsRegex:       false,
				CaseSensitive: false,
				Repos:         []string{"myorg/backend"},
				Languages:     []string{"go", "python"},
				FilePatterns:  []string{"*.go", "*.py"},
				MaxResults:    50,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.buildQuery(tt.query)
			if result == nil {
				t.Error("buildQuery should not return nil")
			}
		})
	}
}

func TestBuildQuery_BranchFilter(t *testing.T) {
	client := &Client{}

	q1 := Query{Pattern: "test"}

	result1 := client.buildQuery(q1)
	if result1 == nil {
		t.Error("buildQuery should not return nil")
	}

	q2 := Query{Pattern: "test branch:main"}

	result2 := client.buildQuery(q2)
	if result2 == nil {
		t.Error("buildQuery should not return nil")
	}
}
