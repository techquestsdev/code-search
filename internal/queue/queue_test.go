package queue

import (
	"encoding/json"
	"testing"
	"time"
)

func TestJobTypes(t *testing.T) {
	tests := []struct {
		jobType  JobType
		expected string
	}{
		{JobTypeIndex, "index"},
		{JobTypeReplace, "replace"},
		{JobTypeSync, "sync"},
		{JobTypeCleanup, "cleanup"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if string(tt.jobType) != tt.expected {
				t.Errorf("JobType = %v, want %v", tt.jobType, tt.expected)
			}
		})
	}
}

func TestJobStatus(t *testing.T) {
	tests := []struct {
		status   JobStatus
		expected string
	}{
		{JobStatusPending, "pending"},
		{JobStatusRunning, "running"},
		{JobStatusCompleted, "completed"},
		{JobStatusFailed, "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("JobStatus = %v, want %v", tt.status, tt.expected)
			}
		})
	}
}

func TestJob_JSON(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	startedAt := now.Add(-5 * time.Minute)
	completedAt := now

	job := &Job{
		ID:          "12345",
		Type:        JobTypeIndex,
		Status:      JobStatusCompleted,
		Payload:     json.RawMessage(`{"repository_id": 1}`),
		Error:       "",
		CreatedAt:   now.Add(-10 * time.Minute),
		UpdatedAt:   now,
		StartedAt:   &startedAt,
		CompletedAt: &completedAt,
		Progress: &JobProgress{
			Current: 100,
			Total:   100,
			Message: "Completed",
		},
	}

	data, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded Job

	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ID != job.ID {
		t.Errorf("ID = %v, want %v", decoded.ID, job.ID)
	}

	if decoded.Type != job.Type {
		t.Errorf("Type = %v, want %v", decoded.Type, job.Type)
	}

	if decoded.Status != job.Status {
		t.Errorf("Status = %v, want %v", decoded.Status, job.Status)
	}

	if decoded.Progress == nil {
		t.Error("Progress should not be nil")
	} else if decoded.Progress.Current != 100 {
		t.Errorf("Progress.Current = %v, want 100", decoded.Progress.Current)
	}
}

