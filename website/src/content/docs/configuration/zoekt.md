---
title: Zoekt Configuration
description: Configure the Zoekt search engine
---

The `zoekt` section configures the Zoekt search engine integration.

## Configuration

```yaml
zoekt:
  url: "http://localhost:6070"
  index_dir: "/data/index"
  repos_dir: "/data/repos"
```

## Options

### `url`

URL of the Zoekt web server.

| Property | Value |
|----------|-------|
| Type | `string` |
| Default | `"http://localhost:6070"` |
| Environment | `CS_ZOEKT_URL` |

The Zoekt web server provides the search API. Code Search queries this server for search operations.

**Examples:**

```yaml
# Local development
url: "http://localhost:6070"

# Docker Compose
url: "http://zoekt:6070"

# Kubernetes
url: "http://code-search-zoekt:6070"
```

### `index_dir`

Directory where Zoekt stores search indexes.

| Property | Value |
|----------|-------|
| Type | `string` |
| Default | `"/data/index"` |
| Environment | `CS_ZOEKT_INDEX_DIR` |

This directory must be:

- Shared between the indexer and Zoekt web server
- Writable by the indexer
- Readable by the Zoekt web server

### `repos_dir`

Directory where Git repositories are cloned.

| Property | Value |
|----------|-------|
| Type | `string` |
| Default | `"/data/repos"` |
| Environment | `CS_ZOEKT_REPOS_DIR` |

This directory must be:

- Shared between the indexer and Zoekt web server
- Writable by the indexer

## Environment Variables

```bash
CS_ZOEKT_URL="http://localhost:6070"
CS_ZOEKT_INDEX_DIR="/data/index"
CS_ZOEKT_REPOS_DIR="/data/repos"
```

## Directory Structure

```
/data/
├── index/                    # Zoekt indexes
│   └── myorg%2Fmyrepo.git/   # Index files per repo
│       ├── ...
│       └── metadata.json
└── repos/                    # Git repositories
    └── myorg/
        └── myrepo/           # Bare git clone
            ├── HEAD
            ├── objects/
            └── refs/
```

## How Zoekt Works

1. **Indexer** clones repositories to `repos_dir`
2. **Indexer** runs `zoekt-index` to create indexes in `index_dir`
3. **Zoekt web server** loads indexes from `index_dir`
4. **Code Search API** queries Zoekt web server for search requests

## Zoekt Web Server

The Zoekt web server is a separate process that must be running. It's included in the Docker images.

### Health Check

```bash
curl http://localhost:6070/health
```

### Search API

The Zoekt web server exposes a search API:

```bash
# Search
curl "http://localhost:6070/api/search?q=FOO"

# List indexed repos
curl "http://localhost:6070/api/list"
```

## Disk Space Requirements

Zoekt indexes are approximately 5-10% of the source code size. Plan disk space accordingly:

| Source Code Size | Index Size (approx) |
|------------------|---------------------|
| 1 GB | 50-100 MB |
| 10 GB | 500 MB - 1 GB |
| 100 GB | 5-10 GB |

The `repos_dir` contains full git clones, so plan for at least 2x your total source code size for repos + indexes.

## Performance Tuning

### Memory

Zoekt loads indexes into memory for fast search. Ensure sufficient RAM:

- **Minimum:** Index size + 1 GB
- **Recommended:** 2x index size + 2 GB

### CPU

Search is CPU-bound for regex queries. More CPU cores = faster search.

### Sharding

For very large codebases, consider running multiple Zoekt instances with different subsets of repositories.

## Troubleshooting

### Zoekt not responding

```
failed to connect to Zoekt: connection refused
```

- Verify Zoekt is running: `docker compose ps zoekt`
- Check Zoekt logs: `docker compose logs zoekt`
- Verify the URL is correct

### Index not found

```
no index found for repo
```

- Check if the repository is indexed: `curl http://localhost:6070/api/list`
- Verify the index directory contains files
- Check indexer logs for errors

### Search returns no results

- Verify the repository is indexed
- Check if the query syntax is correct
- Try a simpler query to test
