# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Code Search is a self-hosted code search and bulk operations platform powered by Zoekt (Google's code search engine). The system consists of three main services:
- **API Server** (Go): REST API with chi router, handles user requests
- **Indexer** (Go): Background worker that clones repos and creates search indexes
- **Web UI** (Next.js): Frontend interface

## Prerequisites

- Go 1.21+
- Node.js 20+ with bun package manager
- PostgreSQL 16+ or MySQL 8+
- Redis 7+
- Zoekt binaries (installed via `make dev-setup`)

## Common Commands

### First-Time Setup
```bash
make dev-setup    # Install dependencies, start infrastructure, run migrations
```

### Development (run in separate terminals)
```bash
make dev-infra    # Start PostgreSQL, Redis, Zoekt (required first)
make dev-zoekt    # Run Zoekt webserver on :6070
make dev-api      # Run API server on :8080
make dev-indexer  # Run indexer worker
make dev-web      # Run Next.js frontend on :3000 (uses bun)
```

### Building
```bash
make build        # Build all binaries → bin/
go build -o bin/code-search ./cmd/cli      # Build specific binary
```

### Testing
```bash
make test         # Run Go tests
make test-cover   # Generate coverage report
go test -v ./internal/queue/...           # Run specific package tests
go test -v -run TestIndexJob ./internal/indexer/...  # Run specific test
cd web && bun run test      # Run frontend tests (vitest watch mode)
cd web && bun run test:run  # Run frontend tests once
```

### Linting
```bash
make lint         # Run golangci-lint
cd web && bun lint         # Run ESLint on frontend
cd web && bun lint:fix     # Auto-fix linting issues
```

### Database Migrations
```bash
make migrate-up               # Apply pending migrations
make migrate-down             # Rollback last migration
make migrate-add name=<name>  # Create new migration
make migrate-status           # Show migration status
```

### Docker
```bash
make docker-up    # Start all services in Docker
make docker-down  # Stop all services
make docker-logs name=api  # View service logs
```

### Other Useful Commands
```bash
make help         # Show all available make commands
make clean        # Clean build artifacts
make vet          # Run go vet static analysis
```

## Architecture Overview

### Service Architecture
```
┌─────────────┐
│   Web UI    │  Next.js (port 3000)
└──────┬──────┘
       │
┌──────▼──────┐
│ API Server  │  Go/chi (port 8080)
└──────┬──────┘
       │
       ├─────────► PostgreSQL (metadata: repos, connections, jobs)
       ├─────────► Redis (job queue)
       └─────────► Zoekt (search index, port 6070)
                     ▲
                     │
              ┌──────┴──────┐
              │   Indexer   │  Go worker (clones & indexes repos)
              └─────────────┘
```

### Key Internal Packages

**Entry Points (`cmd/`):**
- `cmd/api/` - REST API server (chi router)
- `cmd/indexer/` - Background job worker
- `cmd/cli/` - CLI tool for users
- `cmd/migrate/` - Database migration utility
- `cmd/zoekt-refresh/` - Refreshes Zoekt indexes

**Core Services (`internal/`):**
- `internal/queue/` - Redis-based job queue with sharding support
- `internal/indexer/` - Job processing (clone, index, sync, replace, cleanup)
- `internal/scheduler/` - Auto-sync scheduler, runs in API server
- `internal/repos/` - Repository and connection management
- `internal/search/` - Zoekt search client (single or sharded)
- `internal/replace/` - Find & replace with automated MR/PR creation
- `internal/files/` - File browsing (tree, blob, symbols)
- `internal/codehost/` - GitHub/GitLab/Gitea/Bitbucket integration
- `internal/db/` - Database abstraction (Postgres/MySQL)
- `internal/config/` - Configuration loading (Viper)
- `internal/crypto/` - Token encryption at rest
- `internal/symbols/` - SCIP-based code navigation
- `internal/lock/` - Redis distributed locks
- `internal/metrics/` - Prometheus metrics
- `internal/tracing/` - OpenTelemetry spans

### Job System

The indexer processes jobs from Redis queue with these types:

