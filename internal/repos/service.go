package repos

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/aanogueira/code-search/internal/crypto"
	"github.com/aanogueira/code-search/internal/db"
	"github.com/aanogueira/code-search/internal/log"
)

// Repository represents a code repository.
type Repository struct {
	ID            int64
	ConnectionID  int64
	Name          string
	CloneURL      string
	DefaultBranch string
	Branches      db.StringArray
	LastIndexed   *time.Time
	IndexStatus   string
	Excluded      bool // When true, repo is excluded from sync and indexing
	Deleted       bool // When true, repo is permanently excluded and won't be re-added on sync
	Archived      bool // Reflects archived status from code host
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Connection represents a code host connection.
type Connection struct {
	ID              int64
	Name            string
	Type            string // github, gitlab, gitea, bitbucket
	URL             string
	Token           string
	ExcludeArchived bool     // When true, archived repos are excluded from sync
	CleanupArchived bool     // When true, auto-cleanup index for archived repos
	Repos           []string // Specific repos to sync (if empty, syncs all accessible repos)
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Service handles repository operations.
type Service struct {
	pool      db.Pool
	sql       *db.SQLBuilder
	encryptor *crypto.TokenEncryptor
}

// NewService creates a new repository service.
func NewService(pool db.Pool, encryptor *crypto.TokenEncryptor) *Service {
	return &Service{
		pool:      pool,
		sql:       db.NewSQLBuilder(pool.Driver()),
		encryptor: encryptor,
	}
}

// encryptToken encrypts a token for storage. Returns the token unchanged if no encryptor is configured.
func (s *Service) encryptToken(token string) string {
	if s.encryptor == nil || token == "" {
		return token
	}

	encrypted, err := s.encryptor.Encrypt(token)
	if err != nil {
		// Log error but don't fail - store unencrypted for backwards compat
		log.Warn("Failed to encrypt token, storing unencrypted", zap.Error(err))
		return token
	}

	return encrypted
}

// decryptToken decrypts a token from storage. Returns the token unchanged if not encrypted or no encryptor.
func (s *Service) decryptToken(token string) string {
	if s.encryptor == nil || token == "" {
		return token
	}

	return s.encryptor.MustDecrypt(token)
}

// SyncCodeHostsFromConfig syncs code hosts from config to database
// Config-defined code hosts take precedence and will overwrite existing connections with the same name
// Returns a list of connection IDs that were synced (for triggering repo discovery).
func (s *Service) SyncCodeHostsFromConfig(
	ctx context.Context,
	codeHosts map[string]CodeHostConfig,
) ([]int64, error) {
	if len(codeHosts) == 0 {
		return nil, nil
	}

	var syncedIDs []int64

	for name, ch := range codeHosts {
		// Resolve token from environment variable if needed
		token := ch.ResolveToken()
		if token == "" && len(ch.Repos) == 0 {
			// Skip code hosts with empty tokens unless specific repos are listed
			// (public repos can be accessed without auth)
			continue
		}

		// Encrypt token before storing
		encryptedToken := s.encryptToken(token)

		// Upsert: insert or update if exists
		var id int64

		// Convert repos to db.StringArray
		repos := db.StringArray(ch.Repos)

		if s.pool.Driver() == db.DriverMySQL {
			// MySQL: try get existing, then insert or update
			err := s.pool.QueryRow(ctx, `SELECT id FROM connections WHERE name = ?`, name).Scan(&id)
			if db.IsNoRowsError(err) {
				result, err := s.pool.Exec(ctx, `
					INSERT INTO connections (name, type, url, token, exclude_archived, cleanup_archived, repos, created_at, updated_at)
					VALUES (?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
				`, name, ch.Type, ch.URL, encryptedToken, ch.ExcludeArchived, ch.CleanupArchived, repos)
				if err != nil {
					return nil, fmt.Errorf("insert connection %s: %w", name, err)
				}

				id, _ = result.LastInsertId()
			} else if err != nil {
				return nil, fmt.Errorf("query connection %s: %w", name, err)
			} else {
				_, err = s.pool.Exec(ctx, `
					UPDATE connections SET type = ?, url = ?, token = ?, exclude_archived = ?, cleanup_archived = ?, repos = ?, updated_at = NOW()
					WHERE id = ?
				`, ch.Type, ch.URL, encryptedToken, ch.ExcludeArchived, ch.CleanupArchived, repos, id)
				if err != nil {
					return nil, fmt.Errorf("update connection %s: %w", name, err)
				}
			}
		} else {
			// PostgreSQL: use ON CONFLICT with RETURNING
			err := s.pool.QueryRow(ctx, `
				INSERT INTO connections (name, type, url, token, exclude_archived, cleanup_archived, repos, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
				ON CONFLICT (name)
				DO UPDATE SET type = $2, url = $3, token = $4, exclude_archived = $5, cleanup_archived = $6, repos = $7, updated_at = NOW()
				RETURNING id
			`, name, ch.Type, ch.URL, encryptedToken, ch.ExcludeArchived, ch.CleanupArchived, repos).Scan(&id)
			if err != nil {
				return nil, fmt.Errorf("upsert connection %s: %w", name, err)
			}
		}

		syncedIDs = append(syncedIDs, id)
	}

	return syncedIDs, nil
}

// CodeHostConfig represents a code host configuration from config file.
type CodeHostConfig struct {
	Type            string
	URL             string
	Token           string            // Can be literal or env var reference like "$CS_GITHUB_TOKEN"
	ExcludeArchived bool              // When true, archived repos are excluded from sync
	CleanupArchived bool              // When true, auto-cleanup index for archived repos
	Repos           []string          // Specific repos to index (if empty, syncs all accessible repos)
	RepoConfigs     []RepoConfigEntry // Per-repo configuration (branches, exclude, etc.)
}

// RepoConfigEntry represents per-repo configuration from config file.
type RepoConfigEntry struct {
	Name     string   // Repository name (e.g., "owner/repo")
	Branches []string // Specific branches to index (empty = default branch only)
	Exclude  bool     // If true, exclude this repo from indexing (can be re-included)
	Delete   bool     // If true, permanently delete this repo (won't be re-added on sync)
}

// ResolveToken resolves the token, expanding environment variable references.
func (ch *CodeHostConfig) ResolveToken() string {
	if len(ch.Token) > 0 && ch.Token[0] == '$' {
		envVar := ch.Token[1:]
		return os.Getenv(envVar)
	}

	return ch.Token
}

// GetRepoConfig returns the config for a specific repo name, or nil if not found.
func (ch *CodeHostConfig) GetRepoConfig(repoName string) *RepoConfigEntry {
	for i := range ch.RepoConfigs {
		if ch.RepoConfigs[i].Name == repoName {
			return &ch.RepoConfigs[i]
		}
	}

	return nil
}

// ListConnections returns all configured connections.
func (s *Service) ListConnections(ctx context.Context) ([]Connection, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, type, url, token, exclude_archived, cleanup_archived, repos, created_at, updated_at
		FROM connections
		ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("query connections: %w", err)
	}
	defer rows.Close()

	var connections []Connection

	for rows.Next() {
		var (
			c     Connection
			repos db.StringArray
		)

		err := rows.Scan(
			&c.ID,
			&c.Name,
			&c.Type,
			&c.URL,
			&c.Token,
			&c.ExcludeArchived,
			&c.CleanupArchived,
			&repos,
			&c.CreatedAt,
			&c.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan connection: %w", err)
		}

		c.Repos = repos

		// Decrypt token for use
		c.Token = s.decryptToken(c.Token)

		connections = append(connections, c)
	}

	return connections, rows.Err()
}

// GetConnection retrieves a connection by ID.
func (s *Service) GetConnection(ctx context.Context, id int64) (*Connection, error) {
	var (
		c     Connection
		repos db.StringArray
	)

	err := s.pool.QueryRow(ctx, `
		SELECT id, name, type, url, token, exclude_archived, cleanup_archived, repos, created_at, updated_at
		FROM connections
		WHERE id = $1
	`, id).Scan(&c.ID, &c.Name, &c.Type, &c.URL, &c.Token, &c.ExcludeArchived, &c.CleanupArchived, &repos, &c.CreatedAt, &c.UpdatedAt)
	c.Repos = repos

	if db.IsNoRowsError(err) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("query connection: %w", err)
	}

	// Decrypt token for use
	c.Token = s.decryptToken(c.Token)

	return &c, nil
}

