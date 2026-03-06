---
title: Security Configuration
description: How to configure security features including token encryption at rest
---

Code Search provides security features to protect sensitive data like API tokens stored in the database.

## Token Encryption at Rest

Connection tokens (GitHub, GitLab, Gitea, Bitbucket) can be encrypted before being stored in the database using AES-256-GCM encryption.

### Why Encrypt Tokens?

- **Database compromise protection**: If your database is compromised, encrypted tokens are useless without the encryption key
- **Compliance requirements**: Many security standards require encryption of sensitive data at rest
- **Defense in depth**: Adds an extra layer of security beyond database access controls

### Enabling Encryption

Set the encryption key via environment variable (recommended):

```bash
export CS_SECURITY_ENCRYPTION_KEY="your-secret-encryption-key-here"
```

Or in your `config.yaml`:

```yaml
security:
  encryption_key: "your-secret-encryption-key-here"
```

:::caution
Store the encryption key securely. If you lose it, you won't be able to decrypt existing tokens and will need to re-create all connections with new tokens.
:::

### Key Requirements

- **Any string is valid**: The key is hashed using SHA-256 to derive a 32-byte AES-256 key
- **Recommended length**: Use at least 32 characters for security
- **Cryptographically random**: Use a secure random generator for production keys

Generate a secure key:

```bash
# Using OpenSSL
openssl rand -base64 32

# Using /dev/urandom
head -c 32 /dev/urandom | base64

# Using Python
python3 -c "import secrets; print(secrets.token_urlsafe(32))"
```

### How It Works

1. **On write**: When a connection is created or updated, the token is encrypted with AES-256-GCM and prefixed with `enc:`
2. **On read**: When fetching a connection, the token is decrypted in memory before use
3. **Detection**: Encrypted values have an `enc:` prefix, allowing Code Search to distinguish them from plaintext

### Backwards Compatibility

Token encryption is fully backwards compatible:

| Scenario | Behavior |
|----------|----------|
| No encryption key set | Tokens stored/read as plaintext (existing behavior) |
| Encryption key set, existing plaintext tokens | Plaintext tokens continue to work, new tokens are encrypted |
| Encryption key set, token updated | Token is encrypted on save |

This allows gradual migration without disrupting existing deployments.

### Kubernetes Deployment

Store the encryption key as a Kubernetes secret:

```bash
# Generate a secure key
ENCRYPTION_KEY=$(openssl rand -base64 32)

# Create the secret
kubectl create secret generic code-search-encryption \
  --namespace code-search \
  --from-literal=CS_SECURITY_ENCRYPTION_KEY="$ENCRYPTION_KEY"
```

Reference in your deployment:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: code-search-api
spec:
  template:
    spec:
      containers:
        - name: api
          envFrom:
            - secretRef:
                name: code-search-encryption
```

Or with the Helm chart:

```yaml
# values.yaml
extraEnvVars:
  - name: CS_SECURITY_ENCRYPTION_KEY
    valueFrom:
      secretKeyRef:
        name: code-search-encryption
        key: CS_SECURITY_ENCRYPTION_KEY
```

### Docker Compose

```yaml
services:
  api:
    image: ghcr.io/techquestsdev/code-search-api:latest
    environment:
      CS_SECURITY_ENCRYPTION_KEY: ${ENCRYPTION_KEY}
    # Or use Docker secrets
    secrets:
      - encryption_key
    
secrets:
  encryption_key:
    file: ./secrets/encryption_key.txt
```

### Key Rotation

Currently, key rotation requires re-creating connections:

1. Export your connection configurations (without tokens)
2. Set the new encryption key
3. Re-create connections with fresh tokens

:::note
Automatic key rotation is not yet supported. All encrypted tokens use the same key.
:::

### Verifying Encryption

You can verify tokens are encrypted by checking the database directly:

```sql
-- Encrypted tokens start with "enc:"
SELECT name, 
       CASE WHEN token LIKE 'enc:%' THEN 'encrypted' ELSE 'plaintext' END as status
FROM connections;
```

### Troubleshooting

#### Tokens not being encrypted

1. Verify the encryption key is set:
   ```bash
   echo $CS_SECURITY_ENCRYPTION_KEY
   ```

2. Check logs for encryption status on startup:
   ```
   INFO Token encryption enabled
   ```

3. Update existing connections to trigger encryption (tokens are encrypted on save)

#### Decryption errors

If you see decryption errors in logs:

1. **Wrong key**: Ensure the same key is used across all API and indexer instances
2. **Key changed**: If the key was rotated, old encrypted tokens won't decrypt
3. **Corrupted data**: Check database for corrupted token values

For backwards compatibility, decryption errors fall back to using the raw value, so existing plaintext tokens continue to work.

## Environment Variable Reference

| Variable | Description | Default |
|----------|-------------|---------|
| `CS_SECURITY_ENCRYPTION_KEY` | AES-256-GCM encryption key for tokens | - (disabled) |

## Best Practices

1. **Always encrypt in production**: Enable token encryption for any production deployment
2. **Use secrets management**: Store the encryption key in a secrets manager (Vault, AWS Secrets Manager, etc.)
3. **Same key everywhere**: Ensure API and indexer use the same encryption key
4. **Backup the key**: Store the encryption key securely in case of disaster recovery
5. **Rotate tokens, not just keys**: Regularly rotate the underlying API tokens with your code hosts

## Next Steps

- [Secrets Management](/configuration/secrets/) - Loading secrets from files
- [Environment Variables](/configuration/environment-variables/) - Complete environment variable reference
- [Kubernetes Deployment](/deployment/kubernetes/) - Deploy securely on Kubernetes
