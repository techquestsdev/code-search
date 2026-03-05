# Code-Search: Known Issues & Improvements

## Priority Legend
- **P0**: Critical - blocking production
- **P1**: High - should fix soon
- **P2**: Medium - fix when possible
- **P3**: Low - nice to have

---

## Active Issues

*None currently*

---

## Bugs

*None currently - all known bugs fixed*

---

## Performance

*All performance issues addressed*

---

## Code Quality

*All code quality issues addressed*

---

## Security

### Tokens held in memory
- **Priority**: P3
- **Issue**: Connection tokens passed through multiple layers
- **Note**: Necessary for git operations, but could use secrets manager
- **Mitigated**: Tokens are now encrypted at rest in the database (see Completed section)

---

## Missing Features

### No webhook support for real-time updates
- **Priority**: P2
- **Issue**: Only polling-based updates, no push event handlers
- **Fix**: Add webhook handlers for GitHub/GitLab push events

---

## Configuration

*All configuration issues addressed*

---

## Completed

### Token encryption at rest
- **Fixed in**: (this session)
- **Files**: `internal/crypto/crypto.go`, `internal/repos/service.go`, `internal/config/config.go`
- **Issue**: Connection tokens were stored in plaintext in the database
- **Fix**: Added AES-256-GCM encryption for tokens:
  - New `crypto.TokenEncryptor` with `Encrypt()`, `Decrypt()`, `MustDecrypt()` methods
  - Encrypted values prefixed with `enc:` for detection
  - Backwards compatible: existing plaintext tokens continue to work
  - Enable via `CS_SECURITY_ENCRYPTION_KEY` env var or `security.encryption_key` config

### Errors silently ignored
- **Fixed in**: (this session)
- **Files**: `internal/indexer/worker.go`, `internal/scheduler/scheduler.go`, `cmd/api/handlers/*.go`
- **Issue**: Errors from best-effort operations were silently ignored with `_ = err`
- **Fix**: Added helper methods with debug logging for best-effort operations:
  - `updateProgress()` - logs progress update failures
  - `markIndexJobInactive()` - logs job marker failures
  - `updateIndexStatusBestEffort()` - logs status update failures

### Hardcoded default paths
- **Fixed in**: (this session)
- **Files**: `internal/config/config.go`, `internal/scheduler/scheduler.go`, `cmd/zoekt-refresh/main.go`
- **Issue**: Default paths like `/data/repos` and `/data/index` were hardcoded in various places
- **Fix**: Centralized path defaults in main config with env var bindings:
  - `CS_INDEXER_REPOS_PATH` / `indexer.repos_path`
  - `CS_INDEXER_INDEX_PATH` / `indexer.index_path`
  - `CS_ZOEKT_INDEX_PATH` / `zoekt.index_path`
  - `CS_REPOS_BASE_PATH` / `repos.base_path`
  - `CS_REPLACE_WORK_DIR` / `replace.work_dir`

### Missing database indexes
- **Fixed in**: (this session)
- **Files**: `migrations/postgres/20251101000006_add_composite_indexes.sql`, `migrations/mysql/20251101000006_add_composite_indexes.sql`
- **Issue**: Queries filter by excluded, index_status, connection_id without composite indexes
- **Fix**: Added composite indexes for common query patterns:
  - `idx_repos_pending_jobs` - For pending job queries (PostgreSQL partial index)
  - `idx_repos_status_excluded` - For status-based filtering
  - `idx_repos_stale_check` - For stale repo detection
  - `idx_repos_connection_status` - For connection-scoped queries

### Redis TLS configuration
- **Fixed in**: (this session)
- **Files**: `internal/queue/queue.go`, `internal/config/config.go`, `cmd/api/main.go`, `cmd/indexer/main.go`
- **Issue**: No TLS support for Redis connections
- **Fix**: Added TLSConfig struct and ConnectWithTLS function:
  - `CS_REDIS_TLS_ENABLED` - Enable TLS
  - `CS_REDIS_TLS_SKIP_VERIFY` - Skip certificate verification
  - `CS_REDIS_TLS_CERT_FILE` / `CS_REDIS_TLS_KEY_FILE` - Client certificate for mTLS
  - `CS_REDIS_TLS_CA_CERT_FILE` - Custom CA certificate
  - `CS_REDIS_TLS_SERVER_NAME` - Override server name for verification

### Rate limiting on API
- **Already implemented**
- **Files**: `internal/ratelimit/ratelimit.go`, `cmd/api/server/server.go`
- **Config**: `rate_limit.enabled`, `rate_limit.requests_per_second`, `rate_limit.burst_size`
- **Features**: Per-IP token bucket, X-RateLimit headers, cleanup goroutine

### Magic numbers hardcoded
- **Fixed in**: (this session)
- **File**: `internal/queue/queue.go`, `internal/queue/sharded_queue.go`
- **Issue**: Job TTL (24h) and claim TTL (5min) hardcoded throughout
- **Fix**: Extracted to named constants:
  - `DefaultJobTTL` = 24 hours
  - `DefaultJobClaimTTL` = 5 minutes