// CreateConnection creates a new code host connection.
func (s *Service) CreateConnection(
	ctx context.Context,
	name, connType, url, token string,
	excludeArchived bool,
	cleanupArchived bool,
) (*Connection, error) {
	var c Connection

	// Encrypt token before storing
	encryptedToken := s.encryptToken(token)

	if s.pool.Driver() == db.DriverMySQL {
		result, err := s.pool.Exec(ctx, `
			INSERT INTO connections (name, type, url, token, exclude_archived, cleanup_archived, repos, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, '{}', NOW(), NOW())
		`, name, connType, url, encryptedToken, excludeArchived, cleanupArchived)
		if err != nil {
			return nil, fmt.Errorf("insert connection: %w", err)
		}

		id, _ := result.LastInsertId()

		return s.GetConnection(ctx, id)
	}

	var repos db.StringArray

	err := s.pool.QueryRow(ctx, `
		INSERT INTO connections (name, type, url, token, exclude_archived, cleanup_archived, repos)
		VALUES ($1, $2, $3, $4, $5, $6, '{}')
		RETURNING id, name, type, url, token, exclude_archived, cleanup_archived, repos, created_at, updated_at
	`, name, connType, url, encryptedToken, excludeArchived, cleanupArchived).Scan(&c.ID, &c.Name, &c.Type, &c.URL, &c.Token, &c.ExcludeArchived, &c.CleanupArchived, &repos, &c.CreatedAt, &c.UpdatedAt)
	c.Repos = repos

	if err != nil {
		return nil, fmt.Errorf("insert connection: %w", err)
	}

	// Decrypt token for response
	c.Token = s.decryptToken(c.Token)

	return &c, nil
}

// UpdateConnection updates an existing code host connection.
func (s *Service) UpdateConnection(
	ctx context.Context,
	id int64,
	name, connType, url, token string,
	excludeArchived bool,
	cleanupArchived bool,
) (*Connection, error) {
	var c Connection

	// Encrypt token before storing (if provided)
	encryptedToken := s.encryptToken(token)

	if s.pool.Driver() == db.DriverMySQL {
		var err error
		if token != "" {
			_, err = s.pool.Exec(ctx, `
				UPDATE connections
				SET name = ?, type = ?, url = ?, token = ?, exclude_archived = ?, cleanup_archived = ?, updated_at = NOW()
				WHERE id = ?
			`, name, connType, url, encryptedToken, excludeArchived, cleanupArchived, id)
		} else {
			_, err = s.pool.Exec(ctx, `
				UPDATE connections
				SET name = ?, type = ?, url = ?, exclude_archived = ?, cleanup_archived = ?, updated_at = NOW()
				WHERE id = ?
			`, name, connType, url, excludeArchived, cleanupArchived, id)
		}

		if err != nil {
			return nil, fmt.Errorf("update connection: %w", err)
		}

		return s.GetConnection(ctx, id)
	}

	// PostgreSQL: use RETURNING
	var repos db.StringArray
	if token != "" {
		err := s.pool.QueryRow(ctx, `
			UPDATE connections
			SET name = $2, type = $3, url = $4, token = $5, exclude_archived = $6, cleanup_archived = $7, updated_at = NOW()
			WHERE id = $1
			RETURNING id, name, type, url, token, exclude_archived, cleanup_archived, repos, created_at, updated_at
		`, id, name, connType, url, encryptedToken, excludeArchived, cleanupArchived).Scan(&c.ID, &c.Name, &c.Type, &c.URL, &c.Token, &c.ExcludeArchived, &c.CleanupArchived, &repos, &c.CreatedAt, &c.UpdatedAt)
		c.Repos = repos

		if db.IsNoRowsError(err) {
			return nil, nil
		}

		if err != nil {
			return nil, fmt.Errorf("update connection: %w", err)
		}
	} else {
		err := s.pool.QueryRow(ctx, `
			UPDATE connections
			SET name = $2, type = $3, url = $4, exclude_archived = $5, cleanup_archived = $6, updated_at = NOW()
			WHERE id = $1
			RETURNING id, name, type, url, token, exclude_archived, cleanup_archived, repos, created_at, updated_at
		`, id, name, connType, url, excludeArchived, cleanupArchived).Scan(&c.ID, &c.Name, &c.Type, &c.URL, &c.Token, &c.ExcludeArchived, &c.CleanupArchived, &repos, &c.CreatedAt, &c.UpdatedAt)
		c.Repos = repos

		if db.IsNoRowsError(err) {
			return nil, nil
		}

		if err != nil {
			return nil, fmt.Errorf("update connection: %w", err)
		}
	}

	// Decrypt token for response
	c.Token = s.decryptToken(c.Token)

	return &c, nil
}

// DeleteConnection removes a connection and all its repositories.
func (s *Service) DeleteConnection(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM connections WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete connection: %w", err)
	}

	return nil
}

// ListRepositories returns all repositories, optionally filtered by connection.
func (s *Service) ListRepositories(ctx context.Context, connectionID *int64) ([]Repository, error) {
	var (
		rows db.Rows
		err  error
	)

	if connectionID != nil {
		rows, err = s.pool.Query(ctx, `
			SELECT id, connection_id, name, clone_url, default_branch, branches,
				   last_indexed, index_status, excluded, deleted, archived, created_at, updated_at
			FROM repositories
			WHERE connection_id = $1
			ORDER BY name
		`, *connectionID)
	} else {
		rows, err = s.pool.Query(ctx, `
			SELECT id, connection_id, name, clone_url, default_branch, branches,
				   last_indexed, index_status, excluded, deleted, archived, created_at, updated_at
			FROM repositories
			ORDER BY name
		`)
	}

	if err != nil {
		return nil, fmt.Errorf("query repositories: %w", err)
	}

	defer rows.Close()

	var repos []Repository

	for rows.Next() {
		var r Repository

		err := rows.Scan(
			&r.ID,
			&r.ConnectionID,
			&r.Name,
			&r.CloneURL,
			&r.DefaultBranch,
			&r.Branches,
			&r.LastIndexed,
			&r.IndexStatus,
			&r.Excluded,
			&r.Deleted,
			&r.Archived,
			&r.CreatedAt,
			&r.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan repository: %w", err)
		}

		repos = append(repos, r)
	}

	return repos, rows.Err()
}

// RepoListOptions contains options for listing repositories with pagination.
type RepoListOptions struct {
	ConnectionID *int64
	Search       string // Search by name (case-insensitive)
	Status       string // Filter by index status
	Limit        int    // Max results (default 50)
	Offset       int    // Pagination offset
}

