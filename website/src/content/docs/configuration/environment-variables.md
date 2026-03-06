---
title: Environment Variables
description: Complete reference of all environment variables
---

All Code Search configuration can be set via environment variables. This page provides a complete reference.

## Naming Convention

Environment variables follow this pattern:

- Prefix: `CS_`
- Nested keys: separated by `_`
- All uppercase

Example: `server.addr` → `CS_SERVER_ADDR`

## Complete Reference

### Server Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `CS_SERVER_ADDR` | Listen address (host:port) | `:8080` |
| `CS_SERVER_READ_TIMEOUT` | HTTP read timeout | `15s` |
| `CS_SERVER_WRITE_TIMEOUT` | HTTP write timeout | `60s` |

### Database Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `CS_DATABASE_DRIVER` | Database driver (`postgres` or `mysql`, auto-detected from URL) | auto-detect |
| `CS_DATABASE_URL` | Database connection string | - |
| `CS_DATABASE_MAX_OPEN_CONNS` | Maximum open connections | `25` |
| `CS_DATABASE_MAX_IDLE_CONNS` | Maximum idle connections | `5` |
| `CS_DATABASE_CONN_MAX_LIFETIME` | Connection max lifetime | `5m` |

### Redis Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `CS_REDIS_ADDR` | Redis address (host:port) | `localhost:6379` |
| `CS_REDIS_PASSWORD` | Redis password | - |
| `CS_REDIS_DB` | Redis database number | `0` |
| `CS_REDIS_TLS_ENABLED` | Enable TLS connection | `false` |
| `CS_REDIS_TLS_SKIP_VERIFY` | Skip TLS cert verification (insecure) | `false` |
| `CS_REDIS_TLS_CERT_FILE` | Client certificate file (mTLS) | - |
| `CS_REDIS_TLS_KEY_FILE` | Client key file (mTLS) | - |
| `CS_REDIS_TLS_CA_CERT_FILE` | CA certificate file | - |
| `CS_REDIS_TLS_SERVER_NAME` | Override server name for TLS | - |

### Zoekt Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `CS_ZOEKT_URL` | Zoekt server URL | `http://localhost:6070` |
| `CS_ZOEKT_INDEX_PATH` | Index directory path | - |
| `CS_ZOEKT_SHARDS` | Number of shards (0=auto) | `0` |

### Indexer Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `CS_INDEXER_CONCURRENCY` | Concurrent indexing jobs | `2` |
| `CS_INDEXER_INDEX_PATH` | Index directory path | - |
| `CS_INDEXER_REPOS_PATH` | Repositories directory path | - |
| `CS_INDEXER_REINDEX_INTERVAL` | Reindex interval | `1h` |
| `CS_INDEXER_ZOEKT_BIN` | Zoekt indexer binary | `zoekt-git-index` |
| `CS_INDEXER_CTAGS_BIN` | Path to universal-ctags binary | `ctags` |
| `CS_INDEXER_REQUIRE_CTAGS` | Fail indexing if ctags fails | `true` |

The following indexer options are available in `config.yaml` but do not have environment variable bindings:

- `indexer.index_timeout` — Timeout for zoekt-git-index (default: `0` = no timeout)
- `indexer.max_repo_size_mb` — Skip repos larger than this size in MB (default: `0` = no limit)
- `indexer.index_all_branches` — Index all branches, not just default (default: `false`)

### Repository Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `CS_REPOS_BASE_PATH` | Base path for repo clones | - |
| `CS_REPOS_READONLY` | Disable repo delete via UI | `false` |

### Scheduler Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `CS_SCHEDULER_ENABLED` | Enable auto-scheduling | `true` |
| `CS_SCHEDULER_POLL_INTERVAL` | Default sync interval | `6h` |
| `CS_SCHEDULER_CHECK_INTERVAL` | Check interval | `5m` |
| `CS_SCHEDULER_STALE_THRESHOLD` | Stale repo threshold | `24h` |
| `CS_SCHEDULER_MAX_CONCURRENT_CHECKS` | Parallel checks | `5` |
| `CS_SCHEDULER_JOB_RETENTION` | Job retention period | `1h` |

### Replace Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `CS_REPLACE_CONCURRENCY` | Parallel repo processing | `3` |
| `CS_REPLACE_CLONE_TIMEOUT` | Git clone timeout | `10m` |
| `CS_REPLACE_PUSH_TIMEOUT` | Git push timeout | `5m` |
| `CS_REPLACE_MAX_FILE_SIZE` | Max file size (bytes) | `10485760` |

### Rate Limiting

| Variable | Description | Default |
|----------|-------------|---------|
| `CS_RATE_LIMIT_ENABLED` | Enable rate limiting | `false` |
| `CS_RATE_LIMIT_REQUESTS_PER_SECOND` | Requests per second per IP | `10` |
| `CS_RATE_LIMIT_BURST_SIZE` | Maximum burst size | `20` |

