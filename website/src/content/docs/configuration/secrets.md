---
title: Secrets Management
description: How to securely manage secrets in Code Search using file-based secrets or environment variables
---

Code Search supports loading secrets from files, which is the recommended approach for Kubernetes and Docker deployments. This keeps sensitive data like API tokens and database passwords out of configuration files and environment variable definitions.

## How It Works

Code Search can load secrets from files in designated directories. Each file becomes an environment variable:

- **File name** → Environment variable name
- **File content** → Environment variable value

For example:

```
/etc/secrets/GITHUB_TOKEN  (contains "ghp_xxxx")
→ Sets GITHUB_TOKEN=ghp_xxxx
```

## Default Secret Paths

By default, Code Search checks these directories for secret files:

| Path | Description |
|------|-------------|
| `/etc/secrets` | Common Kubernetes secrets mount point |
| `/run/secrets` | Docker secrets location |

## Configuring Secret Paths

Use the `CS_SECRETS_PATH` environment variable to specify custom paths:

```bash
# Single path
export CS_SECRETS_PATH="/custom/secrets"

# Multiple paths (comma or colon separated)
export CS_SECRETS_PATH="/path1,/path2"
export CS_SECRETS_PATH="/path1:/path2"
```

## Kubernetes Secrets

The recommended way to deploy Code Search on Kubernetes is using the [Helm chart](/deployment/helm/), which has built-in support for secrets.

### Using the Helm Chart

Create a Kubernetes secret with your sensitive values:

```bash
kubectl create secret generic code-search-secrets \
  --namespace code-search \
  --from-literal=GITHUB_TOKEN="ghp_your_token_here" \
  --from-literal=GITLAB_TOKEN="glpat_your_token_here" \
  --from-literal=CS_DATABASE_URL="postgres://user:password@postgres:5432/codesearch"
```

Then reference it in your Helm values:

```yaml
# values.yaml
existingSecret: code-search-secrets

config:
  codehosts:
    github:
      type: github
      token: "$GITHUB_TOKEN"
      exclude_archived: true
    gitlab:
      type: gitlab
      url: "https://gitlab.com"
      token: "$GITLAB_TOKEN"
```

Install with:

```bash
helm install code-search code-search/code-search \
  --namespace code-search \
  -f values.yaml
```

The Helm chart mounts the secret at `/etc/secrets`, and Code Search automatically loads the values as environment variables.

See the [Helm Chart documentation](/deployment/helm/) for more configuration options.

## Docker Secrets

Docker Swarm secrets are mounted at `/run/secrets`:

```yaml
version: "3.8"
services:
  api:
    image: ghcr.io/techquestsdev/code-search-api:latest
    secrets:
      - github_token
      - gitlab_token

secrets:
  github_token:
    external: true
  gitlab_token:
    external: true
```

Create secrets with:

```bash
echo "ghp_your_token" | docker secret create github_token -
echo "glpat_your_token" | docker secret create gitlab_token -
```

## Using Secrets in Configuration

Reference secrets in your `config.yaml` using environment variable syntax:

```yaml
codehosts:
  github:
    type: github
    token: "$GITHUB_TOKEN"  # Loaded from /etc/secrets/GITHUB_TOKEN
    exclude_archived: true

  gitlab:
    type: gitlab
    url: "https://gitlab.com"
    token: "$GITLAB_TOKEN"  # Loaded from /etc/secrets/GITLAB_TOKEN
    exclude_archived: true
```

The `$VAR_NAME` syntax tells Code Search to look up the value from environment variables, which includes file-loaded secrets.

## Precedence

When the same secret is defined in multiple places:

1. **Explicit environment variables** (highest priority)
2. **File-loaded secrets**
3. **Default values** (lowest priority)

This means you can override file-loaded secrets with explicit environment variables if needed.

## Common Secrets

| Secret Name | Description | Used By |
|-------------|-------------|---------|
| `CS_DATABASE_URL` | Full PostgreSQL connection string | Database |
| `DATABASE_PASSWORD` | Database password only | Database |
| `GITHUB_TOKEN` | GitHub personal access token | GitHub code host |
| `GITLAB_TOKEN` | GitLab personal access token | GitLab code host |
| `CS_REDIS_PASSWORD` | Redis password | Redis |
| `CS_SECURITY_ENCRYPTION_KEY` | Token encryption key | Security |

## Token Encryption

In addition to loading secrets from files, Code Search can encrypt connection tokens at rest in the database. This provides an extra layer of security if your database is compromised.

To enable token encryption, set the `CS_SECURITY_ENCRYPTION_KEY` secret:

```bash
kubectl create secret generic code-search-secrets \
  --namespace code-search \
  --from-literal=CS_SECURITY_ENCRYPTION_KEY="$(openssl rand -base64 32)" \
  --from-literal=GITHUB_TOKEN="ghp_your_token_here"
```

See [Security Configuration](/configuration/security/) for more details on token encryption.

## Best Practices

1. **Never commit secrets** to version control
2. **Use file-based secrets** in production (Kubernetes/Docker)
3. **Use environment variables** for local development
4. **Rotate tokens regularly** and update secrets
5. **Use separate tokens** for different code hosts
6. **Limit token permissions** to read-only when possible

## Troubleshooting

### Secrets not loading

Check that:

- The secrets directory exists and is readable
- File names match expected environment variable names
- Files contain only the secret value (no extra whitespace or newlines)

Enable debug logging to see which secrets are loaded:

```bash
export CS_LOG_LEVEL=debug
```

You'll see log entries like:

```
DEBUG Loaded secret as env var name=GITHUB_TOKEN source=/etc/secrets/GITHUB_TOKEN
INFO  Loaded secrets from directory path=/etc/secrets count=2
```

### Permission issues

Ensure the Code Search process has read access to the secrets directory:

```bash
# Check permissions
ls -la /etc/secrets/

# Fix permissions if needed (Kubernetes handles this automatically)
chmod 400 /etc/secrets/*
```

## Next Steps

- [Environment Variables](/configuration/environment-variables/) - All available environment variables
- [Kubernetes Deployment](/deployment/kubernetes/) - Deploy with Kubernetes secrets
- [Docker Compose](/deployment/docker-compose/) - Use Docker secrets