// RepoListResult represents the result of listing repositories.
type RepoListResult struct {
	Repos      []Repository `json:"repos"`
	TotalCount int          `json:"total_count"`
	Limit      int          `json:"limit"`
	Offset     int          `json:"offset"`
	HasMore    bool         `json:"has_more"`
}

// ListRepositoriesWithOptions returns repositories with filtering and pagination.
func (s *Service) ListRepositoriesWithOptions(
	ctx context.Context,
	opts RepoListOptions,
) (*RepoListResult, error) {
	if opts.Limit <= 0 {
		opts.Limit = 50
	}

	if opts.Limit > 10000 {
		opts.Limit = 10000
	}

	isMySQL := s.pool.Driver() == db.DriverMySQL

	// Build the WHERE clause dynamically
	whereClause := "WHERE 1=1"
	args := []any{}
	argNum := 1

	addPlaceholder := func() string {
		if isMySQL {
			return "?"
		}

		p := fmt.Sprintf("$%d", argNum)
		argNum++

		return p
	}

	if opts.ConnectionID != nil {
		whereClause += " AND connection_id = " + addPlaceholder()

		args = append(args, *opts.ConnectionID)
	}

	if opts.Search != "" {
		if isMySQL {
			whereClause += " AND (LOWER(name) LIKE LOWER(?) OR LOWER(clone_url) LIKE LOWER(?))"

			args = append(args, "%"+opts.Search+"%", "%"+opts.Search+"%")
		} else {
			p := addPlaceholder()
			whereClause += fmt.Sprintf(" AND (name ILIKE %s OR clone_url ILIKE %s)", p, p)

			args = append(args, "%"+opts.Search+"%")
		}
	}

	if opts.Status != "" {
		if opts.Status == "deleted" {
			whereClause += " AND deleted = true"
		} else if opts.Status == "excluded" {
			whereClause += " AND excluded = true AND deleted = false"
		} else {
			whereClause += " AND index_status = " + addPlaceholder() + " AND excluded = false AND deleted = false"

			args = append(args, opts.Status)
		}
	}

	// Get total count
	countQuery := "SELECT COUNT(*) FROM repositories " + whereClause

	var totalCount int
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return nil, fmt.Errorf("count repositories: %w", err)
	}

	// Get paginated results
	var query string
	if isMySQL {
		query = fmt.Sprintf(`
			SELECT id, connection_id, name, clone_url, default_branch, branches,
				   last_indexed, index_status, excluded, deleted, archived, created_at, updated_at
			FROM repositories
			%s
			ORDER BY name
			LIMIT ? OFFSET ?
		`, whereClause)
	} else {
		query = fmt.Sprintf(`
			SELECT id, connection_id, name, clone_url, default_branch, branches,
				   last_indexed, index_status, excluded, deleted, archived, created_at, updated_at
			FROM repositories
			%s
			ORDER BY name
			LIMIT $%d OFFSET $%d
		`, whereClause, argNum, argNum+1)
	}

	args = append(args, opts.Limit, opts.Offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query repositories: %w", err)
	}
	defer rows.Close()

	var repos []Repository

	for rows.Next() {
		var r Repository

		err := rows.Scan(
			&r.ID,
			&r.ConnectionID,
			&r.Name,
			&r.CloneURL,
			&r.DefaultBranch,
			&r.Branches,
			&r.LastIndexed,
			&r.IndexStatus,
			&r.Excluded,
			&r.Deleted,
			&r.Archived,
			&r.CreatedAt,
			&r.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan repository: %w", err)
		}

		repos = append(repos, r)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &RepoListResult{
		Repos:      repos,
		TotalCount: totalCount,
		Limit:      opts.Limit,
		Offset:     opts.Offset,
		HasMore:    opts.Offset+len(repos) < totalCount,
	}, nil
}

// GetRepository retrieves a repository by ID.
func (s *Service) GetRepository(ctx context.Context, id int64) (*Repository, error) {
	var r Repository

	err := s.pool.QueryRow(ctx, `
		SELECT id, connection_id, name, clone_url, default_branch, branches,
			   last_indexed, index_status, excluded, deleted, archived, created_at, updated_at
		FROM repositories
		WHERE id = $1
	`, id).Scan(
		&r.ID, &r.ConnectionID, &r.Name, &r.CloneURL, &r.DefaultBranch,
		&r.Branches, &r.LastIndexed, &r.IndexStatus, &r.Excluded, &r.Deleted, &r.Archived, &r.CreatedAt, &r.UpdatedAt,
	)
	if db.IsNoRowsError(err) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("query repository: %w", err)
	}

	return &r, nil
}

// GetRepositoryByName retrieves a repository by name.
func (s *Service) GetRepositoryByName(ctx context.Context, name string) (*Repository, error) {
	var r Repository

	err := s.pool.QueryRow(ctx, `
		SELECT id, connection_id, name, clone_url, default_branch, branches,
			   last_indexed, index_status, excluded, deleted, archived, created_at, updated_at
		FROM repositories
		WHERE name = $1
	`, name).Scan(
		&r.ID, &r.ConnectionID, &r.Name, &r.CloneURL, &r.DefaultBranch,
		&r.Branches, &r.LastIndexed, &r.IndexStatus, &r.Excluded, &r.Deleted, &r.Archived, &r.CreatedAt, &r.UpdatedAt,
	)
	if db.IsNoRowsError(err) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("query repository: %w", err)
	}

	return &r, nil
}

