---
title: Indexer Configuration
description: Configure the background indexer worker
---

The `indexer` section configures the background indexer worker that clones and indexes repositories.

## Configuration

```yaml
indexer:
  concurrency: 2
  index_path: "./data/index"
  repos_path: "./data/repos"
  reindex_interval: 1h
  zoekt_bin: "zoekt-git-index"
  ctags_bin: "ctags"
  require_ctags: true
  # index_all_branches: false
  # index_timeout: 0
  # max_repo_size_mb: 0
```

## Options

### `concurrency`

Number of concurrent indexing jobs.

| Property | Value |
|----------|-------|
| Type | `integer` |
| Default | `2` |
| Environment | `CS_INDEXER_CONCURRENCY` |

Higher concurrency means faster indexing but more resource usage (CPU, memory, disk I/O).

**Recommendations:**

- Small deployments (< 100 repos): `1-2`
- Medium deployments (100-1000 repos): `2-4`
- Large deployments (> 1000 repos): `4-8`

### `index_path`

Path to the directory where Zoekt search index shards are stored.

| Property | Value |
|----------|-------|
| Type | `string` |
| Default | `"./data/index"` |
| Environment | `CS_INDEXER_INDEX_PATH` |

### `repos_path`

Path to the directory where cloned repositories are stored.

| Property | Value |
|----------|-------|
| Type | `string` |
| Default | `"./data/repos"` |
| Environment | `CS_INDEXER_REPOS_PATH` |

### `reindex_interval`

How often the indexer re-indexes repositories. Works in conjunction with the scheduler.

| Property | Value |
|----------|-------|
| Type | `duration` |
| Default | `1h` |
| Environment | `CS_INDEXER_REINDEX_INTERVAL` |

### `zoekt_bin`

Path to the Zoekt indexer binary.

| Property | Value |
|----------|-------|
| Type | `string` |
| Default | `"zoekt-git-index"` |
| Environment | `CS_INDEXER_ZOEKT_BIN` |

### `ctags_bin`

Path to the universal-ctags binary for symbol indexing.

| Property | Value |
|----------|-------|
| Type | `string` |
| Default | `"ctags"` |
| Environment | `CS_INDEXER_CTAGS_BIN` |

### `require_ctags`

If true, ctags failures will fail the indexing job. If false, indexing continues without symbol data.

| Property | Value |
|----------|-------|
| Type | `boolean` |
| Default | `true` |
| Environment | `CS_INDEXER_REQUIRE_CTAGS` |

### `index_all_branches`

When true, index all branches (not just the default branch). Increases storage and index time.

| Property | Value |
|----------|-------|
| Type | `boolean` |
| Default | `false` |
| Environment | YAML only |

### `index_timeout`

Timeout for `zoekt-git-index` operations. Set to `0` for no timeout (infinite).

| Property | Value |
|----------|-------|
| Type | `duration` |
| Default | `0` (no timeout) |
| Environment | YAML only |

For large repos (millions of lines), indexing can take hours. The default of `0` allows unlimited time.

**Examples:**

- `0` - No timeout (default)
- `"30m"` - 30 minutes
- `"2h"` - 2 hours (for large monorepos)

### `max_repo_size_mb`

Skip indexing repositories larger than this size in MB. Set to `0` for no limit.

| Property | Value |
|----------|-------|
| Type | `integer` |
| Default | `0` (no limit) |
| Environment | YAML only |

Useful to avoid indexing extremely large monorepos that would consume too much memory or time.

**Example:** `10000` (skip repos larger than 10 GB)

## Environment Variables

```bash
CS_INDEXER_CONCURRENCY="2"
CS_INDEXER_INDEX_PATH="./data/index"
CS_INDEXER_REPOS_PATH="./data/repos"
CS_INDEXER_REINDEX_INTERVAL="1h"
CS_INDEXER_ZOEKT_BIN="zoekt-git-index"
CS_INDEXER_CTAGS_BIN="ctags"
CS_INDEXER_REQUIRE_CTAGS="true"
```

Note: `index_timeout`, `max_repo_size_mb`, and `index_all_branches` are configurable via `config.yaml` only (no environment variable bindings).

## How the Indexer Works

