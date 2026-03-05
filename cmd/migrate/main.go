package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/aanogueira/code-search/internal/db"
	"github.com/aanogueira/code-search/internal/log"
)

func main() {
	// Initialize logger
	if err := log.InitFromEnv(); err != nil {
		panic("failed to initialize logger: " + err.Error())
	}

	defer log.Sync()

	if len(os.Args) < 2 {
		fmt.Println("Usage: migrate [-dir <path>] [up|down|status|add <name>]")
		os.Exit(1)
	}

	// Parse optional -dir flag
	customDir := ""
	args := os.Args[1:]

	if len(args) >= 2 && args[0] == "-dir" {
		customDir = args[1]
		args = args[2:]
	}

	if len(args) < 1 {
		fmt.Println("Usage: migrate [-dir <path>] [up|down|status|add <name>]")
		os.Exit(1)
	}

	command := args[0]

	// Handle 'add' command separately (doesn't need database)
	if command == "add" {
		if len(args) < 2 {
			fmt.Println("Usage: migrate add <migration_name>")
			os.Exit(1)
		}

		if err := addMigration(args[1], customDir); err != nil {
			log.Fatal("Failed to add migration", log.Err(err))
		}

		return
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		// Also check CS_DATABASE_URL for consistency with other services
		databaseURL = os.Getenv("CS_DATABASE_URL")
	}

	if databaseURL == "" {
		log.Fatal("DATABASE_URL or CS_DATABASE_URL environment variable is required")
	}

	// Detect driver from URL
	driverType := db.DetectDriver(databaseURL)
	driver := string(driverType)

	log.Info("Starting migration",
		log.String("command", command),
		log.String("driver", driver),
	)

	// Determine migrations folder based on driver (or use custom dir)
	migrationsDir := customDir
	if migrationsDir == "" {
		migrationsDir = "migrations/" + driver
	}

	// Convert DSN for appropriate driver
	dsn := databaseURL

	var sqlDriver string

	switch driverType {
	case db.DriverPostgres:
		sqlDriver = "pgx"
	case db.DriverMySQL:
		// Convert mysql:// URL to DSN format
		dsn = convertMySQLURL(databaseURL)
		sqlDriver = "mysql"
	default:
		log.Fatal("Unsupported database driver. URL must start with postgres:// or mysql://")
	}

	// Open database connection
	sqlDB, err := sql.Open(sqlDriver, dsn)
	if err != nil {
		log.Fatal("Failed to open database", log.Err(err))
	}
	defer sqlDB.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Test connection
	log.Info("Connecting to database...")

	if err := sqlDB.PingContext(ctx); err != nil {
		log.Fatal("Failed to connect to database", log.Err(err))
	}

	log.Info("Connected to database")

	// Ensure schema_migrations table exists
	if err := ensureMigrationsTable(ctx, sqlDB, driver); err != nil {
		log.Fatal("Failed to create migrations table", log.Err(err))
	}

	switch command {
	case "up":
		err := migrateUp(ctx, sqlDB, driver, migrationsDir)
		if err != nil {
			log.Fatal("Migration up failed", log.Err(err))
		}

		log.Info("Migration up completed successfully")
	case "down":
		err := migrateDown(ctx, sqlDB, driver, migrationsDir)
		if err != nil {
			log.Fatal("Migration down failed", log.Err(err))
		}

		log.Info("Migration down completed successfully")
	case "status":
		err := showStatus(ctx, sqlDB, driver, migrationsDir)
		if err != nil {
			log.Fatal("Status check failed", log.Err(err))
		}
	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Usage: migrate [up|down|status|add <name>]")
		os.Exit(1)
	}
}