// FindRepositoryByZoektName finds a repository by the name returned from Zoekt search.
// Zoekt may return names like "gitlab.example.com/group/repo" while the database
// stores just "group/repo". This method tries multiple matching strategies:
// 1. Exact match on name
// 2. Match where clone_url contains the zoekt name
// 3. Match where zoekt name ends with the database name.
func (s *Service) FindRepositoryByZoektName(
	ctx context.Context,
	zoektName string,
) (*Repository, error) {
	var r Repository

	// Strategy 1: Exact match
	err := s.pool.QueryRow(ctx, `
		SELECT id, connection_id, name, clone_url, default_branch, branches,
			   last_indexed, index_status, excluded, deleted, archived, created_at, updated_at
		FROM repositories
		WHERE name = $1
	`, zoektName).Scan(
		&r.ID, &r.ConnectionID, &r.Name, &r.CloneURL, &r.DefaultBranch,
		&r.Branches, &r.LastIndexed, &r.IndexStatus, &r.Excluded, &r.Deleted, &r.Archived, &r.CreatedAt, &r.UpdatedAt,
	)
	if err == nil {
		return &r, nil
	}

	if !db.IsNoRowsError(err) {
		return nil, fmt.Errorf("query repository: %w", err)
	}

	// Strategy 2: Clone URL contains the zoekt name
	var likePattern string
	if s.pool.Driver() == db.DriverMySQL {
		likePattern = "%" + zoektName + "%"
		err = s.pool.QueryRow(ctx, `
			SELECT id, connection_id, name, clone_url, default_branch, branches,
				   last_indexed, index_status, excluded, deleted, archived, created_at, updated_at
			FROM repositories
			WHERE clone_url LIKE ?
			LIMIT 1
		`, likePattern).Scan(
			&r.ID, &r.ConnectionID, &r.Name, &r.CloneURL, &r.DefaultBranch,
			&r.Branches, &r.LastIndexed, &r.IndexStatus, &r.Excluded, &r.Deleted, &r.Archived, &r.CreatedAt, &r.UpdatedAt,
		)
	} else {
		err = s.pool.QueryRow(ctx, `
			SELECT id, connection_id, name, clone_url, default_branch, branches,
				   last_indexed, index_status, excluded, deleted, archived, created_at, updated_at
			FROM repositories
			WHERE clone_url LIKE '%' || $1 || '%'
			LIMIT 1
		`, zoektName).Scan(
			&r.ID, &r.ConnectionID, &r.Name, &r.CloneURL, &r.DefaultBranch,
			&r.Branches, &r.LastIndexed, &r.IndexStatus, &r.Excluded, &r.Deleted, &r.Archived, &r.CreatedAt, &r.UpdatedAt,
		)
	}

	if err == nil {
		return &r, nil
	}

	if !db.IsNoRowsError(err) {
		return nil, fmt.Errorf("query repository by clone_url: %w", err)
	}

	// Strategy 3: Zoekt name ends with database name
	if s.pool.Driver() == db.DriverMySQL {
		err = s.pool.QueryRow(ctx, `
			SELECT id, connection_id, name, clone_url, default_branch, branches,
				   last_indexed, index_status, excluded, deleted, archived, created_at, updated_at
			FROM repositories
			WHERE ? LIKE CONCAT('%/', name) OR ? LIKE CONCAT('%', name)
			LIMIT 1
		`, zoektName, zoektName).Scan(
			&r.ID, &r.ConnectionID, &r.Name, &r.CloneURL, &r.DefaultBranch,
			&r.Branches, &r.LastIndexed, &r.IndexStatus, &r.Excluded, &r.Deleted, &r.Archived, &r.CreatedAt, &r.UpdatedAt,
		)
	} else {
		err = s.pool.QueryRow(ctx, `
			SELECT id, connection_id, name, clone_url, default_branch, branches,
				   last_indexed, index_status, excluded, deleted, archived, created_at, updated_at
			FROM repositories
			WHERE $1 LIKE '%/' || name OR $1 LIKE '%' || name
			LIMIT 1
		`, zoektName).Scan(
			&r.ID, &r.ConnectionID, &r.Name, &r.CloneURL, &r.DefaultBranch,
			&r.Branches, &r.LastIndexed, &r.IndexStatus, &r.Excluded, &r.Deleted, &r.Archived, &r.CreatedAt, &r.UpdatedAt,
		)
	}

	if err == nil {
		return &r, nil
	}

	if db.IsNoRowsError(err) {
		return nil, nil
	}

	return nil, fmt.Errorf("query repository by suffix: %w", err)
}

// CreateRepository adds a new repository.
func (s *Service) CreateRepository(
	ctx context.Context,
	connectionID int64,
	name, cloneURL, defaultBranch string,
	branches []string,
) (*Repository, error) {
	var r Repository

	branchesArg := prepareBranchesArg(s.pool.Driver(), branches)

	if s.pool.Driver() == db.DriverMySQL {
		result, err := s.pool.Exec(ctx, `
			INSERT INTO repositories (connection_id, name, clone_url, default_branch, branches, index_status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 'pending', NOW(), NOW())
		`, connectionID, name, cloneURL, defaultBranch, branchesArg)
		if err != nil {
			return nil, fmt.Errorf("insert repository: %w", err)
		}

		id, _ := result.LastInsertId()

		return s.GetRepository(ctx, id)
	}

	err := s.pool.QueryRow(ctx, `
		INSERT INTO repositories (connection_id, name, clone_url, default_branch, branches, index_status)
		VALUES ($1, $2, $3, $4, $5, 'pending')
		RETURNING id, connection_id, name, clone_url, default_branch, branches,
				  last_indexed, index_status, excluded, deleted, archived, created_at, updated_at
	`, connectionID, name, cloneURL, defaultBranch, branches).Scan(
		&r.ID, &r.ConnectionID, &r.Name, &r.CloneURL, &r.DefaultBranch,
		&r.Branches, &r.LastIndexed, &r.IndexStatus, &r.Excluded, &r.Deleted, &r.Archived, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert repository: %w", err)
	}

	return &r, nil
}

// prepareBranchesArg converts branches to the appropriate format for the database driver.
func prepareBranchesArg(driver db.DriverType, branches []string) any {
	if driver == db.DriverMySQL {
		// MySQL: use JSON
		if branches == nil {
			return "[]"
		}

		data, _ := json.Marshal(branches)

		return string(data)
	}
	// PostgreSQL: native array
	return branches
}