**Job Types:**
1. **IndexJob**: Clone/fetch repo + run zoekt-git-index
2. **SyncJob**: Fetch repos from code host, queue IndexJobs
3. **ReplaceJob**: Execute find & replace, create MRs/PRs
4. **CleanupJob**: Remove Zoekt shards and repo files

**Job Flow Example:**
```
User syncs connection
  → Queue SyncJob
  → Worker lists repos from GitHub/GitLab
  → Upsert repos to database
  → Queue IndexJob for each repo
  → Worker clones repo
  → Worker runs zoekt-git-index
  → Zoekt loads new index shards
  → Repo searchable
```

**Distributed Locking:**
- Uses Redis locks to prevent concurrent operations on same repo
- Lock TTL: 30 minutes with automatic extension
- Required for all repo operations (clone, fetch, index)

### Horizontal Scaling

**Indexer Sharding:**
Set environment variables for multiple indexer instances:
```bash
TOTAL_SHARDS=3    # Total number of indexer instances
SHARD_INDEX=0     # This instance's shard (0-based)
```
Repos are distributed via consistent hashing on repo name. Only shard 0 runs recovery loops.

**Zoekt Sharding:**
Set in API server config:
```yaml
zoekt:
  url: http://zoekt-0:6070,http://zoekt-1:6070,http://zoekt-2:6070
```
Search queries are distributed across all shards and results merged.

## Configuration

### Environment Variables (Development)

The Makefile's `DEV_ENV` sets these defaults:
```bash
CS_DATABASE_URL=postgres://codesearch:codesearch@localhost:5432/codesearch?sslmode=disable
CS_REDIS_ADDR=localhost:6379
CS_ZOEKT_URL=http://localhost:6070
CS_REPOS_BASE_PATH=./data/repos
CS_INDEXER_INDEX_PATH=./data/index
CS_LOG_LEVEL=debug
```

### Configuration File

The system loads config from `config.yaml` with env var substitution:
```yaml
codehosts:
  my-github:
    type: github
    token: $GITHUB_TOKEN  # Reads from env var
    exclude_archived: true
    repos: ["org/repo1"]  # Empty = sync all
```

See `config.example.yaml` for all available options.

## Key Patterns and Gotchas

### Database Support
- **Primary**: PostgreSQL (recommended)
- **Alternative**: MySQL (supported via driver abstraction)
- Driver auto-detected from connection URL prefix (`postgres://` or `mysql://`)
- SQL builder in `internal/db/` abstracts placeholder syntax and upserts

### Token Security
- Connection tokens are encrypted at rest using `crypto.TokenEncryptor`
- Encryption key from config: `security.encryption_key`
- Tokens decrypted on-demand when fetching from database

### Read-Only Mode for Repos
- Repos can be marked as read-only (checkbox in UI)
- Read-only repos can be synced/indexed but not manually modified
- For replace operations on read-only repos, users must provide their own tokens

### Scheduler (runs in API server)
The scheduler is a single-instance component that:
- Checks for repos due for sync every 30s
- Recovers orphaned active markers every 10 minutes (repos stuck in `codesearch:active:index` Redis SET without actual jobs, typically caused by indexer crashes)
- Recovers stale indexing jobs (stuck > 1 hour)
- Re-queues pending jobs with no active worker
- Cleans up old completed jobs (7 day retention)
- Removes orphaned Zoekt index shards

**Important**: Only run one API server instance with `scheduler.enabled: true` to avoid duplicate job creation.

**Orphaned Job Recovery**: When the indexer crashes, repos may remain in the active index set without corresponding jobs. The scheduler automatically detects and clears these every 10 minutes, allowing jobs to be re-queued. This is tracked via `codesearch_orphaned_active_markers_total` metric.

### Graceful Shutdown
Both API server and indexer handle SIGTERM/SIGINT:
- API server: Waits up to 30s for active requests
- Indexer: Waits up to 5 minutes for current job to complete
- Jobs update heartbeat to prevent timeout during shutdown

### Testing Considerations
- Integration tests require PostgreSQL and Redis running
- Use `make dev-infra` to start dependencies
- Frontend tests use vitest with jsdom
- Mock implementations available in test files

### Replace Operations
Find & replace is a two-phase process:
1. **Preview**: Search for matches (no side effects)
2. **Execute**: Clone repos, create branches, apply changes, push, create MRs