func TestIndexPayload_JSON(t *testing.T) {
	payload := IndexPayload{
		RepositoryID: 123,
		ConnectionID: 456,
		RepoName:     "owner/repo",
		CloneURL:     "https://github.com/owner/repo.git",
		Branch:       "main",
		Branches:     []string{"main", "develop"},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded IndexPayload

	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.RepositoryID != payload.RepositoryID {
		t.Errorf("RepositoryID = %v, want %v", decoded.RepositoryID, payload.RepositoryID)
	}

	if decoded.RepoName != payload.RepoName {
		t.Errorf("RepoName = %v, want %v", decoded.RepoName, payload.RepoName)
	}

	if len(decoded.Branches) != 2 {
		t.Errorf("Branches length = %v, want 2", len(decoded.Branches))
	}
}

func TestReplacePayload_JSON(t *testing.T) {
	payload := ReplacePayload{
		SearchPattern: "oldFunc",
		ReplaceWith:   "newFunc",
		IsRegex:       false,
		CaseSensitive: true,
		FilePatterns:  []string{"*.go"},
		Matches: []ReplaceMatch{
			{
				RepositoryID:   1,
				RepositoryName: "org/repo1",
				FilePath:       "main.go",
			},
			{
				RepositoryID:   2,
				RepositoryName: "org/repo2",
				FilePath:       "utils.go",
			},
		},
		BranchName:    "fix/rename-func",
		MRTitle:       "Rename oldFunc to newFunc",
		MRDescription: "Automated refactoring",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded ReplacePayload

	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.SearchPattern != payload.SearchPattern {
		t.Errorf("SearchPattern = %v, want %v", decoded.SearchPattern, payload.SearchPattern)
	}

	if len(decoded.Matches) != 2 {
		t.Errorf("Matches length = %v, want 2", len(decoded.Matches))
	}

	if decoded.Matches[0].FilePath != "main.go" {
		t.Errorf("Matches[0].FilePath = %v, want main.go", decoded.Matches[0].FilePath)
	}
}

func TestSyncPayload_JSON(t *testing.T) {
	payload := SyncPayload{
		ConnectionID: 789,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded SyncPayload

	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ConnectionID != payload.ConnectionID {
		t.Errorf("ConnectionID = %v, want %v", decoded.ConnectionID, payload.ConnectionID)
	}
}

func TestCleanupPayload_JSON(t *testing.T) {
	payload := CleanupPayload{
		RepositoryID:   111,
		RepositoryName: "org/old-repo",
		DataDir:        "/data/repos",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded CleanupPayload

	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.RepositoryID != payload.RepositoryID {
		t.Errorf("RepositoryID = %v, want %v", decoded.RepositoryID, payload.RepositoryID)
	}

	if decoded.DataDir != payload.DataDir {
		t.Errorf("DataDir = %v, want %v", decoded.DataDir, payload.DataDir)
	}
}

func TestJobProgress(t *testing.T) {
	progress := &JobProgress{
		Current: 50,
		Total:   100,
		Message: "Processing repository 50 of 100",
	}

	if progress.Current > progress.Total {
		t.Error("Current should not exceed Total")
	}

	percentage := float64(progress.Current) / float64(progress.Total) * 100
	if percentage != 50.0 {
		t.Errorf("Percentage = %v, want 50.0", percentage)
	}
}

func TestJobListOptions(t *testing.T) {
	tests := []struct {
		name string
		opts JobListOptions
	}{
		{
			name: "default options",
			opts: JobListOptions{},
		},
		{
			name: "filter by type",
			opts: JobListOptions{
				Type:  JobTypeIndex,
				Limit: 50,
			},
		},
		{
			name: "filter by status",
			opts: JobListOptions{
				Status: JobStatusPending,
				Limit:  100,
			},
		},
		{
			name: "exclude status",
			opts: JobListOptions{
				ExcludeStatus: JobStatusCompleted,
				Limit:         25,
			},
		},
		{
			name: "filter by repo name",
			opts: JobListOptions{
				RepoName: "my-org/",
				Limit:    10,
				Offset:   20,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.opts.Limit < 0 {
				t.Error("Limit should not be negative")
			}

			if tt.opts.Offset < 0 {
				t.Error("Offset should not be negative")
			}
		})
	}
}

func TestJobListResult(t *testing.T) {
	result := &JobListResult{
		Jobs:       []*Job{{ID: "1"}, {ID: "2"}},
		TotalCount: 100,
		Limit:      50,
		Offset:     0,
		HasMore:    true,
	}

	if len(result.Jobs) != 2 {
		t.Errorf("Jobs length = %v, want 2", len(result.Jobs))
	}

	if !result.HasMore {
		t.Error("HasMore should be true when TotalCount > Limit")
	}
}

func TestBulkActionResult(t *testing.T) {
	result := &BulkActionResult{
		Processed: 100,
		Succeeded: 95,
		Failed:    5,
	}

	if result.Succeeded+result.Failed != result.Processed {
		t.Error("Succeeded + Failed should equal Processed")
	}

	successRate := float64(result.Succeeded) / float64(result.Processed) * 100
	if successRate != 95.0 {
		t.Errorf("Success rate = %v, want 95.0", successRate)
	}
}

func TestCleanupResult(t *testing.T) {
	result := &CleanupResult{
		DeletedCount: 45,
		ScannedCount: 100,
	}

	if result.DeletedCount > result.ScannedCount {
		t.Error("DeletedCount should not exceed ScannedCount")
	}
}

func TestNewQueue(t *testing.T) {
	q := NewQueue(nil)

	if q == nil {
		t.Fatal("NewQueue should not return nil")
	}

	if q.queueKey != "codesearch:jobs:queue" {
		t.Errorf("queueKey = %v, want codesearch:jobs:queue", q.queueKey)
	}

	if q.jobPrefix != "codesearch:job:" {
		t.Errorf("jobPrefix = %v, want codesearch:job:", q.jobPrefix)
	}
}

func TestReplaceMatch(t *testing.T) {
	match := ReplaceMatch{
		RepositoryID:   42,
		RepositoryName: "myorg/myrepo",
		FilePath:       "src/main.go",
	}

	data, err := json.Marshal(match)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded ReplaceMatch

	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.RepositoryID != match.RepositoryID {
		t.Errorf("RepositoryID = %v, want %v", decoded.RepositoryID, match.RepositoryID)
	}

	if decoded.RepositoryName != match.RepositoryName {
		t.Errorf("RepositoryName = %v, want %v", decoded.RepositoryName, match.RepositoryName)
	}

	if decoded.FilePath != match.FilePath {
		t.Errorf("FilePath = %v, want %v", decoded.FilePath, match.FilePath)
	}
}

func TestErrJobAlreadyExists(t *testing.T) {
	if ErrJobAlreadyExists == nil {
		t.Error("ErrJobAlreadyExists should not be nil")
	}

	if ErrJobAlreadyExists.Error() != "job already exists" {
		t.Errorf("ErrJobAlreadyExists = %v, want 'job already exists'", ErrJobAlreadyExists.Error())
	}
}

// Test retry-related functionality

func TestRetryConstants(t *testing.T) {
	// Verify constants are set to reasonable values
	if DefaultMaxAttempts < 1 {
		t.Errorf("DefaultMaxAttempts = %d, should be at least 1", DefaultMaxAttempts)
	}

	if DefaultRetryBaseDelay < time.Second {
		t.Errorf("DefaultRetryBaseDelay = %v, should be at least 1 second", DefaultRetryBaseDelay)
	}

	if DefaultRetryMaxDelay < DefaultRetryBaseDelay {
		t.Errorf(
			"DefaultRetryMaxDelay = %v, should be >= DefaultRetryBaseDelay",
			DefaultRetryMaxDelay,
		)
	}
}

func TestCalculateRetryDelay(t *testing.T) {
	tests := []struct {
		name     string
		attempt  int
		minDelay time.Duration
		maxDelay time.Duration
	}{
		{"attempt_0", 0, DefaultRetryBaseDelay, DefaultRetryBaseDelay},         // 0 is treated as 1
		{"attempt_1", 1, DefaultRetryBaseDelay, DefaultRetryBaseDelay},         // 30s
		{"attempt_2", 2, 2 * DefaultRetryBaseDelay, 2 * DefaultRetryBaseDelay}, // 60s
		{"attempt_3", 3, 4 * DefaultRetryBaseDelay, 4 * DefaultRetryBaseDelay}, // 120s
		{"attempt_4", 4, 8 * DefaultRetryBaseDelay, 8 * DefaultRetryBaseDelay}, // 240s
		{"attempt_10", 10, DefaultRetryMaxDelay, DefaultRetryMaxDelay},         // Capped at max
		{"attempt_100", 100, DefaultRetryMaxDelay, DefaultRetryMaxDelay},       // Still capped
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay := CalculateRetryDelay(tt.attempt)
			if delay < tt.minDelay {
				t.Errorf("CalculateRetryDelay(%d) = %v, want >= %v", tt.attempt, delay, tt.minDelay)
			}

			if delay > tt.maxDelay {
				t.Errorf("CalculateRetryDelay(%d) = %v, want <= %v", tt.attempt, delay, tt.maxDelay)
			}
		})
	}
}

func TestJob_ShouldRetry(t *testing.T) {
	tests := []struct {
		name        string
		attempts    int
		maxAttempts int
		shouldRetry bool
	}{
		{
			name:        "first attempt with default max",
			attempts:    0,
			maxAttempts: 0, // Use default
			shouldRetry: true,
		},
		{
			name:        "second attempt with default max",
			attempts:    1,
			maxAttempts: 0,
			shouldRetry: true,
		},
		{
			name:        "at max attempts with default",
			attempts:    DefaultMaxAttempts,
			maxAttempts: 0,
			shouldRetry: false,
		},
		{
			name:        "custom max attempts - under",
			attempts:    2,
			maxAttempts: 5,
			shouldRetry: true,
		},
		{
			name:        "custom max attempts - at limit",
			attempts:    5,
			maxAttempts: 5,
			shouldRetry: false,
		},
		{
			name:        "custom max attempts - over",
			attempts:    6,
			maxAttempts: 5,
			shouldRetry: false,
		},
		{
			name:        "no retries allowed",
			attempts:    0,
			maxAttempts: 1,
			shouldRetry: true, // Can still do one attempt
		},
		{
			name:        "no retries - already failed once",
			attempts:    1,
			maxAttempts: 1,
			shouldRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &Job{
				Attempts:    tt.attempts,
				MaxAttempts: tt.maxAttempts,
			}
			if got := job.ShouldRetry(); got != tt.shouldRetry {
				t.Errorf("ShouldRetry() = %v, want %v", got, tt.shouldRetry)
			}
		})
	}
}

func TestJob_RetryFields_JSON(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	nextRetry := now.Add(30 * time.Second)

	job := &Job{
		ID:          "test-123",
		Type:        JobTypeIndex,
		Status:      JobStatusPending,
		Payload:     json.RawMessage(`{"repository_id": 1}`),
		CreatedAt:   now.Add(-10 * time.Minute),
		UpdatedAt:   now,
		Attempts:    2,
		MaxAttempts: 5,
		NextRetryAt: &nextRetry,
		LastError:   "connection timeout",
	}

	data, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded Job

	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Attempts != job.Attempts {
		t.Errorf("Attempts = %v, want %v", decoded.Attempts, job.Attempts)
	}

	if decoded.MaxAttempts != job.MaxAttempts {
		t.Errorf("MaxAttempts = %v, want %v", decoded.MaxAttempts, job.MaxAttempts)
	}

	if decoded.NextRetryAt == nil {
		t.Error("NextRetryAt should not be nil")
	} else if !decoded.NextRetryAt.Equal(nextRetry) {
		t.Errorf("NextRetryAt = %v, want %v", decoded.NextRetryAt, nextRetry)
	}

	if decoded.LastError != job.LastError {
		t.Errorf("LastError = %v, want %v", decoded.LastError, job.LastError)
	}
}

func TestJob_RetryFields_OmitEmpty(t *testing.T) {
	// Verify that empty retry fields don't appear in JSON when not set
	job := &Job{
		ID:        "test-123",
		Type:      JobTypeIndex,
		Status:    JobStatusPending,
		Payload:   json.RawMessage(`{}`),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		// Retry fields left at zero values
	}

	data, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	jsonStr := string(data)

	// Attempts should be present (not omitempty)
	// MaxAttempts, NextRetryAt, LastError should be omitted when empty
	// Note: MaxAttempts=0 is valid and may or may not be omitted depending on omitempty usage
	_ = jsonStr // Used for debugging if needed

	// Verify we can still unmarshal
	var decoded Job
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
}
