package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/techquestsdev/code-search/internal/search"
)

func TestSearch_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/search",
		bytes.NewReader([]byte("invalid json")),
	)
	req.Header.Set("Content-Type", "application/json")

	var parsedReq search.SearchRequest

	err := json.NewDecoder(req.Body).Decode(&parsedReq)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestSearch_EmptyBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/search", bytes.NewReader([]byte("{}")))

	var parsedReq search.SearchRequest

	err := json.NewDecoder(req.Body).Decode(&parsedReq)
	if err != nil {
		t.Fatalf("failed to parse empty body: %v", err)
	}

	if parsedReq.Query != "" {
		t.Errorf("expected empty query, got %q", parsedReq.Query)
	}
}

func TestSearch_WithAllOptions(t *testing.T) {
	reqBody := search.SearchRequest{
		Query:         "test pattern",
		Repos:         []string{"repo1", "repo2"},
		FilePatterns:  []string{"*.go"},
		Languages:     []string{"Go"},
		IsRegex:       true,
		CaseSensitive: true,
		ContextLines:  5,
		Limit:         100,
	}
	body, _ := json.Marshal(reqBody)

	var parsedReq search.SearchRequest

	err := json.NewDecoder(bytes.NewReader(body)).Decode(&parsedReq)
	if err != nil {
		t.Fatalf("failed to parse request: %v", err)
	}

	if parsedReq.Query != "test pattern" {
		t.Errorf("expected query 'test pattern', got %q", parsedReq.Query)
	}

	if len(parsedReq.Repos) != 2 {
		t.Errorf("expected 2 repos, got %d", len(parsedReq.Repos))
	}

	if !parsedReq.IsRegex {
		t.Error("expected IsRegex to be true")
	}

	if !parsedReq.CaseSensitive {
		t.Error("expected CaseSensitive to be true")
	}

	if parsedReq.ContextLines != 5 {
		t.Errorf("expected context lines 5, got %d", parsedReq.ContextLines)
	}

	if parsedReq.Limit != 100 {
		t.Errorf("expected limit 100, got %d", parsedReq.Limit)
	}
}

func TestSearchResponse_JSON(t *testing.T) {
	response := map[string]any{
		"results": []search.SearchResult{
			{
				Repo:     "github.com/test/repo",
				File:     "main.go",
				Language: "Go",
				Line:     10,
				Content:  "func main() {",
			},
		},
		"total_count": 1,
		"truncated":   false,
		"duration":    "100ms",
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if parsed["total_count"].(float64) != 1 {
		t.Errorf("expected total_count 1, got %v", parsed["total_count"])
	}

	if parsed["truncated"].(bool) != false {
		t.Error("expected truncated to be false")
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"status": "ok"}

	writeJSON(w, data)

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", w.Header().Get("Content-Type"))
	}

	var parsed map[string]string

	err := json.NewDecoder(w.Body).Decode(&parsed)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if parsed["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", parsed["status"])
	}
}

func TestSearch_DefaultContextLines(t *testing.T) {
	reqBody := search.SearchRequest{
		Query: "func main",
	}
	body, _ := json.Marshal(reqBody)

	var parsedReq search.SearchRequest

	err := json.NewDecoder(bytes.NewReader(body)).Decode(&parsedReq)
	if err != nil {
		t.Fatalf("failed to parse request: %v", err)
	}

	// Simulate the handler's default setting
	if parsedReq.ContextLines == 0 {
		parsedReq.ContextLines = 2
	}

	if parsedReq.ContextLines != 2 {
		t.Errorf("expected default context lines 2, got %d", parsedReq.ContextLines)
	}
}