For each repo during execute:
- Creates feature branch from default branch
- Commits changes with message from user
- Pushes to code host
- Creates merge request/pull request
- Returns MR URL or error

Concurrency controlled by `replace.concurrency` config (default: 3 parallel repos).

### File Structure
- `data/repos/` - Cloned repositories (git format)
- `data/index/` - Zoekt index shards
- `migrations/` - SQL migration files
- `web/` - Next.js frontend
- `website/` - Project documentation site
- `docs/` - Documentation markdown files

### Code Host Integration
Each code host client implements `codehost.Client` interface:
- `ValidateCredentials()` - Test connection/token
- `ListRepositories()` - Discover repos
- `CreateMergeRequest()` - For replace operations
- `GetCloneURL()` - Generate authenticated clone URL

Supported: GitHub (cloud + enterprise), GitLab (cloud + self-hosted), Gitea, Bitbucket.

### Metrics and Observability
- Prometheus metrics at `/metrics` endpoint on API server (:8080)
- Comprehensive indexing metrics: repo sizes, failure categorization (OOM, timeout, git errors), stuck jobs, orphaned temp files
- Production-ready Prometheus alerts in `prometheus-alerts.yaml` (critical and warning levels)
- OpenTelemetry tracing (optional, configure via config.yaml)
- Structured logging with Zap including repo context (repo_id, repo_name, repo_size_mb, failure_reason)
- Metrics include: job durations, search counts, git operations, active index repos, orphaned marker recovery

See `docs/observability.md` for complete metrics guide, dashboard queries, and troubleshooting runbooks.

## Development Tips

### Adding a New API Endpoint
1. Add route in `cmd/api/server/server.go`
2. Create handler in `cmd/api/handlers/`
3. Handler receives `Services` struct with all dependencies
4. Use dependency injection pattern (no globals)

### Adding a New Job Type
1. Define job type constant in `internal/queue/job.go`
2. Add job-specific payload struct
3. Implement processing logic in `internal/indexer/worker.go`
4. Queue jobs via `queue.Enqueue()` with payload

### Database Migrations
1. Create migration: `make migrate-add name=add_users_table`
2. Edit generated files in `migrations/`
3. Apply: `make migrate-up`
4. Rollback if needed: `make migrate-down`

### Frontend Development
- Uses Next.js App Router (React 19)
- CodeMirror for syntax highlighting
- UI components in `web/components/`
- API client in `web/lib/api.ts`
- State management via React hooks (no Redux)

### Adding a New Code Host
1. Implement `codehost.Client` interface
2. Add factory case in `codehost.NewClient()`
3. Update config schema to accept new type
4. Add integration tests

## Common Troubleshooting

### Repos Stuck in "Pending" Status
- **Symptom**: Repos remain pending for hours without indexing
- **Cause**: Orphaned active markers in Redis from indexer crash
- **Solution**: Scheduler auto-recovers these every 10 minutes. Check `codesearch:active:index` Redis SET
- **Manual fix**: `redis-cli SREM "codesearch:active:index" <repo_id>`

### OOM Kills During Indexing
- **Symptom**: Large repos fail to index, `.tmp` files left behind
- **Cause**: Repository size exceeds available indexer memory
- **Solution**: Increase indexer memory limit or set `max_repo_size_mb` to skip large repos
- **Detection**: Check `codesearch_index_failures_total{reason="oom_killed"}` metric

### Orphaned .tmp Files
- **Symptom**: Disk space consumed by `*.tmp` files in index directory
- **Cause**: Indexer crashed before completing index creation
- **Solution**: Safe to delete `.tmp` files manually
- **Prevention**: Increase indexer memory, set appropriate `index_timeout`

### Duplicate Search Results
- **Symptom**: Same file appears multiple times in results
- **Note**: Fixed in recent versions - deduplication now merges results across branches

## Storage Requirements

For capacity planning:
- **Repos** (`data/repos/`): ~10-20x original repo size (full git history)
- **Index** (`data/index/`): ~5-10x indexed source code size
- **Database**: Metadata only, typically < 1GB per 10k repos
- **Redis**: Job queue, typically < 100MB (ephemeral data)

## License

Apache License 2.0 - see LICENSE file for details.
