# Code Search Website Helm Chart

A Helm chart for deploying the Code Search documentation website to Kubernetes.

## Prerequisites

- Kubernetes 1.24+
- Helm 3.0+

## Installation

```bash
helm install code-search-website ./deploy/helm/code-search-website
```

### With Ingress

```bash
helm install code-search-website ./deploy/helm/code-search-website \
  --set ingress.enabled=true \
  --set ingress.host=docs.example.com
```

## Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of replicas | `1` |
| `image.repository` | Image repository | `ghcr.io/techquestsdev/code-search-website` |
| `image.tag` | Image tag (defaults to appVersion) | `""` |
| `service.type` | Service type | `ClusterIP` |
| `service.port` | Service port | `4321` |
| `ingress.enabled` | Enable ingress | `false` |
| `ingress.host` | Ingress hostname | `code-search-website.example.com` |
| `env` | Additional environment variables | `{}` |

See `values.yaml` for all available options.

## Uninstalling

```bash
helm uninstall code-search-website
```