1. **Poll Queue** - Indexer polls Redis for pending jobs
2. **Clone Repository** - Git clone/fetch the repository
3. **Run Zoekt Index** - Create search index
4. **Update Database** - Mark repository as indexed
5. **Notify** - Signal Zoekt to reload indexes

### Job Types

| Type | Description |
|------|-------------|
| `index` | Initial indexing of a new repository |
| `sync` | Re-sync an existing repository (fetch + re-index) |
| `replace` | Execute a search-and-replace operation |

## Scaling Indexers

There are three approaches to scale indexing:

### 1. Multiple Workers with Shared Storage

Run multiple indexer instances sharing the same storage:

```bash
# Docker Compose
docker compose up -d --scale indexer=4

# Kubernetes (requires ReadWriteMany PVC)
kubectl scale deployment code-search-indexer --replicas=4
```

Each indexer processes jobs independently from the Redis queue. All workers share the same index and repos directories.

**Requirements:** ReadWriteMany (RWX) storage class (NFS, CephFS, EFS, Azure Files)

### 2. Hash-Based Sharding

For very large deployments or when RWX storage isn't available, use hash-based sharding:

```yaml
sharding:
  enabled: true
  total_shards: 3
  federated_access: true
```

Each shard:

- Has its own PersistentVolume (ReadWriteOnce)
- Processes only repositories assigned to it via consistent hashing
- Runs its own Zoekt instance

See [Sharding Configuration](/configuration/sharding/) for details.

### 3. Single Indexer (Default)

For smaller deployments (< 1000 repos), a single indexer handles everything:

- Simpler to operate
- No shared storage requirements
- Scale vertically with more CPU/memory

## Resource Requirements

### CPU

Indexing is CPU-intensive. Each concurrent job uses approximately 1 CPU core.

### Memory

Memory usage depends on repository size:

- Small repos (< 100 MB): ~512 MB per job
- Medium repos (100 MB - 1 GB): ~1 GB per job
- Large repos (> 1 GB): ~2-4 GB per job

### Disk I/O

Indexing is disk I/O intensive. Use SSDs for best performance.

### Network

Initial clones download the full repository. Subsequent syncs only fetch changes.

## Git Configuration

The indexer uses these Git settings:

```bash
# Shallow clone for initial index (faster)
git clone --depth 1 --single-branch

# Full history for sync operations
git fetch --all
```

### Authentication

Git authentication is handled via the connection's access token. The token is used for HTTPS cloning:

```
https://oauth2:{token}@github.com/org/repo.git
```

## Branch Support

By default, only the **default branch** (usually `main` or `master`) is indexed. Zoekt supports indexing multiple branches using the `-branches` flag.

### How Branch Indexing Works

When a repository is indexed, the indexer runs:

```bash
zoekt-git-index -index /data/index -branches main,develop /data/repos/myrepo
```

This creates searchable indexes for each specified branch.

### Searching Branches

Use the `branch:` filter to search specific branches:

```
FOO branch:develop
```

Omitting the `branch:` filter searches the default branch (`HEAD`).

### Current Limitations

- Tags are not currently indexed (only branches)
- Multi-branch indexing requires manual configuration per repository
- The default behavior indexes only the default branch for efficiency

### Future Enhancements

Planned improvements include:

- Per-repository branch configuration
- Tag indexing support
- Automatic branch discovery based on patterns

## SCIP Auto-Indexing

The indexer can automatically run SCIP code intelligence indexing after Zoekt search indexing completes. This provides precise go-to-definition and find-references without manual API calls.

### Configuration

```yaml
scip:
  enabled: false        # Master switch
  auto_index: true      # Auto-index after Zoekt indexing
  timeout: 10m          # Timeout per SCIP indexing operation
  # work_dir: ""        # Temp directory for checkouts (default: system temp)
  # cache_dir: ""       # SCIP database directory (default: <repos_path>/../scip)
```

| Option | Type | Default | Environment | Description |
|--------|------|---------|-------------|-------------|
| `enabled` | `boolean` | `false` | `CS_SCIP_ENABLED` | Enable SCIP indexing |
| `auto_index` | `boolean` | `true` | `CS_SCIP_AUTO_INDEX` | Auto-index after Zoekt indexing |
| `timeout` | `duration` | `10m` | `CS_SCIP_TIMEOUT` | Aggregate timeout for all SCIP indexing per repo (across all languages) |
| `work_dir` | `string` | `""` | `CS_SCIP_WORK_DIR` | Working directory for temporary checkouts |
| `cache_dir` | `string` | `""` | `CS_SCIP_CACHE_DIR` | Directory for SCIP SQLite databases |