### Metrics Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `CS_METRICS_ENABLED` | Enable Prometheus metrics | `true` |
| `CS_METRICS_PATH` | Metrics endpoint path | `/metrics` |

### Tracing Configuration (OpenTelemetry)

| Variable | Description | Default |
|----------|-------------|---------|
| `CS_TRACING_ENABLED` | Enable OpenTelemetry tracing | `false` |
| `CS_TRACING_SERVICE_NAME` | Service name for traces | `code-search` |
| `CS_TRACING_SERVICE_VERSION` | Service version | `1.0.0` |
| `CS_TRACING_ENVIRONMENT` | Deployment environment | `development` |
| `CS_TRACING_ENDPOINT` | OTLP endpoint | `localhost:4317` |
| `CS_TRACING_PROTOCOL` | Protocol (grpc/http) | `grpc` |
| `CS_TRACING_SAMPLE_RATE` | Sampling rate (0.0-1.0) | `1.0` |
| `CS_TRACING_INSECURE` | Disable TLS | `true` |

Also supports standard OpenTelemetry and Datadog environment variables:

- `OTEL_SERVICE_NAME`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_PROTOCOL`
- `DD_SERVICE`, `DD_VERSION`, `DD_ENV`, `DD_TRACE_ENABLED`

### Security Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `CS_SECURITY_ENCRYPTION_KEY` | Key for encrypting tokens at rest | - (disabled) |

When an encryption key is set, connection tokens (GitHub, GitLab, etc.) are encrypted using AES-256-GCM before being stored in the database. See [Security Configuration](/configuration/security/) for details.

### Sharding Configuration (Horizontal Scaling)

| Variable | Description | Default |
|----------|-------------|---------|
| `CS_SHARDING_ENABLED` | Enable hash-based sharding | `false` |
| `CS_SHARDING_TOTAL_SHARDS` | Number of indexer shards | `1` |
| `CS_SHARDING_INDEXER_API_PORT` | HTTP API port for federated access | `8081` |
| `CS_SHARDING_INDEXER_SERVICE` | Headless service for pod discovery | `code-search-indexer-headless` |
| `CS_SHARDING_FEDERATED_ACCESS` | Enable federated file browsing/replace | `false` |

### Search Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `CS_SEARCH_ENABLE_STREAMING` | Enable true streaming from Zoekt | `false` |

When streaming is enabled, search results are sent to the client as they arrive from Zoekt, rather than waiting for all results. This provides faster time-to-first-result, especially for large result sets.

:::note
Replace operations always use batch search regardless of this setting, as they need all results upfront to display the preview.
:::

### SCIP Code Intelligence

| Variable | Description | Default |
|----------|-------------|---------|
| `CS_SCIP_ENABLED` | Enable SCIP code intelligence indexing | `false` |
| `CS_SCIP_AUTO_INDEX` | Auto-index after Zoekt indexing | `true` |
| `CS_SCIP_TIMEOUT` | Aggregate timeout for SCIP indexing per repo | `10m` |
| `CS_SCIP_WORK_DIR` | Working directory for temporary checkouts | system temp |
| `CS_SCIP_CACHE_DIR` | Directory for SCIP SQLite databases | `<repos_path>/../scip` |

See [Indexer Configuration](/configuration/indexer/#scip-auto-indexing) for language tiers and per-language configuration.

### UI Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `CS_UI_HIDE_READONLY_BANNER` | Hide read-only mode banner | `false` |
| `CS_UI_HIDE_FILE_NAVIGATOR` | Hide browse links in search results | `false` |
| `CS_UI_DISABLE_BROWSE_API` | Disable browse API endpoints | `false` |
| `CS_UI_HIDE_REPOS_PAGE` | Hide the Repositories page from navigation | `false` |
| `CS_UI_HIDE_CONNECTIONS_PAGE` | Hide the Connections page from navigation | `false` |
| `CS_UI_HIDE_JOBS_PAGE` | Hide the Jobs page from navigation | `false` |
| `CS_UI_HIDE_REPLACE_PAGE` | Hide the Replace page from navigation | `false` |

### Connection Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `CS_CONNECTIONS_READONLY` | Lock connections to config | `false` |

### Config File Path

| Variable | Description | Default |
|----------|-------------|---------|
| `CS_CONFIG_FILE` | Path to config file | Auto-discovered |

## Enterprise Environment Variables

Enterprise features use the `CSE_` prefix. These require the enterprise API server binary and a valid license.

### License Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `CSE_LICENSE_KEY` | Signed license key string | - |
| `CSE_LICENSE_KEY_FILE` | Path to license key file on disk | - |
| `CSE_LICENSE_PUBLIC_KEY` | Base64-encoded Ed25519 public key for license verification | - |

### OIDC Authentication (SSO)

| Variable | Description | Default |
|----------|-------------|---------|
| `CSE_OIDC_ISSUER` | OIDC issuer URL (e.g., `https://login.mycompany.com`) | - |
| `CSE_OIDC_CLIENT_ID` | OIDC client ID | - |
| `CSE_OIDC_CLIENT_SECRET` | OIDC client secret | - |
| `CSE_OIDC_REDIRECT_URL` | OAuth2 redirect URL (e.g., `https://search.example.com/api/v1/auth/callback`) | - |
| `CSE_SESSION_SECRET` | HMAC secret for signing session JWTs (32+ chars recommended) | - |
| `CSE_SESSION_DURATION` | Session validity duration | `24h` |
| `CSE_COOKIE_SECURE` | Set Secure flag on session cookie (disable for local HTTP dev) | `true` |

