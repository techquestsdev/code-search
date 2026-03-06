# Code Search Helm Chart

A Helm chart for deploying Code Search to Kubernetes.

## Prerequisites

- Kubernetes 1.24+
- Helm 3.0+
- PV provisioner (for persistence)
- External PostgreSQL and Redis (connection details provided via secrets)

## Installation

### Quick Start

```bash
# Create secrets for database and Redis
kubectl create secret generic code-search-postgres-url \
  --from-literal=url='postgres://user:pass@postgres:5432/codesearch?sslmode=disable'
kubectl create secret generic code-search-redis-password \
  --from-literal=password='redis-password'

# Install with default values
helm install code-search ./deploy/helm/code-search
```

### With Ingress

```bash
helm install code-search ./deploy/helm/code-search \
  --set ingress.enabled=true \
  --set ingress.host=code-search.example.com
```

## Configuration

### Key Values

| Parameter | Description | Default |
|-----------|-------------|---------|
| `api.replicaCount` | Number of API replicas | `1` |
| `web.replicaCount` | Number of web replicas | `1` |
| `indexer.replicaCount` | Number of indexer workers | `1` |
| `ingress.enabled` | Enable ingress | `false` |
| `ingress.host` | Ingress hostname | `code-search.example.com` |
| `sharding.enabled` | Enable hash-based sharding | `false` |
| `sharding.replicas` | Number of indexer+zoekt shard pairs | `3` |

### Database and Redis

PostgreSQL and Redis are configured via existing Kubernetes secrets:

| Parameter | Description | Default |
|-----------|-------------|---------|
| `postgresql.host` | PostgreSQL host | `code-search-postgres` |
| `postgresql.port` | PostgreSQL port | `5432` |
| `postgresql.existingSecret` | Secret containing database URL | `code-search-postgres-url` |
| `postgresql.existingSecretKey` | Key in the secret | `url` |
| `redis.host` | Redis host | `code-search-redis` |
| `redis.port` | Redis port | `6379` |
| `redis.existingSecret` | Secret containing Redis password | `code-search-redis-password` |
| `redis.existingSecretKey` | Key in the secret | `password` |

### Persistence

| Parameter | Description | Default |
|-----------|-------------|---------|
| `indexer.persistence.enabled` | Enable persistence for repos and index | `true` |
| `indexer.persistence.size` | Storage size | `50Gi` |
| `indexer.persistence.accessMode` | PVC access mode | `ReadWriteOnce` |

Zoekt runs as a sidecar in the indexer pod and shares its volume — no separate persistence config needed.

### Code Hosts

Configure code host tokens via secrets:

```yaml
codehosts:
  github:
    existingSecret: github-token
    existingSecretKey: token
    envVar: CS_GITHUB_TOKEN
```

### Application Config

Provide a `config.yaml` via ConfigMap:

```yaml
config:
  existingConfigMap: my-config  # Use existing ConfigMap
  # Or inline:
  inline: |
    codehosts:
      github:
        type: github
        token: "$CS_GITHUB_TOKEN"
```

### Resources

See `values.yaml` for default resource requests/limits.

## Sharding

For large deployments (1000+ repositories), enable hash-based sharding:

```yaml
sharding:
  enabled: true
  replicas: 3  # Creates indexer-0, indexer-1, indexer-2 (each with zoekt sidecar)
  federatedAccess:
    enabled: true  # Enable file browsing and replace via proxy
```

Each shard handles a subset of repositories using consistent hashing.

For simpler parallelism without sharding, increase `indexer.replicaCount` with RWX storage:

```yaml
indexer:
  replicaCount: 3
  persistence:
    accessMode: ReadWriteMany
```

## Upgrading

```bash
helm upgrade code-search ./deploy/helm/code-search
```

Database migrations run automatically as a pre-upgrade hook (when `migrations.enabled: true`).

## Uninstalling

```bash
helm uninstall code-search
```

**Note:** PVCs are not deleted automatically. To delete all data:

```bash
kubectl delete pvc -l app.kubernetes.io/instance=code-search
```

## Troubleshooting

### Check pod status

```bash
kubectl get pods -l app.kubernetes.io/instance=code-search
```

### View logs

```bash
# API logs
kubectl logs -l app.kubernetes.io/component=api

# Indexer logs
kubectl logs -l app.kubernetes.io/component=indexer

# Zoekt logs
kubectl logs -l app.kubernetes.io/component=zoekt
```

### Migration issues

```bash
# Check migration job
kubectl logs job/code-search-migrate

# Re-run migrations
kubectl delete job code-search-migrate
helm upgrade code-search ./deploy/helm/code-search
```
