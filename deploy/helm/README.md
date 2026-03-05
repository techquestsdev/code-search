# Code Search Helm Chart

A Helm chart for deploying Code Search to Kubernetes.

## Prerequisites

- Kubernetes 1.24+
- Helm 3.0+
- PV provisioner (for persistence)

## Installation

### Quick Start

```bash
# Add dependencies (PostgreSQL, Redis)
helm repo add bitnami https://charts.bitnami.com/bitnami

# Install with default values
helm install code-search ./deploy/helm/code-search
```

### Production Deployment

```bash
# Install with production values
helm install code-search ./deploy/helm/code-search \
  -f ./deploy/helm/code-search/values-production.yaml \
  --namespace code-search \
  --create-namespace
```

### Using External Database

```bash
helm install code-search ./deploy/helm/code-search \
  --set postgresql.enabled=false \
  --set postgresql.external.host=my-postgres.example.com \
  --set postgresql.external.username=codesearch \
  --set postgresql.external.password=secret \
  --set postgresql.external.database=codesearch
```

## Configuration

### Key Values

| Parameter | Description | Default |
|-----------|-------------|---------|
| `api.replicaCount` | Number of API replicas | `1` |
| `web.replicaCount` | Number of web replicas | `1` |
| `sharding.enabled` | Enable sharding for large deployments | `false` |
| `sharding.zoekt.replicas` | Number of Zoekt shards | `3` |
| `sharding.indexer.replicas` | Number of indexer workers | `3` |
| `postgresql.enabled` | Deploy PostgreSQL | `true` |
| `redis.enabled` | Deploy Redis | `true` |
| `ingress.enabled` | Enable ingress | `false` |

### Persistence

| Parameter | Description | Default |
|-----------|-------------|---------|
| `indexer.persistence.enabled` | Enable persistence for repos | `true` |
| `indexer.persistence.size` | Storage size | `50Gi` |
| `zoekt.persistence.enabled` | Enable persistence for index | `true` |
| `zoekt.persistence.size` | Storage size | `100Gi` |

### Resources

See `values.yaml` for default resource requests/limits. For production, use `values-production.yaml` as a starting point.

## Sharding

For large deployments (1000+ repositories), enable sharding:

```yaml
sharding:
  enabled: true
  zoekt:
    replicas: 3  # Number of search shards
  indexer:
    replicas: 3  # Number of indexer workers
```

Each shard handles a subset of repositories using consistent hashing.

## Upgrading

```bash
helm upgrade code-search ./deploy/helm/code-search \
  -f values-production.yaml
```

Database migrations run automatically as a pre-upgrade hook.

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