### Language Tiers

Languages are grouped into two tiers:

**Standalone** (auto-enabled when `scip.enabled=true` and binary is in PATH):
- **Go** (`scip-go`)
- **TypeScript/JavaScript** (`scip-typescript` via npx)
- **Python** (`scip-python`)

**Build-dependent** (require explicit opt-in):
- **Java** (`scip-java`)
- **Rust** (`rust-analyzer`)
- **PHP** (`scip-php` — per-project Composer dependency)

### Per-Language Configuration

Override defaults for specific languages:

```yaml
scip:
  enabled: true
  languages:
    go:
      enabled: true                              # Standalone: auto-enabled
    java:
      enabled: true                              # Build-dependent: explicit opt-in
      binary_path: "/usr/local/bin/scip-java"    # Optional: custom binary path
    rust:
      enabled: false                             # Disabled (default for build-dependent)
```

### Docker Setup

The default indexer image does not include SCIP binaries. Use the SCIP-enabled image:

```yaml
# docker-compose.yml
indexer:
  build:
    dockerfile: docker/indexer-scip.Dockerfile
  environment:
    CS_SCIP_ENABLED: "true"
```

The `indexer-scip.Dockerfile` includes `scip-go`, `scip-typescript` (via Node.js/npx), and `scip-python`.

### How It Works

1. After Zoekt search indexing completes successfully, the indexer checks if SCIP is enabled
2. It detects **all** languages present in the repository from marker files (`go.mod`, `package.json`, `Cargo.toml`, etc.) using `git ls-tree` — no full checkout needed at this stage. For monorepos, marker files in subdirectories (up to 3 levels deep) are also detected.
3. For each detected language that is enabled and has an available indexer binary, it discovers project directories and runs SCIP indexing. Results from all languages are merged into a single index per repository.
4. The SCIP index is stored in a per-repo SQLite database with file paths correctly mapped to the full repository path

SCIP indexing failure is **non-fatal** — it never fails the Zoekt index job. Errors are logged as warnings.

### Multi-Language Monorepos

Repositories containing multiple languages (e.g., a Go backend with a TypeScript frontend) are fully supported. The indexer detects and indexes all languages, merging results into a single SCIP index.

Project discovery behavior depends on the language:

- **Independent projects** (Go, PHP): Each marker file (e.g., `go.mod`) represents a separate project. All directories with markers are discovered and indexed independently, even if the root also has one. This is correct for Go modules, where nested `go.mod` files are distinct modules.
- **Workspace-aware** (TypeScript, JavaScript, Java, Rust, Python): A root-level marker means the tool handles the entire tree (e.g., TypeScript workspaces, Cargo workspaces, Maven multi-module). Subdirectories are only searched when no root marker exists.

### Timeout Behavior

The `scip.timeout` config sets an **aggregate** timeout for all SCIP indexing within a single repository — across all detected languages and all project directories. Each individual indexer invocation also has its own per-execution timeout (`scip.indexer_timeout` internally), but the outer aggregate timeout caps the total time spent on SCIP indexing for one repo.

## Troubleshooting

### Clone timeout (replace operations)

```
job failed: clone timeout after 10m
```

Clone timeouts during replace operations can be adjusted via the replace config:

```yaml
replace:
  clone_timeout: "30m"
```

### Index timeout

```
job failed: index timeout
```

Increase `index_timeout` for very large repositories:

```yaml
indexer:
  index_timeout: "2h"
```

### Out of memory

If the indexer is killed by OOM:

- Reduce `concurrency`
- Increase container/pod memory limits
- Exclude very large repositories

### Disk full

The indexer needs space for:

- Git clones (repos_dir)
- Zoekt indexes (index_dir)
- Temporary files during indexing

Monitor disk usage and increase storage as needed.

### Jobs stuck in "running"

If jobs are stuck:

1. Check indexer logs for errors
2. Restart the indexer: `docker compose restart indexer`
3. Failed jobs will be retried