### Response body leak in pagination loops
- **Fixed in**: 38f3b29
- **File**: `internal/codehost/client.go`, `gitea.go`, `bitbucket.go`
- **Issue**: `defer resp.Body.Close()` inside loop causes file descriptor exhaustion
- **Fix**: Close body immediately after use, not with defer in loop

### Race condition: HasPendingJob then Enqueue
- **Fixed in**: b212031
- **Files**: `internal/queue/queue.go`, `internal/indexer/worker.go`, `internal/scheduler/scheduler.go`
- **Issue**: Check-then-enqueue is not atomic, can cause duplicate jobs
- **Fix**: Added atomic TryAcquire* functions using Redis SADD return value

### RecoveryLoop uses fmt.Printf instead of logger
- **Fixed in**: b212031
- **File**: `internal/queue/sharded_queue.go`
- **Fix**: Use structured zap logger via internal/log package

### ConvertPlaceholders limited to 20 parameters
- **Fixed in**: fe58f03
- **File**: `internal/db/driver.go`
- **Fix**: Use regex replacement for unlimited parameters

### Duplicate functions across files
- **Fixed in**: 8011b44
- **Files**: Created `internal/gitutil/gitutil.go` package
- **Fix**: Consolidated addAuthToURL, extractHost, sanitizeGitOutput into shared package

### Inconsistent GitHub token auth format
- **Fixed in**: 8011b44
- **Fix**: Standardized on `x-access-token:TOKEN@` format for GitHub

### default_branch not updated during sync
- **Fixed in**: d10655c
- **Issue**: PostgreSQL upsert had WHERE clause that prevented updating existing repos
- **Error**: `zoekt-git-index failed: getCommit("refs/heads/", "master"): reference not found`
- **Fix**: Remove WHERE clause from ON CONFLICT DO UPDATE in `internal/repos/service.go`

### Bare repo git fetch not updating branches
- **Fixed in**: a047ba7
- **Issue**: `git fetch --all` doesn't update refs in bare repos
- **Fix**: Use `git fetch origin +refs/heads/*:refs/heads/*`

### ReDoS vulnerability in regex patterns
- **Fixed in**: 89d2ac4
- **Files**: Created `internal/regexutil/validate.go`, updated `cmd/api/handlers/search.go`, `internal/replace/service.go`
- **Issue**: Malicious regex patterns could cause catastrophic backtracking (ReDoS attacks)
- **Fix**: Added regexutil package with pattern validation:
  - Max pattern length (1000 chars)
  - Max repetition count (100)
  - Max nested quantifiers (3 levels)
  - Integrated SafeCompile in search and replace handlers

### No retry mechanism for failed jobs
- **Fixed in**: 4297a0b
- **Files**: `internal/queue/queue.go`, `internal/queue/sharded_queue.go`
- **Issue**: Failed jobs were marked as permanently failed with no retry
- **Fix**: Added retry mechanism with exponential backoff:
  - Job struct now includes Attempts, MaxAttempts, NextRetryAt, LastError fields
  - Default 3 retry attempts with 30s base delay (exponential backoff up to 30min)
  - Retry queue using Redis sorted set for delayed job scheduling
  - Recovery loop now also processes retry queue every 10 seconds

### MySQL compatibility broken
- **Fixed in**: e1376a9
- **Files**: `internal/db/compat.go`, `internal/repos/service.go`, `internal/scheduler/scheduler.go`
- **Issue**: PostgreSQL-specific syntax used (FILTER, RETURNING, ::timestamptz)
- **Fix**: Added TimestampLiteral() and CountFilter() to SQLBuilder for database-agnostic queries

### Queue operations use SCAN (O(n))
- **Fixed in**: 4975ed3
- **File**: `internal/queue/queue.go`
- **Issue**: SCAN iterates all jobs, slow with large job counts
- **Fix**: Added sorted set indexes for O(log n) operations:
  - `codesearch:jobs:index` - all jobs sorted by creation time
  - `codesearch:jobs:status:{status}` - jobs by status
  - `codesearch:jobs:type:{type}` - jobs by type

### No HTTP connection pooling for code host clients
- **Fixed in**: bf814e3
- **Files**: `internal/codehost/client.go`, `gitea.go`, `bitbucket.go`
- **Issue**: Each client created its own http.Client without shared transport
- **Fix**: Added shared http.Transport with connection pool settings (MaxIdleConns=100, MaxConnsPerHost=20)

### No graceful shutdown coordination
- **Fixed in**: 6115f0d
- **Files**: `internal/indexer/worker.go`, `cmd/indexer/main.go`
- **Issue**: In-progress jobs interrupted on shutdown signal
- **Fix**: Workers now complete current job before stopping (up to 5 minute timeout)

### No health check endpoint
- **Already implemented**
- **Files**: `cmd/api/handlers/health.go`, `cmd/api/server/server.go`
- **Endpoints**:
  - `/health` - Liveness probe (simple ok response)
  - `/ready` - Readiness probe (checks DB, Redis, Zoekt with latencies)
