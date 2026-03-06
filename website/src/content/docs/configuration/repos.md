---
title: Repository Configuration
description: Configure repository management settings
---

The `repos` section configures repository management behavior.

## Configuration

```yaml
repos:
  readonly: false
```

## Options

### `readonly`

Prevent modifications to repositories via UI and API.

| Property | Value |
|----------|-------|
| Type | `boolean` |
| Default | `false` |
| Environment | `CS_REPOS_READONLY` |

When enabled:

- Cannot add new repositories
- Cannot delete repositories
- Cannot modify repository settings
- Sync operations still work (read from code host, update index)

**Use cases:**

- Production environments where repos are managed via config-as-code
- Multi-tenant deployments where admins control repo access
- Preventing accidental changes

## Connections Readonly

Similarly, connections can be set to readonly:

```yaml
connections:
  readonly: false
```

### `connections.readonly`

| Property | Value |
|----------|-------|
| Type | `boolean` |
| Default | `false` |
| Environment | `CS_CONNECTIONS_READONLY` |

When enabled:

- Cannot add new connections
- Cannot delete connections
- Cannot modify connection settings
- Sync operations still work

## Environment Variables

```bash
CS_REPOS_READONLY="false"
CS_CONNECTIONS_READONLY="false"
```

## Use Cases

### Development Environment

```yaml
repos:
  readonly: false
connections:
  readonly: false
```

Full access for testing and development.

### Production Environment

```yaml
repos:
  readonly: true
connections:
  readonly: true
```

Managed via CI/CD, no manual changes allowed.

### Managed Connections, User Repos

```yaml
repos:
  readonly: false
connections:
  readonly: true
```

Admins manage code host connections, but users can add their own repositories.

## Config-as-Code

For production deployments, define connections and repositories in configuration:

```yaml
codehosts:
  - name: "GitHub Production"
    type: "github"
    url: "https://api.github.com"
    token_env: "GITHUB_TOKEN"  # Read from environment
    repos:
      include:
        - "myorg/*"
      exclude:
        - "myorg/deprecated-*"
```

This approach:

- Version controls your configuration
- Enables GitOps workflows
- Provides audit trail for changes

## API Behavior

When readonly mode is enabled, modification endpoints return `403 Forbidden`:

```bash
# Attempt to add repo in readonly mode
curl -X POST "http://localhost:8080/api/v1/repos" \
  -H "Content-Type: application/json" \
  -d '{"name": "test"}'

# Response: 403 Forbidden
{
  "error": "repository modifications are disabled (readonly mode)"
}
```

Read operations still work normally:

- Listing repositories
- Viewing repository details
- Searching code
- Triggering syncs