// addMigration creates a new migration file pair.
func addMigration(name string, customDir string) error {
	migrationsDir := customDir
	if migrationsDir == "" {
		// Determine driver from DATABASE_URL or default to postgres
		databaseURL := os.Getenv("DATABASE_URL")
		if databaseURL == "" {
			databaseURL = os.Getenv("CS_DATABASE_URL")
		}

		driver := "postgres" // default

		if databaseURL != "" {
			driverType := db.DetectDriver(databaseURL)
			driver = string(driverType)
		}

		migrationsDir = "migrations/" + driver
	}

	// Generate timestamp-based version
	version := time.Now().Format("20060102150405")

	upFile := filepath.Join(migrationsDir, fmt.Sprintf("%s_%s.sql", version, name))
	downFile := filepath.Join(migrationsDir, fmt.Sprintf("%s_%s_down.sql", version, name))

	// Create up migration
	upContent := fmt.Sprintf(
		"-- Migration: %s\n-- Created: %s\n\n-- Add your migration SQL here\n",
		name,
		time.Now().Format(time.RFC3339),
	)
	if err := os.WriteFile(upFile, []byte(upContent), 0o644); err != nil {
		return fmt.Errorf("creating up migration: %w", err)
	}

	// Create down migration
	downContent := fmt.Sprintf(
		"-- Migration: %s (down)\n-- Created: %s\n\n-- Add your rollback SQL here\n",
		name,
		time.Now().Format(time.RFC3339),
	)
	if err := os.WriteFile(downFile, []byte(downContent), 0o644); err != nil {
		return fmt.Errorf("creating down migration: %w", err)
	}

	log.Info("Created migration files",
		log.String("up", upFile),
		log.String("down", downFile),
	)

	return nil
}

// convertMySQLURL converts mysql:// URL to DSN format
// mysql://user:pass@host:port/dbname -> user:pass@tcp(host:port)/dbname
func convertMySQLURL(url string) string {
	url = strings.TrimPrefix(url, "mysql://")

	// Split user:pass@host:port/dbname
	atIdx := strings.LastIndex(url, "@")
	if atIdx == -1 {
		return url
	}

	userPass := url[:atIdx]
	hostPath := url[atIdx+1:]

	// Split host:port/dbname
	slashIdx := strings.Index(hostPath, "/")
	if slashIdx == -1 {
		return fmt.Sprintf("%s@tcp(%s)/", userPass, hostPath)
	}

	host := hostPath[:slashIdx]
	dbname := hostPath[slashIdx+1:]

	// Add parseTime=true for proper time handling
	if strings.Contains(dbname, "?") {
		dbname += "&parseTime=true"
	} else {
		dbname += "?parseTime=true"
	}

	return fmt.Sprintf("%s@tcp(%s)/%s", userPass, host, dbname)
}

func ensureMigrationsTable(ctx context.Context, db *sql.DB, driver string) error {
	var query string

	switch driver {
	case "postgres":
		query = `
			CREATE TABLE IF NOT EXISTS schema_migrations (
				version TEXT PRIMARY KEY,
				applied_at TIMESTAMPTZ DEFAULT NOW()
			)`
	case "mysql":
		query = `
			CREATE TABLE IF NOT EXISTS schema_migrations (
				version VARCHAR(255) PRIMARY KEY,
				applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`
	}

	_, err := db.ExecContext(ctx, query)

	return err
}

