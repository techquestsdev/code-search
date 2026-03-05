package audit

import (
	"context"
	"time"
)

// AuditLogger records user actions for compliance and observability.
// The default NoOpAuditLogger discards all events.
// Enterprise implementations can persist to a database, send to SIEM, etc.
type AuditLogger interface {
	LogSearch(ctx context.Context, event SearchEvent)
	LogAccess(ctx context.Context, event AccessEvent)
	LogReplace(ctx context.Context, event ReplaceEvent)
	LogOperation(ctx context.Context, event OperationEvent)
}

// SearchEvent records a search operation.
type SearchEvent struct {
	UserID    string
	Query     string
	Results   int
	Duration  time.Duration
	ClientIP  string
	Timestamp time.Time
}

// AccessEvent records a file or repository access.
type AccessEvent struct {
	UserID    string
	RepoName  string
	FilePath  string
	Action    string // "browse", "raw_download", "view_blob"
	ClientIP  string
	Timestamp time.Time
}

// ReplaceEvent records a search-and-replace operation.
type ReplaceEvent struct {
	UserID     string
	Query      string
	ReposCount int
	FilesCount int
	ClientIP   string
	Timestamp  time.Time
}

// OperationEvent records a generic management operation (create, update, delete, etc.).
type OperationEvent struct {
	UserID       string
	Action       string         // "create", "update", "delete", "sync", "exclude", "include", "restore", "assign"
	ResourceType string         // "repo", "connection", "role", "user", "token"
	ResourceID   string         // ID of the resource
	ResourceName string         // Human-readable name
	Details      map[string]any // Action-specific details
	ClientIP     string
	Timestamp    time.Time
}

// NoOpAuditLogger is the default logger that discards all events.
// Used in the open-source core where audit logging is not required.
type NoOpAuditLogger struct{}

func (n *NoOpAuditLogger) LogSearch(_ context.Context, _ SearchEvent)       {}
func (n *NoOpAuditLogger) LogAccess(_ context.Context, _ AccessEvent)       {}
func (n *NoOpAuditLogger) LogReplace(_ context.Context, _ ReplaceEvent)     {}
func (n *NoOpAuditLogger) LogOperation(_ context.Context, _ OperationEvent) {}
