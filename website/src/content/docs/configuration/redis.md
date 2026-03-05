---
title: Redis Configuration
description: Configure Redis connection for job queue
---

The `redis` section configures the Redis connection used for the job queue.

## Configuration

```yaml
redis:
  addr: "localhost:6379"
  password: ""
  db: 0
  # TLS configuration (optional)
  tls_enabled: false
  tls_skip_verify: false
  tls_cert_file: ""
  tls_key_file: ""
  tls_ca_cert_file: ""
  tls_server_name: ""
```

## Options

### `addr`

Redis server address in `host:port` format.

| Property | Value |
|----------|-------|
| Type | `string` |
| Default | `localhost:6379` |
| Environment | `CS_REDIS_ADDR` |

**Examples:**

```yaml
# Local development
addr: "localhost:6379"

# Docker Compose
addr: "redis:6379"

# Custom port
addr: "redis.example.com:6380"
```

### `password`

Redis password for authentication (if required).

| Property | Value |
|----------|-------|
| Type | `string` |
| Default | `""` (no password) |
| Environment | `CS_REDIS_PASSWORD` |

### `db`

Redis database number to use.

| Property | Value |
|----------|-------|
| Type | `integer` |
| Default | `0` |
| Environment | `CS_REDIS_DB` |

## TLS Configuration

Code Search supports TLS connections to Redis, which is required for managed Redis services like AWS ElastiCache, Azure Cache for Redis, and Google Cloud Memorystore.

### `tls_enabled`

Enable TLS connection to Redis.

| Property | Value |
|----------|-------|
| Type | `boolean` |
| Default | `false` |
| Environment | `CS_REDIS_TLS_ENABLED` |

### `tls_skip_verify`

Skip TLS certificate verification. **Not recommended for production.**

| Property | Value |
|----------|-------|
| Type | `boolean` |
| Default | `false` |
| Environment | `CS_REDIS_TLS_SKIP_VERIFY` |

### `tls_cert_file`

Path to client certificate file for mutual TLS (mTLS).

| Property | Value |
|----------|-------|
| Type | `string` |
| Default | `""` |
| Environment | `CS_REDIS_TLS_CERT_FILE` |

### `tls_key_file`

Path to client private key file for mutual TLS (mTLS).

| Property | Value |
|----------|-------|
| Type | `string` |
| Default | `""` |
| Environment | `CS_REDIS_TLS_KEY_FILE` |

### `tls_ca_cert_file`

Path to CA certificate file for verifying the Redis server certificate.

| Property | Value |
|----------|-------|
| Type | `string` |
| Default | `""` |
| Environment | `CS_REDIS_TLS_CA_CERT_FILE` |

### `tls_server_name`

Override the server name for TLS verification. Useful when the Redis hostname doesn't match the certificate.

| Property | Value |
|----------|-------|
| Type | `string` |
| Default | `""` |
| Environment | `CS_REDIS_TLS_SERVER_NAME` |

## TLS Examples

### AWS ElastiCache (encryption in-transit)

```yaml
redis:
  addr: "my-cluster.xxxxx.use1.cache.amazonaws.com:6379"
  tls_enabled: true
```

### Azure Cache for Redis

```yaml
redis:
  addr: "my-cache.redis.cache.windows.net:6380"
  password: "your-access-key"
  tls_enabled: true
```

### Self-signed certificate

```yaml
redis:
  addr: "redis.internal:6379"
  tls_enabled: true
  tls_ca_cert_file: "/etc/ssl/redis-ca.crt"
```

### Mutual TLS (mTLS)

```yaml
redis:
  addr: "redis.internal:6379"
  tls_enabled: true
  tls_cert_file: "/etc/ssl/redis-client.crt"
  tls_key_file: "/etc/ssl/redis-client.key"
  tls_ca_cert_file: "/etc/ssl/redis-ca.crt"
```

## Environment Variables

```bash
# Basic connection
CS_REDIS_ADDR="localhost:6379"
CS_REDIS_PASSWORD="secret"
CS_REDIS_DB="0"

# TLS connection
CS_REDIS_TLS_ENABLED="true"
CS_REDIS_TLS_SKIP_VERIFY="false"
CS_REDIS_TLS_CERT_FILE="/path/to/client.crt"
CS_REDIS_TLS_KEY_FILE="/path/to/client.key"
CS_REDIS_TLS_CA_CERT_FILE="/path/to/ca.crt"
CS_REDIS_TLS_SERVER_NAME="redis.example.com"
```

## Redis Requirements

Code Search requires Redis 6.0 or later.

### Memory Configuration

For most deployments, Redis default memory settings are sufficient. For large deployments:

```bash
# redis.conf
maxmemory 256mb
maxmemory-policy noeviction
```

**Important:** Use `noeviction` policy to prevent job data loss.

## How Redis is Used

Code Search uses Redis for:

1. **Job Queue** - Background jobs for indexing, syncing, and replace operations
2. **Job Results** - Storing job completion status and errors
3. **Job Progress** - Tracking progress of running jobs

### Queue Keys

| Key Pattern | Purpose |
|-------------|---------|
| `codesearch:jobs:queue` | Job queue |
| `codesearch:job:*` | Individual job data |

## Troubleshooting

### Connection refused

```text
failed to connect to Redis: connection refused
```

- Verify Redis is running: `redis-cli ping`
- Check host and port are correct
- Ensure Redis is bound to the correct interface

### Authentication required

```text
NOAUTH Authentication required
```

Set the password in config:

```yaml
redis:
  addr: "redis:6379"
  password: "your-password"
```

Or via environment variable:

```bash
CS_REDIS_PASSWORD="your-password"
```

### TLS handshake errors

```text
tls: failed to verify certificate
```

- Ensure `tls_enabled: true` is set
- Check the CA certificate path is correct
- Verify the server certificate is valid
- Use `tls_skip_verify: true` only for testing (not production)