func migrateUp(
	ctx context.Context,
	sqlDB *sql.DB,
	driver, migrationsDir string,
) error {
	migrations, err := findMigrations(migrationsDir, "up")
	if err != nil {
		return fmt.Errorf("finding migrations: %w", err)
	}

	applied, err := getAppliedMigrations(ctx, sqlDB)
	if err != nil {
		return fmt.Errorf("getting applied migrations: %w", err)
	}

	pendingCount := 0

	for _, m := range migrations {
		if !applied[m.version] {
			pendingCount++
		}
	}

	if pendingCount == 0 {
		log.Info("No pending migrations")
		return nil
	}

	log.Info("Found pending migrations", log.Int("count", pendingCount))

	appliedCount := 0

	for _, m := range migrations {
		if applied[m.version] {
			continue
		}

		log.Info("Applying migration",
			log.String("version", m.version),
			log.String("file", m.filename),
		)

		content, err := os.ReadFile(m.path)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", m.filename, err)
		}

		// Execute each statement separately
		statements := splitStatements(string(content))
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}

			if _, err := sqlDB.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf(
					"executing migration %s: %w\nStatement: %s",
					m.filename,
					err,
					stmt,
				)
			}
		}

		// Record migration
		var insertQuery string

		switch driver {
		case "postgres":
			insertQuery = "INSERT INTO schema_migrations (version) VALUES ($1)"
		case "mysql":
			insertQuery = "INSERT INTO schema_migrations (version) VALUES (?)"
		}

		if _, err := sqlDB.ExecContext(ctx, insertQuery, m.version); err != nil {
			return fmt.Errorf("recording migration %s: %w", m.version, err)
		}

		log.Info("Applied migration", log.String("version", m.version))

		appliedCount++
	}

	log.Info("Migrations completed", log.Int("applied", appliedCount))

	return nil
}

func migrateDown(
	ctx context.Context,
	sqlDB *sql.DB,
	driver, migrationsDir string,
) error {
	migrations, err := findMigrations(migrationsDir, "down")
	if err != nil {
		return fmt.Errorf("finding migrations: %w", err)
	}

	applied, err := getAppliedMigrations(ctx, sqlDB)
	if err != nil {
		return fmt.Errorf("getting applied migrations: %w", err)
	}

	// Sort descending for down migrations
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version > migrations[j].version
	})

	// Only revert the latest migration
	for _, m := range migrations {
		if !applied[m.version] {
			continue
		}

		log.Info("Reverting migration",
			log.String("version", m.version),
			log.String("file", m.filename),
		)

		content, err := os.ReadFile(m.path)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", m.filename, err)
		}

		statements := splitStatements(string(content))
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}

			if _, err := sqlDB.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf(
					"executing migration %s: %w\nStatement: %s",
					m.filename,
					err,
					stmt,
				)
			}
		}

		// Remove migration record
		var deleteQuery string

		switch driver {
		case "postgres":
			deleteQuery = "DELETE FROM schema_migrations WHERE version = $1"
		case "mysql":
			deleteQuery = "DELETE FROM schema_migrations WHERE version = ?"
		}

		if _, err := sqlDB.ExecContext(ctx, deleteQuery, m.version); err != nil {
			return fmt.Errorf("removing migration record %s: %w", m.version, err)
		}

		log.Info("Reverted migration", log.String("version", m.version))

		break // Only revert one migration at a time
	}

	return nil
}

func showStatus(
	ctx context.Context,
	sqlDB *sql.DB,
	driver, migrationsDir string,
) error {
	migrations, err := findMigrations(migrationsDir, "up")
	if err != nil {
		return fmt.Errorf("finding migrations: %w", err)
	}

	applied, err := getAppliedMigrations(ctx, sqlDB)
	if err != nil {
		return fmt.Errorf("getting applied migrations: %w", err)
	}

	fmt.Printf("\nMigration Status (driver: %s, dir: %s)\n", driver, migrationsDir)
	fmt.Println(strings.Repeat("-", 60))

	appliedCount := 0
	pendingCount := 0

	for _, m := range migrations {
		status := "pending"
		if applied[m.version] {
			status = "applied"
			appliedCount++
		} else {
			pendingCount++
		}

		fmt.Printf("%-20s %s\n", m.version, status)
	}

	fmt.Println()
	log.Info("Migration status",
		log.Int("applied", appliedCount),
		log.Int("pending", pendingCount),
		log.Int("total", len(migrations)),
	)

	return nil
}

type migration struct {
	version  string
	filename string
	path     string
}

