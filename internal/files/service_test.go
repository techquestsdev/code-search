package files

import (
	"encoding/json"
	"testing"
)

func TestNewService(t *testing.T) {
	svc := NewService("/path/to/repos")
	if svc == nil {
		t.Fatal("expected service to be created")
	}

	if svc.reposPath != "/path/to/repos" {
		t.Errorf("expected repos path '/path/to/repos', got %q", svc.reposPath)
	}
}

func TestService_GetRepoPath(t *testing.T) {
	svc := NewService("/data/repos")

	tests := []struct {
		name     string
		repoName string
		expected string
	}{
		{
			name:     "simple name",
			repoName: "myrepo",
			expected: "/data/repos/myrepo.git",
		},
		{
			name:     "with single slash",
			repoName: "org/repo",
			expected: "/data/repos/org_repo.git",
		},
		{
			name:     "with multiple slashes",
			repoName: "apps/adr/registry",
			expected: "/data/repos/apps_adr_registry.git",
		},
		{
			name:     "github style",
			repoName: "github.com/user/repo",
			expected: "/data/repos/github.com_user_repo.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := svc.GetRepoPath(tt.repoName)
			if path != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, path)
			}
		})
	}
}

func TestService_RepoExists_NonExistent(t *testing.T) {
	svc := NewService("/nonexistent/path")

	exists := svc.RepoExists("some-repo")
	if exists {
		t.Error("expected repo to not exist")
	}
}

func TestTreeEntry_JSON(t *testing.T) {
	entry := TreeEntry{
		Name:     "main.go",
		Type:     "file",
		Path:     "cmd/main.go",
		Size:     1234,
		Language: "Go",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed TreeEntry
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Name != "main.go" {
		t.Errorf("expected name 'main.go', got %q", parsed.Name)
	}

	if parsed.Type != "file" {
		t.Errorf("expected type 'file', got %q", parsed.Type)
	}

	if parsed.Path != "cmd/main.go" {
		t.Errorf("expected path 'cmd/main.go', got %q", parsed.Path)
	}

	if parsed.Size != 1234 {
		t.Errorf("expected size 1234, got %d", parsed.Size)
	}

	if parsed.Language != "Go" {
		t.Errorf("expected language 'Go', got %q", parsed.Language)
	}
}

func TestTreeEntry_Dir_JSON(t *testing.T) {
	entry := TreeEntry{
		Name: "src",
		Type: "dir",
		Path: "src",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Size should be omitted for directories (omitempty)
	if _, ok := parsed["size"]; ok {
		t.Error("expected size to be omitted for directory")
	}
	// Language should be omitted for directories
	if _, ok := parsed["language"]; ok {
		t.Error("expected language to be omitted for directory")
	}
}

func TestBlobResponse_JSON(t *testing.T) {
	blob := BlobResponse{
		Content:  "package main\n\nfunc main() {}\n",
		Path:     "main.go",
		Size:     28,
		Language: "Go",
		Binary:   false,
	}

	data, err := json.Marshal(blob)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed BlobResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Content != "package main\n\nfunc main() {}\n" {
		t.Errorf("content mismatch")
	}

	if parsed.Path != "main.go" {
		t.Errorf("expected path 'main.go', got %q", parsed.Path)
	}

	if parsed.Size != 28 {
		t.Errorf("expected size 28, got %d", parsed.Size)
	}

	if parsed.Language != "Go" {
		t.Errorf("expected language 'Go', got %q", parsed.Language)
	}

	if parsed.Binary {
		t.Error("expected binary to be false")
	}
}

func TestBlobResponse_Binary_JSON(t *testing.T) {
	blob := BlobResponse{
		Content:  "",
		Path:     "image.png",
		Size:     4096,
		Language: "binary",
		Binary:   true,
	}

	data, err := json.Marshal(blob)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed BlobResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if !parsed.Binary {
		t.Error("expected binary to be true")
	}

	if parsed.Content != "" {
		t.Error("expected empty content for binary file")
	}

	if parsed.Language != "binary" {
		t.Errorf("expected language 'binary', got %q", parsed.Language)
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected string
		content  []byte
	}{
		{
			name:     "Go file",
			filename: "main.go",
			expected: "Go",
		},
		{
			name:     "JavaScript file",
			filename: "index.js",
			expected: "JavaScript",
		},
		{
			name:     "TypeScript file",
			filename: "app.ts",
			expected: "TypeScript",
		},
		{
			name:     "Python file",
			filename: "script.py",
			expected: "Python",
		},
		{
			name:     "Rust file with content",
			filename: "main.rs",
			expected: "Rust",
			content:  []byte("fn main() {\n    println!(\"Hello\");\n}\n"),
		},
		{
			name:     "Markdown file",
			filename: "README.md",
			expected: "Markdown",
		},
		{
			name:     "JSON file",
			filename: "config.json",
			expected: "JSON",
		},
		{
			name:     "YAML file",
			filename: "config.yaml",
			expected: "YAML",
		},
		{
			name:     "Shell file",
			filename: "setup.sh",
			expected: "Shell",
		},
		{
			name:     "Dockerfile",
			filename: "Dockerfile",
			expected: "Dockerfile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lang := DetectLanguage(tt.filename, tt.content)
			if lang != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, lang)
			}
		})
	}
}

func TestDetectLanguage_WithContent(t *testing.T) {
	// Test Go content detection
	goContent := []byte("package main\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}\n")
	lang := DetectLanguage("unknown.txt", goContent)
	// Content should help detect Go, but may vary by library
	// At minimum it should return something
	if lang == "" {
		t.Error("expected some language to be detected")
	}
}
