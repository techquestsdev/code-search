package audit

import (
	"context"
	"testing"
	"time"
)

func TestNoOpAuditLogger(t *testing.T) {
	logger := &NoOpAuditLogger{}
	ctx := context.Background()

	// Verify all methods can be called without panic
	logger.LogSearch(ctx, SearchEvent{
		UserID:    "user-1",
		Query:     "test query",
		Results:   42,
		Duration:  100 * time.Millisecond,
		ClientIP:  "127.0.0.1",
		Timestamp: time.Now(),
	})

	logger.LogAccess(ctx, AccessEvent{
		UserID:    "user-1",
		RepoName:  "org/repo",
		FilePath:  "main.go",
		Action:    "browse",
		ClientIP:  "127.0.0.1",
		Timestamp: time.Now(),
	})

	logger.LogReplace(ctx, ReplaceEvent{
		UserID:     "user-1",
		Query:      "old -> new",
		ReposCount: 5,
		FilesCount: 20,
		ClientIP:   "127.0.0.1",
		Timestamp:  time.Now(),
	})
}

// Verify interface compliance at compile time.
var _ AuditLogger = (*NoOpAuditLogger)(nil)
