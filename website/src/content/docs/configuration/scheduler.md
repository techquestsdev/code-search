---
title: Scheduler Configuration
description: Configure automatic repository synchronization
---

The `scheduler` section configures automatic repository synchronization to keep indexes up-to-date.

## Configuration

```yaml
scheduler:
  enabled: true
  poll_interval: 6h
  check_interval: 5m
  stale_threshold: 24h
  max_concurrent_checks: 5
  job_retention: 1h
```

## Options

### `enabled`

Enable or disable the automatic scheduler.

| Property | Value |
|----------|-------|
| Type | `boolean` |
| Default | `true` |
| Environment | `CS_SCHEDULER_ENABLED` |

When enabled, the scheduler automatically queues repositories for re-indexing based on `poll_interval`.

### `poll_interval`

Default time between syncs for each repository.

| Property | Value |
|----------|-------|
| Type | `duration` |
| Default | `6h` |
| Environment | `CS_SCHEDULER_POLL_INTERVAL` |

Repositories will be re-indexed at least this often.

**Examples:**

- `1h` - Every hour (for fast-moving repositories)
- `6h` - Every 6 hours (default, good balance)
- `24h` - Daily (for slow-moving repositories)

### `check_interval`

How often the scheduler checks for repositories needing sync.

| Property | Value |
|----------|-------|
| Type | `duration` |
| Default | `5m` |
| Environment | `CS_SCHEDULER_CHECK_INTERVAL` |

This is how frequently the scheduler looks for repos that need syncing.

### `stale_threshold`

Maximum time before a repository is considered stale and needs sync.

| Property | Value |
|----------|-------|
| Type | `duration` |
| Default | `24h` |
| Environment | `CS_SCHEDULER_STALE_THRESHOLD` |

### `max_concurrent_checks`

Maximum number of parallel git fetch checks.

| Property | Value |
|----------|-------|
| Type | `integer` |
| Default | `5` |
| Environment | `CS_SCHEDULER_MAX_CONCURRENT_CHECKS` |

Controls how many repositories can be checked for updates simultaneously.

### `job_retention`

How long to keep completed/failed jobs before automatic cleanup.

| Property | Value |
|----------|-------|
| Type | `duration` |
| Default | `1h` |
| Environment | `CS_SCHEDULER_JOB_RETENTION` |

The scheduler automatically cleans up old completed and failed jobs every 10 minutes. Set to `0` to disable automatic cleanup.

**Examples:**

- `30m` - Keep jobs for 30 minutes
- `1h` - Keep jobs for 1 hour (default)
- `24h` - Keep jobs for 24 hours
- `0` - Disable automatic cleanup

## Environment Variables

```bash
CS_SCHEDULER_ENABLED="true"
CS_SCHEDULER_POLL_INTERVAL="6h"
CS_SCHEDULER_CHECK_INTERVAL="5m"
CS_SCHEDULER_STALE_THRESHOLD="24h"
CS_SCHEDULER_MAX_CONCURRENT_CHECKS="5"
CS_SCHEDULER_JOB_RETENTION="1h"
```

## How the Scheduler Works

1. **Leader election** - On startup, each API server attempts to acquire a Redis lease (`scheduler:leader`). Only the leader runs scheduler ticks.
2. **Check** - Every `check_interval`, the leader finds repos not indexed recently
3. **Queue** - Add sync jobs to Redis queue
4. **Process** - Indexer picks up and processes jobs
5. **Update** - `last_indexed` timestamp is updated

### Automatic Leader Election

The scheduler uses Redis-based leader election, so you can run multiple API server replicas safely. Only one instance runs the scheduler at a time:

- The leader holds a 30-second lease, refreshed every 10 seconds
- If the leader crashes, another instance takes over within 30 seconds
- On graceful shutdown, the leader releases its lease immediately for instant failover

No manual configuration is needed — all instances can have `scheduler.enabled: true`.

## Per-Repository Poll Intervals

Individual repositories can have custom poll intervals:

```bash
# Set via API
curl -X PUT "http://localhost:8080/api/v1/repos/by-id/123/poll-interval" \
  -H "Content-Type: application/json" \
  -d '{"interval_seconds": 3600}'  # 1 hour

# Reset to default
curl -X PUT "http://localhost:8080/api/v1/repos/by-id/123/poll-interval" \
  -H "Content-Type: application/json" \
  -d '{"interval_seconds": 0}'
```

Use this for:

- High-activity repositories that need frequent updates
- Archives or slow-moving repos that don't need frequent syncs

## Manual Sync

Trigger a sync manually:

```bash
# Sync a single repository
curl -X POST "http://localhost:8080/api/v1/repos/by-id/123/sync"

# Sync a connection
curl -X POST "http://localhost:8080/api/v1/connections/1/sync"
```

## Disabling the Scheduler

To disable automatic scheduling:

```yaml
scheduler:
  enabled: false
```

With the scheduler disabled:

- Repositories are only indexed on initial add
- Use manual sync to update indexes
- Useful for read-only/archive deployments

## Best Practices

### Poll Interval Selection

| Repository Type | Recommended Poll Interval |
|-----------------|---------------------------|
| Active development | `1h` - `6h` |
| Stable/maintenance | `24h` |
| Archives | Use per-repo custom interval |

### Resource Considerations

- Lower `poll_interval` = more indexing jobs = more resources
- Consider your indexer capacity when setting intervals
- Monitor job queue length to ensure indexers keep up

## Troubleshooting

### Repos not being synced

1. Check scheduler is enabled: `scheduler.enabled: true`
2. Verify intervals are set correctly
3. Check jobs page for pending/failed jobs
4. Verify indexer is running and processing jobs

### Too many jobs queued

If the job queue grows faster than indexers can process:

- Increase indexer concurrency
- Increase `poll_interval` to reduce frequency
- Set per-repo poll intervals for less active repos
