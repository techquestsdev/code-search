package replace

import "testing"

func TestExtractContentPattern(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "simple word",
			query:    "destroyer",
			expected: "destroyer",
		},
		{
			name:     "content prefix",
			query:    "content:destroyer",
			expected: "destroyer",
		},
		{
			name:     "repo and content",
			query:    "repo:^gitlab\\.example\\.com/foo$ content:destroyer",
			expected: "destroyer",
		},
		{
			name:     "repo, content, and file",
			query:    "repo:^gitlab\\.example\\.com/foo$ content:destroyer file:README*",
			expected: "destroyer",
		},
		{
			name:     "content with file pattern",
			query:    "content:findme file:*.go",
			expected: "findme",
		},
		{
			name:     "word after repo filter",
			query:    "repo:myrepo searchterm",
			expected: "searchterm",
		},
		{
			name:     "quoted phrase",
			query:    "\"exact phrase\"",
			expected: "exact phrase",
		},
		{
			name:     "content with quoted phrase",
			query:    "content:\"multi word search\"",
			expected: "multi word search",
		},
		{
			name:     "multiple repos with content",
			query:    "repo:foo repo:bar content:baz",
			expected: "baz",
		},
		{
			name:     "lang filter with content",
			query:    "lang:go content:func",
			expected: "func",
		},
		{
			name:     "case filter with content",
			query:    "case:yes content:MyFunc",
			expected: "MyFunc",
		},
		{
			name:     "parenthesized repo with OR and content word",
			query:    "(repo:^gitlab\\.com/myorg/group/project$ or repo:^github\\.com/myorg/project$) destroyer",
			expected: "destroyer",
		},
		{
			name:     "parenthesized repo with AND and content word",
			query:    "(repo:foo and repo:bar) searchterm",
			expected: "searchterm",
		},
		{
			name:     "OR without parentheses",
			query:    "repo:foo or repo:bar destroyer",
			expected: "destroyer",
		},
		{
			name:     "AND without parentheses",
			query:    "repo:foo and repo:bar searchme",
			expected: "searchme",
		},
		{
			name:     "NOT operator with repo",
			query:    "not repo:excluded findthis",
			expected: "findthis",
		},
		{
			name:     "complex boolean with multiple content words",
			query:    "(repo:a or repo:b) hello world",
			expected: "hello world",
		},
		{
			name:     "parenthesized repo or with content",
			query:    "(repo:foo or repo:baz) mypattern",
			expected: "mypattern",
		},
		{
			name:     "mixed case boolean operators",
			query:    "repo:foo OR repo:bar AND repo:baz content",
			expected: "content",
		},
		{
			name:     "empty string",
			query:    "",
			expected: "",
		},
		{
			name:     "only whitespace",
			query:    "   ",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractContentPattern(tt.query)
			if result != tt.expected {
				t.Errorf("extractContentPattern(%q) = %q, want %q", tt.query, result, tt.expected)
			}
		})
	}
}