// UpdateIndexStatus updates the indexing status of a repository
// Uses optimistic locking to prevent race conditions.
func (s *Service) UpdateIndexStatus(
	ctx context.Context,
	id int64,
	status string,
	indexed bool,
) error {
	var query string
	if indexed {
		query = `UPDATE repositories SET index_status = $2, last_indexed = NOW(), updated_at = NOW() WHERE id = $1`
	} else {
		query = `UPDATE repositories SET index_status = $2, updated_at = NOW() WHERE id = $1`
	}

	result, err := s.pool.Exec(ctx, query, id, status)
	if err != nil {
		return fmt.Errorf("update index status: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("repository %d not found", id)
	}

	return nil
}

// UpdateIndexStatusIfMatch updates status only if current status matches expected
// This prevents race conditions when multiple workers process the same repo.
func (s *Service) UpdateIndexStatusIfMatch(
	ctx context.Context,
	id int64,
	expectedStatus, newStatus string,
	indexed bool,
) (bool, error) {
	var query string
	if indexed {
		query = `UPDATE repositories SET index_status = $2, last_indexed = NOW(), updated_at = NOW()
		         WHERE id = $1 AND index_status = $3`
	} else {
		query = `UPDATE repositories SET index_status = $2, updated_at = NOW()
		         WHERE id = $1 AND index_status = $3`
	}

	result, err := s.pool.Exec(ctx, query, id, newStatus, expectedStatus)
	if err != nil {
		return false, fmt.Errorf("update index status: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()

	return rowsAffected > 0, nil
}

// TouchRepository updates the updated_at timestamp for a repository.
// This is used as a heartbeat during long-running indexing operations
// to prevent the stale cleanup from resetting the repository status.
func (s *Service) TouchRepository(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE repositories SET updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("touch repository: %w", err)
	}

	return nil
}

// DeleteRepository marks a repository as deleted (soft delete).
// Deleted repos are permanently excluded and won't be re-added on sync.
func (s *Service) DeleteRepository(ctx context.Context, id int64) error {
	result, err := s.pool.Exec(
		ctx,
		`UPDATE repositories SET deleted = true, excluded = true, updated_at = NOW() WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("delete repository: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("repository %d not found", id)
	}

	return nil
}

// ExcludeRepository marks a repository as excluded (soft delete)
// Excluded repos are skipped during sync and should be removed from the index.
func (s *Service) ExcludeRepository(ctx context.Context, id int64) error {
	result, err := s.pool.Exec(
		ctx,
		`UPDATE repositories SET excluded = true, updated_at = NOW() WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("exclude repository: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("repository %d not found", id)
	}

	return nil
}

// IncludeRepository marks a repository as included (un-exclude).
func (s *Service) IncludeRepository(ctx context.Context, id int64) error {
	result, err := s.pool.Exec(
		ctx,
		`UPDATE repositories SET excluded = false, index_status = 'pending', updated_at = NOW() WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("include repository: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("repository %d not found", id)
	}

	return nil
}

// RestoreRepository un-deletes a repository (clears both deleted and excluded flags).
// This allows a previously deleted repo to be re-added and indexed.
func (s *Service) RestoreRepository(ctx context.Context, id int64) error {
	result, err := s.pool.Exec(
		ctx,
		`UPDATE repositories SET deleted = false, excluded = false, index_status = 'pending', updated_at = NOW() WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("restore repository: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("repository %d not found", id)
	}

	return nil
}

// ArchivedRepo represents a repository that was newly archived and excluded during sync.
type ArchivedRepo struct {
	ID   int64
	Name string
}

// SyncRepositories fetches and syncs repositories from a connection
// Excluded repos are skipped - they won't be re-added or updated
// If excludeArchived is true, archived repos will be soft-deleted (excluded)
// If a repo becomes unarchived, it will be re-included on next sync.
// If allowedRepos is non-empty, only repos matching those patterns will be synced.
// If repoConfigs is non-empty, per-repo settings (branches, exclude) are applied.
// If cleanupArchived is true, returns repos that were newly excluded due to archival
// so the caller can queue cleanup jobs.
//
//nolint:gocyclo,cyclop
func (s *Service) SyncRepositories(
	ctx context.Context,
	connectionID int64,
	client CodeHostClient,
	excludeArchived bool,
	cleanupArchived bool,
	allowedRepos []string,
	repoConfigs []RepoConfigEntry,
) ([]ArchivedRepo, error) {
	var repos []CodeHostRepository
	var archivedRepos []ArchivedRepo

	// Build a map of repo configs for quick lookup
	repoConfigMap := make(map[string]RepoConfigEntry)
	for _, rc := range repoConfigs {
		repoConfigMap[rc.Name] = rc
	}

	log.Info("SyncRepositories called",
		zap.Int64("connection_id", connectionID),
		zap.Int("allowed_repos_count", len(allowedRepos)),
		zap.Strings("allowed_repos", allowedRepos),
		zap.Int("repo_configs_count", len(repoConfigs)),
	)

	// If specific repos are provided, try to fetch them directly first
	// This supports unauthenticated access to public repos
	if len(allowedRepos) > 0 && !containsWildcard(allowedRepos) {
		log.Info("Fetching specific repos directly (no wildcards)")
		// All repos are explicit names without wildcards - fetch them directly
		for _, repoName := range allowedRepos {
			repo, err := client.GetRepository(ctx, repoName)
			if err != nil {
				log.Warn("Failed to fetch repository",
					zap.String("repo", repoName),
					zap.Error(err),
				)

				continue
			}

			if repo != nil {
				repos = append(repos, *repo)
			}
		}
	} else {
		log.Info("Listing all repositories from code host")
		// List all repositories and filter
		var err error

		repos, err = client.ListRepositories(ctx)
		if err != nil {
			return nil, fmt.Errorf("list repositories: %w", err)
		}

		// If allowedRepos is specified, filter to only include those repos
		if len(allowedRepos) > 0 {
			repos = filterReposByAllowList(repos, allowedRepos)
		}
	}

	// Build a set of current repo names from code host for tracking
	codeHostRepos := make(map[string]CodeHostRepository)
	for _, repo := range repos {
		codeHostRepos[repo.FullName] = repo
	}

	isMySQL := s.pool.Driver() == db.DriverMySQL

	// Upsert each repository
	for _, repo := range repos {
		// Debug log the values being synced from the code host
		log.L.Debug("syncing repository from code host",
			zap.Int64("connection_id", connectionID),
			zap.String("repo", repo.FullName),
			zap.String("default_branch", repo.DefaultBranch),
			zap.String("clone_url", repo.CloneURL),
			zap.Bool("archived", repo.Archived),
		)

		shouldExclude := excludeArchived && repo.Archived

		if shouldExclude {
			// Check if repo currently exists and is not yet excluded (to detect transition)
			if cleanupArchived {
				var repoID int64
				var currentExcluded bool
				err := s.pool.QueryRow(ctx, `
					SELECT id, excluded FROM repositories
					WHERE connection_id = $1 AND name = $2
				`, connectionID, repo.FullName).Scan(&repoID, &currentExcluded)
				if err == nil && !currentExcluded {
					archivedRepos = append(archivedRepos, ArchivedRepo{ID: repoID, Name: repo.FullName})
				}
			}

			// Mark repo as archived and excluded
			_, err := s.pool.Exec(ctx, `
				UPDATE repositories
				SET archived = true, excluded = true, updated_at = NOW()
				WHERE connection_id = $1 AND name = $2
			`, connectionID, repo.FullName)
			if err != nil {
				return nil, fmt.Errorf("exclude archived repository %s: %w", repo.FullName, err)
			}

			continue
		}

		// Check for per-repo config
		repoConfig, hasConfig := repoConfigMap[repo.FullName]

		// For delete/exclude config settings, we only apply them to NEW repos.
		// If the repo already exists, UI changes take precedence over config.
		// This is checked in the INSERT ... ON CONFLICT query below - the INSERT
		// will set the initial state, but ON CONFLICT UPDATE won't change deleted/excluded.

		// Skip upsert for repos that should be deleted via config AND don't exist yet
		// (they'll be created as deleted in the upsert below)
		// But if they already exist and were restored via UI, we respect that.

		// Insert new repos, or update existing ones
		// Use configured branches if available, otherwise default branch
		branches := []string{repo.DefaultBranch}
		if hasConfig && len(repoConfig.Branches) > 0 {
			branches = repoConfig.Branches
		}

		branchesArg := prepareBranchesArg(s.pool.Driver(), branches)

		// When we have config-defined branches, always use them (force update)
		// Otherwise, preserve existing branches if set
		forceBranches := hasConfig && len(repoConfig.Branches) > 0

		// Determine initial deleted/excluded state from config (only applies to NEW repos)
		initialDeleted := hasConfig && repoConfig.Delete
		initialExcluded := hasConfig && (repoConfig.Exclude || repoConfig.Delete) // deleted repos are also excluded

		if isMySQL {
			// MySQL: use INSERT ... ON DUPLICATE KEY UPDATE
			// Note: deleted and excluded are set on INSERT but NOT updated on conflict
			// This ensures UI changes (restore/exclude) take precedence over config
			var err error
			if forceBranches {
				// Force branches from config
				_, err = s.pool.Exec(ctx, `
					INSERT INTO repositories (connection_id, name, clone_url, default_branch, branches, archived, deleted, excluded, index_status, created_at, updated_at)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending', NOW(), NOW())
					ON DUPLICATE KEY UPDATE
						clone_url = VALUES(clone_url),
						default_branch = VALUES(default_branch),
						branches = VALUES(branches),
						archived = VALUES(archived),
						excluded = CASE
							WHEN deleted = true THEN true
							WHEN archived = true AND VALUES(archived) = false THEN false
							ELSE excluded
						END,
						index_status = CASE
							WHEN deleted = true THEN index_status
							WHEN archived = true AND VALUES(archived) = false THEN 'pending'
							ELSE index_status
						END,
						updated_at = NOW()
				`, connectionID, repo.FullName, repo.CloneURL, repo.DefaultBranch, branchesArg, repo.Archived, initialDeleted, initialExcluded)
			} else {
				// Preserve existing branches if set, but update if only contains stale default branch
				_, err = s.pool.Exec(ctx, `
					INSERT INTO repositories (connection_id, name, clone_url, default_branch, branches, archived, deleted, excluded, index_status, created_at, updated_at)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending', NOW(), NOW())
					ON DUPLICATE KEY UPDATE
						clone_url = VALUES(clone_url),
						default_branch = VALUES(default_branch),
						branches = CASE
							WHEN JSON_LENGTH(branches) = 0 THEN VALUES(branches)
							WHEN JSON_LENGTH(branches) = 1 AND JSON_UNQUOTE(JSON_EXTRACT(branches, '$[0]')) != VALUES(default_branch) THEN VALUES(branches)
							ELSE branches
						END,
						archived = VALUES(archived),
						excluded = CASE
							WHEN deleted = true THEN true
							WHEN archived = true AND VALUES(archived) = false THEN false
							ELSE excluded
						END,
						index_status = CASE
							WHEN deleted = true THEN index_status
							WHEN archived = true AND VALUES(archived) = false THEN 'pending'
							ELSE index_status
						END,
						updated_at = NOW()
				`, connectionID, repo.FullName, repo.CloneURL, repo.DefaultBranch, branchesArg, repo.Archived, initialDeleted, initialExcluded)
			}

			if err != nil {
				return nil, fmt.Errorf("upsert repository %s: %w", repo.FullName, err)
			}
		} else {
			// PostgreSQL: use ON CONFLICT
			// Note: deleted and excluded are set on INSERT but NOT updated on conflict
			// This ensures UI changes (restore/exclude) take precedence over config
			var err error
			if forceBranches {
				// Force branches from config
				_, err = s.pool.Exec(ctx, `
					INSERT INTO repositories (connection_id, name, clone_url, default_branch, branches, archived, deleted, excluded, index_status, created_at, updated_at)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'pending', NOW(), NOW())
					ON CONFLICT (connection_id, name)
					DO UPDATE SET
						clone_url = $3,
						default_branch = $4,
						branches = $5,
						archived = $6,
						excluded = CASE
							WHEN repositories.deleted = true THEN true
							WHEN repositories.archived = true AND $6 = false THEN false
							ELSE repositories.excluded
						END,
						index_status = CASE
							WHEN repositories.deleted = true THEN repositories.index_status
							WHEN repositories.archived = true AND $6 = false THEN 'pending'
							ELSE repositories.index_status
						END,
						updated_at = NOW()
				`, connectionID, repo.FullName, repo.CloneURL, repo.DefaultBranch, branches, repo.Archived, initialDeleted, initialExcluded)
			} else {
				// Preserve existing branches if set, but update if only contains stale default branch
				_, err = s.pool.Exec(ctx, `
					INSERT INTO repositories (connection_id, name, clone_url, default_branch, branches, archived, deleted, excluded, index_status, created_at, updated_at)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'pending', NOW(), NOW())
					ON CONFLICT (connection_id, name)
					DO UPDATE SET
						clone_url = $3,
						default_branch = $4,
						branches = CASE
							WHEN cardinality(repositories.branches) = 0 THEN $5
							WHEN cardinality(repositories.branches) = 1 AND repositories.branches[1] != $4 THEN $5
							ELSE repositories.branches
						END,
						archived = $6,
						excluded = CASE
							WHEN repositories.deleted = true THEN true
							WHEN repositories.archived = true AND $6 = false THEN false
							ELSE repositories.excluded
						END,
						index_status = CASE
							WHEN repositories.deleted = true THEN repositories.index_status
							WHEN repositories.archived = true AND $6 = false THEN 'pending'
							ELSE repositories.index_status
						END,
						updated_at = NOW()
				`, connectionID, repo.FullName, repo.CloneURL, repo.DefaultBranch, branches, repo.Archived, initialDeleted, initialExcluded)
			}

			if err != nil {
				return nil, fmt.Errorf("upsert repository %s: %w", repo.FullName, err)
			}
		}
	}

	// Detect repos that were deleted from the remote code host.
	// Get all existing repos for this connection that are not excluded.
	existingRepos, err := s.ListRepositories(ctx, &connectionID)
	if err != nil {
		log.Warn("Failed to list existing repos for deletion detection",
			zap.Int64("connection_id", connectionID),
			zap.Error(err),
		)
		// Don't fail the sync, just skip deletion detection
		return archivedRepos, nil
	}

	// Find repos that exist in the database but not in the code host response
	deletedCount := 0

	for _, existingRepo := range existingRepos {
		// Skip repos that are already excluded
		if existingRepo.Excluded {
			continue
		}

		// Check if this repo is still on the code host
		if _, exists := codeHostRepos[existingRepo.Name]; !exists {
			// Repo was deleted from the code host - mark it as excluded
			log.Info("Repository no longer exists on code host, marking as excluded",
				zap.String("repo", existingRepo.Name),
				zap.Int64("connection_id", connectionID),
			)

			_, err := s.pool.Exec(ctx, `
				UPDATE repositories
				SET excluded = true, updated_at = NOW()
				WHERE id = $1
			`, existingRepo.ID)
			if err != nil {
				log.Warn("Failed to mark deleted repo as excluded",
					zap.String("repo", existingRepo.Name),
					zap.Error(err),
				)

				continue
			}

			deletedCount++
			// Note: Index shards will be cleaned up by the scheduler's orphan shard cleanup job
		}
	}

	if deletedCount > 0 {
		log.Info("Marked deleted repos as excluded",
			zap.Int64("connection_id", connectionID),
			zap.Int("count", deletedCount),
		)
	}

	return archivedRepos, nil
}

// CodeHostClient interface for syncing repositories.
type CodeHostClient interface {
	ListRepositories(ctx context.Context) ([]CodeHostRepository, error)
	// GetRepository fetches a single repository by name (e.g., "owner/repo").
	// Returns nil if the repository doesn't exist.
	GetRepository(ctx context.Context, name string) (*CodeHostRepository, error)
}

// CodeHostRepository represents a repository from a code host.
type CodeHostRepository struct {
	Name          string
	FullName      string
	CloneURL      string
	DefaultBranch string
	Private       bool
	Archived      bool
}

// filterReposByAllowList filters repositories to only include those matching the allow list.
// If allowedRepos is empty, all repos are returned (no filtering).
// The matching is case-insensitive and supports:
// - Exact matches (e.g., "owner/repo")
// - Wildcard patterns (e.g., "owner/*" or "*/repo").
func filterReposByAllowList(
	repos []CodeHostRepository,
	allowedRepos []string,
) []CodeHostRepository {
	if len(allowedRepos) == 0 {
		return repos
	}

	// Build a set for exact matches and collect patterns
	exactMatches := make(map[string]bool)

	var patterns []string

	for _, r := range allowedRepos {
		lower := strings.ToLower(r)
		if strings.Contains(lower, "*") {
			patterns = append(patterns, lower)
		} else {
			exactMatches[lower] = true
		}
	}

	var filtered []CodeHostRepository

	for _, repo := range repos {
		repoName := strings.ToLower(repo.FullName)

		// Check exact match first
		if exactMatches[repoName] {
			filtered = append(filtered, repo)
			continue
		}

		// Check wildcard patterns
		for _, pattern := range patterns {
			if matchWildcard(pattern, repoName) {
				filtered = append(filtered, repo)
				break
			}
		}
	}

	return filtered
}

// matchWildcard matches a simple wildcard pattern against a string.
// Supports * as a wildcard for any sequence of characters.
func matchWildcard(pattern, s string) bool {
	// Simple wildcard matching: convert to parts split by *
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == s
	}

	// Check prefix
	if parts[0] != "" && !strings.HasPrefix(s, parts[0]) {
		return false
	}

	// Check suffix
	lastPart := parts[len(parts)-1]
	if lastPart != "" && !strings.HasSuffix(s, lastPart) {
		return false
	}

	// Check middle parts in order
	pos := len(parts[0])
	for i := 1; i < len(parts)-1; i++ {
		idx := strings.Index(s[pos:], parts[i])
		if idx == -1 {
			return false
		}

		pos += idx + len(parts[i])
	}

	return true
}

// containsWildcard checks if any of the repo patterns contain wildcards.
func containsWildcard(repos []string) bool {
	for _, r := range repos {
		if strings.Contains(r, "*") {
			return true
		}
	}

	return false
}

// ScheduleIndex marks a repository for indexing.
func (s *Service) ScheduleIndex(ctx context.Context, repoID int64) error {
	return s.UpdateIndexStatus(ctx, repoID, "pending", false)
}

// GetPendingIndexJobs returns repositories that need indexing
// Uses FOR UPDATE SKIP LOCKED to prevent multiple workers from picking the same jobs.
func (s *Service) GetPendingIndexJobs(ctx context.Context, limit int) ([]Repository, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, connection_id, name, clone_url, default_branch, branches,
			   last_indexed, index_status, excluded, deleted, archived, created_at, updated_at
		FROM repositories
		WHERE index_status = 'pending' AND excluded = false AND deleted = false
		ORDER BY created_at
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query pending jobs: %w", err)
	}
	defer rows.Close()

	var repos []Repository

	for rows.Next() {
		var r Repository

		err := rows.Scan(
			&r.ID,
			&r.ConnectionID,
			&r.Name,
			&r.CloneURL,
			&r.DefaultBranch,
			&r.Branches,
			&r.LastIndexed,
			&r.IndexStatus,
			&r.Excluded,
			&r.Deleted,
			&r.Archived,
			&r.CreatedAt,
			&r.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan repository: %w", err)
		}

		repos = append(repos, r)
	}

	return repos, rows.Err()
}

// ClaimPendingIndexJob atomically claims a single pending job for this worker
// Uses SELECT FOR UPDATE SKIP LOCKED to prevent race conditions.
func (s *Service) ClaimPendingIndexJob(ctx context.Context, workerID string) (*Repository, error) {
	// Start a transaction
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	// Select and lock a single pending, non-excluded repository
	var r Repository

	err = tx.QueryRow(ctx, `
		SELECT id, connection_id, name, clone_url, default_branch, branches,
			   last_indexed, index_status, excluded, deleted, archived, created_at, updated_at
		FROM repositories
		WHERE index_status = 'pending' AND excluded = false AND deleted = false
		ORDER BY created_at
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`).Scan(
		&r.ID, &r.ConnectionID, &r.Name, &r.CloneURL, &r.DefaultBranch,
		&r.Branches, &r.LastIndexed, &r.IndexStatus, &r.Excluded, &r.Deleted, &r.Archived, &r.CreatedAt, &r.UpdatedAt,
	)
	if db.IsNoRowsError(err) {
		return nil, nil // No pending jobs
	}

	if err != nil {
		return nil, fmt.Errorf("select pending job: %w", err)
	}

	// Update status to 'indexing' with worker ID
	_, err = tx.Exec(ctx, `
		UPDATE repositories
		SET index_status = 'indexing', updated_at = NOW()
		WHERE id = $1
	`, r.ID)
	if err != nil {
		return nil, fmt.Errorf("claim job: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	r.IndexStatus = "indexing"

	return &r, nil
}

// GetStats returns repository statistics.
func (s *Service) GetStats(ctx context.Context) (*RepoStats, error) {
	var stats RepoStats

	// Build database-agnostic query using SQLBuilder
	query := fmt.Sprintf(`
		SELECT
			COUNT(*) as total,
			%s as indexed,
			%s as pending,
			%s as indexing,
			%s as failed,
			%s as excluded
		FROM repositories
	`,
		s.sql.CountFilter("index_status = 'indexed' AND excluded = false AND deleted = false"),
		s.sql.CountFilter("index_status = 'pending' AND excluded = false AND deleted = false"),
		s.sql.CountFilter("index_status = 'indexing' AND excluded = false AND deleted = false"),
		s.sql.CountFilter("index_status = 'failed' AND excluded = false AND deleted = false"),
		s.sql.CountFilter("excluded = true AND deleted = false"),
	)

	err := s.pool.QueryRow(ctx, query).
		Scan(&stats.Total, &stats.Indexed, &stats.Pending, &stats.Indexing, &stats.Failed, &stats.Excluded)
	if err != nil {
		return nil, fmt.Errorf("query stats: %w", err)
	}

	return &stats, nil
}

// RepoStats contains repository statistics.
type RepoStats struct {
	Total    int `json:"total"`
	Indexed  int `json:"indexed"`
	Pending  int `json:"pending"`
	Indexing int `json:"indexing"`
	Failed   int `json:"failed"`
	Excluded int `json:"excluded"`
	Deleted  int `json:"deleted"`
	Stale    int `json:"stale,omitempty"`
}

// GetConnectionStats returns statistics for a specific connection.
func (s *Service) GetConnectionStats(ctx context.Context, connectionID int64) (*RepoStats, error) {
	var stats RepoStats

	query := `
		SELECT
			COUNT(*) as total,
			SUM(CASE WHEN index_status = 'indexed' AND excluded = false AND deleted = false THEN 1 ELSE 0 END) as indexed,
			SUM(CASE WHEN index_status = 'pending' AND excluded = false AND deleted = false THEN 1 ELSE 0 END) as pending,
			SUM(CASE WHEN index_status = 'indexing' AND excluded = false AND deleted = false THEN 1 ELSE 0 END) as indexing,
			SUM(CASE WHEN index_status = 'failed' AND excluded = false AND deleted = false THEN 1 ELSE 0 END) as failed,
			SUM(CASE WHEN excluded = true AND deleted = false THEN 1 ELSE 0 END) as excluded,
			SUM(CASE WHEN deleted = true THEN 1 ELSE 0 END) as deleted
		FROM repositories
		WHERE connection_id = $1
	`

	err := s.pool.QueryRow(ctx, query, connectionID).
		Scan(&stats.Total, &stats.Indexed, &stats.Pending, &stats.Indexing, &stats.Failed, &stats.Excluded, &stats.Deleted)
	if err != nil {
		return nil, fmt.Errorf("query connection stats: %w", err)
	}

	return &stats, nil
}

// UpdateBranches updates the branches for a repository.
func (s *Service) UpdateBranches(ctx context.Context, repoID int64, branches []string) error {
	branchesArg := prepareBranchesArg(s.pool.Driver(), branches)

	_, err := s.pool.Exec(ctx, `
		UPDATE repositories
		SET branches = $2, updated_at = NOW()
		WHERE id = $1
	`, repoID, branchesArg)
	if err != nil {
		return fmt.Errorf("update branches: %w", err)
	}

	return nil
}

// GetRepositoriesByStatus returns repositories with a specific status.
func (s *Service) GetRepositoriesByStatus(
	ctx context.Context,
	status string,
	limit int,
) ([]Repository, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, connection_id, name, clone_url, default_branch, branches,
			   last_indexed, index_status, excluded, deleted, archived, created_at, updated_at
		FROM repositories
		WHERE index_status = $1 AND excluded = false AND deleted = false
		ORDER BY updated_at DESC
		LIMIT $2
	`, status, limit)
	if err != nil {
		return nil, fmt.Errorf("query repositories by status: %w", err)
	}
	defer rows.Close()

	var repos []Repository

	for rows.Next() {
		var r Repository

		err := rows.Scan(
			&r.ID,
			&r.ConnectionID,
			&r.Name,
			&r.CloneURL,
			&r.DefaultBranch,
			&r.Branches,
			&r.LastIndexed,
			&r.IndexStatus,
			&r.Excluded,
			&r.Deleted,
			&r.Archived,
			&r.CreatedAt,
			&r.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan repository: %w", err)
		}

		repos = append(repos, r)
	}

	return repos, rows.Err()
}

// ReindexAll marks all indexed repositories for re-indexing.
func (s *Service) ReindexAll(ctx context.Context) (int64, error) {
	result, err := s.pool.Exec(ctx, `
		UPDATE repositories
		SET index_status = 'pending', updated_at = NOW()
		WHERE index_status = 'indexed' AND excluded = false AND deleted = false
	`)
	if err != nil {
		return 0, fmt.Errorf("reindex all: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()

	return rowsAffected, nil
}

// ReindexConnection marks all repositories in a connection for re-indexing.
func (s *Service) ReindexConnection(ctx context.Context, connectionID int64) (int64, error) {
	result, err := s.pool.Exec(ctx, `
		UPDATE repositories
		SET index_status = 'pending', updated_at = NOW()
		WHERE connection_id = $1 AND index_status = 'indexed' AND excluded = false AND deleted = false
	`, connectionID)
	if err != nil {
		return 0, fmt.Errorf("reindex connection: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()

	return rowsAffected, nil
}

// CleanupStaleIndexing resets repositories stuck in 'indexing' or 'cloning' state for too long.
func (s *Service) CleanupStaleIndexing(
	ctx context.Context,
	olderThan time.Duration,
) (int64, error) {
	threshold := time.Now().Add(-olderThan)

	result, err := s.pool.Exec(ctx, `
		UPDATE repositories
		SET index_status = 'pending', updated_at = NOW()
		WHERE index_status IN ('indexing', 'cloning') AND updated_at < $1
	`, threshold)
	if err != nil {
		return 0, fmt.Errorf("cleanup stale indexing: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()

	return rowsAffected, nil
}

// GetConnectionByName retrieves a connection by name.
func (s *Service) GetConnectionByName(ctx context.Context, name string) (*Connection, error) {
	var c Connection

	err := s.pool.QueryRow(ctx, `
		SELECT id, name, type, url, token, exclude_archived, cleanup_archived, created_at, updated_at
		FROM connections
		WHERE name = $1
	`, name).Scan(&c.ID, &c.Name, &c.Type, &c.URL, &c.Token, &c.ExcludeArchived, &c.CleanupArchived, &c.CreatedAt, &c.UpdatedAt)
	if db.IsNoRowsError(err) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("query connection by name: %w", err)
	}

	// Decrypt token for use
	c.Token = s.decryptToken(c.Token)

	return &c, nil
}

// GetRepoStats returns repository statistics grouped by index status.
func (s *Service) GetRepoStats(ctx context.Context, staleThreshold time.Time) (*RepoStats, error) {
	stats := &RepoStats{}

	// Query all stats in a single query for efficiency
	err := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) as total,
			SUM(CASE WHEN index_status = 'indexed' AND excluded = false AND deleted = false THEN 1 ELSE 0 END) as indexed,
			SUM(CASE WHEN index_status = 'pending' AND excluded = false AND deleted = false THEN 1 ELSE 0 END) as pending,
			SUM(CASE WHEN index_status = 'indexing' AND excluded = false AND deleted = false THEN 1 ELSE 0 END) as indexing,
			SUM(CASE WHEN index_status = 'failed' AND excluded = false AND deleted = false THEN 1 ELSE 0 END) as failed,
			SUM(CASE WHEN excluded = true AND deleted = false THEN 1 ELSE 0 END) as excluded,
			SUM(CASE WHEN deleted = true THEN 1 ELSE 0 END) as deleted
		FROM repositories
	`).Scan(&stats.Total, &stats.Indexed, &stats.Pending, &stats.Indexing, &stats.Failed, &stats.Excluded, &stats.Deleted)
	if err != nil {
		return nil, fmt.Errorf("query repo stats: %w", err)
	}

	// Count stale repos (only non-deleted, non-excluded)
	err = s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM repositories
		WHERE index_status = 'indexed'
		AND excluded = false
		AND deleted = false
		AND (last_indexed IS NULL OR last_indexed < $1)
	`, staleThreshold).Scan(&stats.Stale)
	if err != nil {
		// Don't fail on stale count error
		stats.Stale = 0
	}

	return stats, nil
}

// SetRepoPollInterval sets a custom poll interval for a repository.
// Pass 0 to reset to default (NULL).
func (s *Service) SetRepoPollInterval(
	ctx context.Context,
	repoID int64,
	intervalSeconds int,
) error {
	var (
		query string
		err   error
	)

	if intervalSeconds == 0 {
		query = `UPDATE repositories SET poll_interval_seconds = NULL, updated_at = NOW() WHERE id = $1`
		_, err = s.pool.Exec(ctx, query, repoID)
	} else {
		query = `UPDATE repositories SET poll_interval_seconds = $2, updated_at = NOW() WHERE id = $1`
		_, err = s.pool.Exec(ctx, query, repoID, intervalSeconds)
	}

	if err != nil {
		return fmt.Errorf("set poll interval: %w", err)
	}

	return nil
}

// TriggerSyncStaleRepos marks stale repos as pending for re-indexing.
// Returns the count of repos marked for sync.
func (s *Service) TriggerSyncStaleRepos(
	ctx context.Context,
	staleThreshold time.Time,
) (int, error) {
	if s.pool.Driver() == db.DriverMySQL {
		result, err := s.pool.Exec(ctx, `
			UPDATE repositories
			SET index_status = 'pending', updated_at = NOW()
			WHERE index_status = 'indexed'
			AND (last_indexed IS NULL OR last_indexed < ?)
		`, staleThreshold)
		if err != nil {
			return 0, fmt.Errorf("trigger sync stale repos: %w", err)
		}

		count, _ := result.RowsAffected()

		return int(count), nil
	}

	// PostgreSQL: use RETURNING to count affected rows
	rows, err := s.pool.Query(ctx, `
		UPDATE repositories
		SET index_status = 'pending', updated_at = NOW()
		WHERE index_status = 'indexed'
		AND (last_indexed IS NULL OR last_indexed < $1)
		RETURNING id
	`, staleThreshold)
	if err != nil {
		return 0, fmt.Errorf("trigger sync stale repos: %w", err)
	}
	defer rows.Close()

	var count int

	for rows.Next() {
		var id int64

		if err := rows.Scan(&id); err != nil {
			continue // Skip rows that fail to scan
		}

		count++
	}

	return count, nil
}

// TriggerSyncAllRepos marks all indexed or failed repos as pending for re-indexing.
// Returns the count of repos marked for sync.
func (s *Service) TriggerSyncAllRepos(ctx context.Context) (int, error) {
	if s.pool.Driver() == db.DriverMySQL {
		result, err := s.pool.Exec(ctx, `
			UPDATE repositories
			SET index_status = 'pending', updated_at = NOW()
			WHERE index_status IN ('indexed', 'failed')
		`)
		if err != nil {
			return 0, fmt.Errorf("trigger sync all repos: %w", err)
		}

		count, _ := result.RowsAffected()

		return int(count), nil
	}

	// PostgreSQL: use RETURNING to count affected rows
	rows, err := s.pool.Query(ctx, `
		UPDATE repositories
		SET index_status = 'pending', updated_at = NOW()
		WHERE index_status IN ('indexed', 'failed')
		RETURNING id
	`)
	if err != nil {
		return 0, fmt.Errorf("trigger sync all repos: %w", err)
	}
	defer rows.Close()

	var count int

	for rows.Next() {
		var id int64

		if err := rows.Scan(&id); err != nil {
			continue // Skip rows that fail to scan
		}

		count++
	}

	return count, nil
}