### Audit Logging

| Variable | Description | Default |
|----------|-------------|---------|
| `CSE_AUDIT_ENABLED` | Enable audit logging | `true` |
| `CSE_AUDIT_RETENTION` | How long to keep audit events | `2160h` (90 days) |
| `CSE_AUDIT_BUFFER_SIZE` | In-memory event buffer size | `1000` |

### RBAC

RBAC roles and group mappings are configured via `config.yaml` or the admin API.

## Docker Compose Example

```yaml
services:
  api:
    image: ghcr.io/techquestsdev/code-search-api:latest
    environment:
      CS_DATABASE_URL: postgres://user:pass@postgres:5432/codesearch?sslmode=disable
      CS_REDIS_ADDR: redis:6379
      CS_ZOEKT_URL: http://zoekt:6070
      CS_SERVER_ADDR: ":8080"
      CS_SCHEDULER_ENABLED: "true"
      CS_SCHEDULER_POLL_INTERVAL: "6h"

  indexer:
    image: ghcr.io/techquestsdev/code-search-indexer:latest
    environment:
      CS_DATABASE_URL: postgres://user:pass@postgres:5432/codesearch?sslmode=disable
      CS_REDIS_ADDR: redis:6379
      CS_INDEXER_INDEX_PATH: /data/index
      CS_INDEXER_REPOS_PATH: /data/repos
      CS_INDEXER_CONCURRENCY: "2"
```

## Kubernetes ConfigMap/Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: code-search-secrets
type: Opaque
stringData:
  CS_DATABASE_URL: "postgres://user:pass@postgres:5432/codesearch"
  CS_REDIS_PASSWORD: ""
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: code-search-config
data:
  CS_SERVER_ADDR: ":8080"
  CS_REDIS_ADDR: "redis:6379"
  CS_ZOEKT_URL: "http://zoekt:6070"
  CS_INDEXER_CONCURRENCY: "2"
  CS_SCHEDULER_ENABLED: "true"
  CS_SCHEDULER_POLL_INTERVAL: "6h"
```

## Shell Script

```bash
#!/bin/bash

# Database and Redis
export CS_DATABASE_URL="postgres://user:pass@localhost:5432/codesearch?sslmode=disable"
export CS_REDIS_ADDR="localhost:6379"

# Server
export CS_SERVER_ADDR=":8080"
export CS_SERVER_READ_TIMEOUT="15s"
export CS_SERVER_WRITE_TIMEOUT="60s"

# Zoekt
export CS_ZOEKT_URL="http://localhost:6070"
export CS_INDEXER_INDEX_PATH="./data/index"
export CS_INDEXER_REPOS_PATH="./data/repos"

# Indexer
export CS_INDEXER_CONCURRENCY="2"
export CS_INDEXER_REINDEX_INTERVAL="1h"

# Scheduler
export CS_SCHEDULER_ENABLED="true"
export CS_SCHEDULER_POLL_INTERVAL="6h"
export CS_SCHEDULER_CHECK_INTERVAL="5m"
export CS_SCHEDULER_STALE_THRESHOLD="24h"

# Readonly modes
export CS_REPOS_READONLY="false"
export CS_CONNECTIONS_READONLY="false"

# Start the API server
./bin/api-server
```

## Duration Format

Duration values use Go's duration format:

| Suffix | Meaning | Example |
|--------|---------|---------|
| `s` | Seconds | `30s` |
| `m` | Minutes | `10m` |
| `h` | Hours | `6h` |

Combine for complex durations: `1h30m`, `2h45m30s`

## Boolean Values

Boolean environment variables accept:

- **True:** `true`, `1`, `yes`, `on`
- **False:** `false`, `0`, `no`, `off`