func findMigrations(dir string, direction string) ([]migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading migrations directory %s: %w", dir, err)
	}

	var suffix string
	if direction == "down" {
		suffix = "_down.sql"
	} else {
		suffix = ".sql"
	}

	// Pattern: 20251101000001_name.sql or 20251101000001_name_down.sql
	versionPattern := regexp.MustCompile(`^(\d+)_`)

	var migrations []migration

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		// Skip down files when looking for up, and vice versa
		if direction == "up" && strings.HasSuffix(name, "_down.sql") {
			continue
		}

		if direction == "down" && !strings.HasSuffix(name, "_down.sql") {
			continue
		}

		if !strings.HasSuffix(name, suffix) && direction == "up" {
			continue
		}

		matches := versionPattern.FindStringSubmatch(name)
		if len(matches) < 2 {
			continue
		}

		migrations = append(migrations, migration{
			version:  matches[1],
			filename: name,
			path:     filepath.Join(dir, name),
		})
	}

	// Sort by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})

	return migrations, nil
}

func getAppliedMigrations(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)

	for rows.Next() {
		var version string

		err := rows.Scan(&version)
		if err != nil {
			return nil, err
		}

		applied[version] = true
	}

	return applied, rows.Err()
}

// splitStatements splits SQL content into individual statements
// while respecting dollar-quoted strings (PostgreSQL functions).
func splitStatements(content string) []string {
	var (
		statements []string
		current    strings.Builder
	)

	lines := strings.Split(content, "\n")
	inDollarQuote := false
	dollarTag := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments (but only if not in a dollar quote)
		if !inDollarQuote {
			if trimmed == "" || strings.HasPrefix(trimmed, "--") {
				continue
			}
		}

		current.WriteString(line)
		current.WriteString("\n")

		// Check for dollar quote boundaries
		if !inDollarQuote {
			// Look for opening dollar quote: $$ or $tag$
			if idx := findDollarQuoteStart(trimmed); idx != -1 {
				dollarTag = extractDollarTag(trimmed, idx)
				// Check if it closes on the same line
				remainder := trimmed[idx+len(dollarTag):]
				if !strings.Contains(remainder, dollarTag) {
					inDollarQuote = true
				}
			}
		} else {
			// Look for closing dollar quote
			if strings.Contains(trimmed, dollarTag) {
				// Count occurrences - if odd number after the first, we're still inside
				count := strings.Count(trimmed, dollarTag)
				if count%2 == 1 || strings.HasSuffix(trimmed, dollarTag+";") || strings.HasSuffix(trimmed, dollarTag+" LANGUAGE") || strings.Contains(trimmed, dollarTag+" ") {
					inDollarQuote = false
					dollarTag = ""
				}
			}
		}

		// Only split on ; if we're not inside a dollar-quoted string
		if !inDollarQuote && strings.HasSuffix(trimmed, ";") {
			stmt := strings.TrimSpace(current.String())
			if stmt != "" && stmt != ";" {
				statements = append(statements, stmt)
			}

			current.Reset()
		}
	}

	// Handle any remaining content
	if stmt := strings.TrimSpace(current.String()); stmt != "" {
		statements = append(statements, stmt)
	}

	return statements
}

// findDollarQuoteStart finds the position of a dollar quote start.
func findDollarQuoteStart(s string) int {
	// Match $$ or $identifier$
	for i := range len(s) {
		if s[i] == '$' {
			// Check for $$
			if i+1 < len(s) && s[i+1] == '$' {
				return i
			}
			// Check for $tag$
			for j := i + 1; j < len(s); j++ {
				if s[j] == '$' {
					return i
				}

				if !isIdentChar(s[j]) {
					break
				}
			}
		}
	}

	return -1
}

// extractDollarTag extracts the full dollar tag (e.g., "$$" or "$func$").
func extractDollarTag(s string, start int) string {
	if start+1 < len(s) && s[start+1] == '$' {
		return "$$"
	}

	for i := start + 1; i < len(s); i++ {
		if s[i] == '$' {
			return s[start : i+1]
		}
	}

	return "$$"
}

// isIdentChar returns true if c is valid in an identifier.
func isIdentChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}
