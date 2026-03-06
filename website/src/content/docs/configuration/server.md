---
title: Server Configuration
description: Configure the API server settings
---

The `server` section configures the Code Search API server.

## Configuration

```yaml
server:
  addr: ":8080"
  read_timeout: 15s
  write_timeout: 60s
```

## Options

### `addr`

The address to listen on in `host:port` format. Use `:port` to listen on all interfaces.

| Property | Value |
|----------|-------|
| Type | `string` |
| Default | `":8080"` |
| Environment | `CS_SERVER_ADDR` |

**Examples:**

- `":8080"` - Listen on port 8080 on all interfaces
- `"0.0.0.0:8080"` - Same as above (explicit)
- `"127.0.0.1:8080"` - Listen only on localhost
- `":3000"` - Use port 3000

### `read_timeout`

Maximum time to read the entire request, including the body.

| Property | Value |
|----------|-------|
| Type | `duration` |
| Default | `15s` |
| Environment | `CS_SERVER_READ_TIMEOUT` |

### `write_timeout`

Maximum time to write the response. Should be longer than typical request processing time.

| Property | Value |
|----------|-------|
| Type | `duration` |
| Default | `60s` |
| Environment | `CS_SERVER_WRITE_TIMEOUT` |

**Note:** Search requests can take a few seconds on large indexes, so keep `write_timeout` at 60s or higher.

## Full Example

```yaml
server:
  addr: ":8080"
  read_timeout: 15s
  write_timeout: 60s
```

## Environment Variables

```bash
CS_SERVER_ADDR=":8080"
CS_SERVER_READ_TIMEOUT="15s"
CS_SERVER_WRITE_TIMEOUT="60s"
```

## API Endpoints

The server exposes these endpoints on the configured address:

| Endpoint | Description |
|----------|-------------|
| `/api/v1/search` | Search API (POST) |
| `/api/v1/repos` | Repository management |
| `/api/v1/connections` | Connection management |
| `/api/v1/jobs` | Job queue management |
| `/api/v1/symbols/definitions` | Symbol definitions |
| `/api/v1/symbols/refs` | Symbol references |
