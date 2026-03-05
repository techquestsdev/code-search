---
title: Sharding Configuration
description: Configure horizontal scaling with hash-based sharding
---

The `sharding` section configures horizontal scaling for large deployments with thousands of repositories.

## Deployment Modes

Code Search supports three deployment modes for scaling:

### 1. Single Indexer (Default)

The simplest deployment with one indexer and its Zoekt sidecar:

- **Storage**: Single PersistentVolume (ReadWriteOnce)
- **Features**: Full search, file browsing, replace jobs (with shared storage to API)
- **Scale**: Handles thousands of repositories
- **Best for**: Most deployments

```yaml
# Default - no sharding config needed
sharding:
  enabled: false
```

### 2. Shared Storage (RWX)

Multiple indexer workers sharing the same PersistentVolume:

- **Storage**: ReadWriteMany (NFS, CephFS, EFS, Azure Files)
- **Features**: All features work - search, file browsing, replace jobs
- **Scale**: Parallel indexing with queue-based work distribution
- **Best for**: Faster indexing without complex configuration

```yaml
# In Kubernetes/Helm
indexer:
  replicaCount: 4  # Multiple workers
persistence:
  accessMode: ReadWriteMany  # Shared storage

# No sharding config needed
sharding:
  enabled: false
```

### 3. Hash-Based Sharding with Federated Access

Each shard handles a subset of repositories using consistent FNV hashing:

- **Storage**: Each shard has its own PersistentVolume (ReadWriteOnce)
- **Features**: Search via parallel Zoekt queries, file browsing/replace via federated proxy
- **Scale**: Extreme horizontal scaling without shared storage
- **Best for**: Very large deployments, cloud environments without good RWX options

```yaml
sharding:
  enabled: true
  total_shards: 3
  indexer_api_port: 8081
  indexer_service: "code-search-indexer-headless"
  federated_access: true
```

## Configuration Options

### `enabled`

Enable hash-based sharding mode.

| Property | Value |
|----------|-------|
| Type | `boolean` |
| Default | `false` |
| Environment | `CS_SHARDING_ENABLED` |

When enabled:

- Repositories are distributed across shards using consistent FNV hashing
- Each indexer only processes repositories assigned to its shard
- Search queries all Zoekt instances in parallel

### `total_shards`

Total number of indexer shards. Must match the number of indexer replicas.

| Property | Value |
|----------|-------|
| Type | `integer` |
| Default | `1` |
| Environment | `CS_SHARDING_TOTAL_SHARDS` |

The shard index is determined from the pod's ordinal number (e.g., `indexer-0` is shard 0).

### `indexer_api_port`

HTTP API port exposed by each indexer for federated file/replace access.

| Property | Value |
|----------|-------|
| Type | `integer` |
| Default | `8081` |
| Environment | `CS_SHARDING_INDEXER_API_PORT` |

This port is used internally for API-to-indexer communication. The endpoints are:

- `GET /files/{repo}/tree` - List directory contents
- `GET /files/{repo}/blob` - Get file contents
- `GET /files/{repo}/exists` - Check if repo exists on shard
- `POST /replace/execute` - Execute replace job on shard

### `indexer_service`

Headless Kubernetes service name for pod discovery.

| Property | Value |
|----------|-------|
| Type | `string` |
| Default | `"code-search-indexer-headless"` |
| Environment | `CS_SHARDING_INDEXER_SERVICE` |

The API uses this service to construct pod DNS names for routing requests:

```
{indexer_service}.{namespace}.svc.cluster.local
```

For shard N:

```
code-search-indexer-N.{indexer_service}.{namespace}.svc.cluster.local
```

### `federated_access`

Enable federated file browsing and replace jobs via proxy.

| Property | Value |
|----------|-------|
| Type | `boolean` |
| Default | `false` |
| Environment | `CS_SHARDING_FEDERATED_ACCESS` |

When enabled:

- File browser requests are proxied to the indexer that owns the repository
- Replace jobs are fanned out to all shards and results are merged
- Each indexer runs an HTTP API on `indexer_api_port`

## Environment Variables

```bash
CS_SHARDING_ENABLED="true"
CS_SHARDING_TOTAL_SHARDS="3"
CS_SHARDING_INDEXER_API_PORT="8081"
CS_SHARDING_INDEXER_SERVICE="code-search-indexer-headless"
CS_SHARDING_FEDERATED_ACCESS="true"
```

## How Sharding Works

### Repository Distribution

Repositories are assigned to shards using consistent FNV hashing:

```
shard = FNV32(repoName) % totalShards
```

This ensures:

- Same repository always goes to the same shard
- Even distribution across shards
- No coordination needed between shards

### Search Flow

1. API receives search query
2. Query is sent to all Zoekt instances in parallel
3. Results are merged and returned to the client

### File Browsing Flow (Federated)

1. API receives file browse request for repository X
2. API calculates: `shard = hash(X) % totalShards`
3. API proxies request to `indexer-{shard}` via headless service
4. Indexer returns file contents from its local storage

### Replace Job Flow (Federated)

1. API receives replace request with matches from multiple repos
2. API groups matches by shard (based on repository hash)
3. API sends execute request to each shard in parallel
4. Each shard processes its assigned repositories
5. API merges results and returns to client

## Helm Chart Configuration

For Kubernetes deployments, use the Helm chart's sharding configuration:

```yaml
# values.yaml
sharding:
  enabled: true
  replicas: 3
  federatedAccess:
    enabled: true
    indexerAPIPort: 8081
```

This automatically:

- Creates a StatefulSet with the specified replicas
- Configures the headless service for pod discovery
- Sets up environment variables for sharding
- Exposes the indexer API port

## Recommendations

| Scenario | Mode | Configuration |
|----------|------|---------------|
| < 1000 repos | Single | Default (no sharding) |
| 1000-5000 repos, fast indexing | Shared Storage | RWX + multiple replicas |
| > 5000 repos, no RWX available | Sharding | Hash-based with federated access |
| Cloud with limited storage options | Sharding | Hash-based with federated access |

## Migrating Between Modes

### Single → Shared Storage

1. Change storage class to RWX
2. Increase `indexer.replicaCount`
3. No data migration needed

### Shared Storage → Sharding

1. Enable sharding with `total_shards` matching desired replicas
2. Each shard will re-index its assigned repositories
3. Consider running re-index during off-peak hours

### Sharding → Single/Shared

1. Disable sharding
2. Scale to single replica
3. Re-index all repositories to single storage

## Next Steps

- [Indexer Configuration](/configuration/indexer/) - Configure indexer workers
- [Helm Deployment](/deployment/helm/) - Deploy with Helm
- [Kubernetes Deployment](/deployment/kubernetes/) - Deploy with manifests
