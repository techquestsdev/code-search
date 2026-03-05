---
title: Webhooks
description: Configure webhook-driven instant re-indexing
---

Code Search can receive push webhooks from your code hosts to trigger immediate re-indexing when code changes. This reduces the delay between a push and the code being searchable from minutes (poll-based) to seconds.

## How It Works

1. Developer pushes code to a repository
2. Code host sends a webhook to Code Search
3. Code Search looks up the repository and queues an index job
4. The indexer re-indexes the repository immediately

Only push events trigger re-indexing. Other events (pull requests, issues, etc.) are acknowledged but ignored.

## Endpoint

```
POST /api/v1/webhooks/{provider}
```

Supported providers: `github`, `gitlab`, `gitea`, `bitbucket`.

## Setup

### GitHub

1. Go to your repository or organization settings
2. Navigate to **Webhooks** > **Add webhook**
3. Set the **Payload URL** to:
   ```
   https://your-code-search.example.com/api/v1/webhooks/github
   ```
4. Set **Content type** to `application/json`
5. Select **Just the push event**
6. Click **Add webhook**

#### Organization-wide webhook

For GitHub organizations, you can configure a single webhook that covers all repositories:

1. Go to your organization settings
2. Navigate to **Webhooks** > **Add webhook**
3. Use the same URL and settings as above

### GitLab

1. Go to your project or group settings
2. Navigate to **Webhooks**
3. Set the **URL** to:
   ```
   https://your-code-search.example.com/api/v1/webhooks/gitlab
   ```
4. Check **Push events** and **Tag push events**
5. Click **Add webhook**

#### Group-level webhook

For GitLab groups (Premium/Ultimate), configure a group webhook to cover all projects:

1. Go to your group settings
2. Navigate to **Webhooks**
3. Use the same URL and settings as above

### Gitea

1. Go to your repository settings
2. Navigate to **Webhooks** > **Add Webhook** > **Gitea**
3. Set the **Target URL** to:
   ```
   https://your-code-search.example.com/api/v1/webhooks/gitea
   ```
4. Select **Push Events** under trigger events
5. Click **Add Webhook**

### Bitbucket

1. Go to your repository settings
2. Navigate to **Webhooks** > **Add webhook**
3. Set the **URL** to:
   ```
   https://your-code-search.example.com/api/v1/webhooks/bitbucket
   ```
4. Select **Repository push** under triggers
5. Click **Save**

## Event Headers

Code Search identifies the provider and event type using standard headers:

| Provider | Event Header | Push Event Value |
|----------|-------------|-----------------|
| GitHub | `X-GitHub-Event` | `push` |
| GitLab | `X-Gitlab-Event` | `Push Hook` or `Tag Push Hook` |
| Gitea | `X-Gitea-Event` | `push` |
| Bitbucket | `X-Event-Key` | `repo:push` |

## Response Format

All webhook responses return JSON:

```json
// Successfully queued
{
  "received": true,
  "action": "queued",
  "repo": "org/repo-name"
}

// Skipped — job already active
{
  "received": true,
  "action": "skipped",
  "reason": "index job already active"
}

// Ignored — not a push event
{
  "received": true,
  "action": "ignored",
  "reason": "not a push event"
}

// Ignored — repo not found or excluded
{
  "received": true,
  "action": "ignored",
  "reason": "repository not found"
}
```

## Idempotency

If a repository already has an active index job (e.g., from a previous push or a scheduled sync), the webhook is acknowledged but no duplicate job is created. This prevents job flooding when multiple pushes happen in quick succession.

## Combining with Scheduled Sync

Webhooks and scheduled sync work together:

- **Webhooks** provide instant re-indexing for push events
- **Scheduled sync** catches changes that don't trigger webhooks (new repos, force pushes, branch deletions)

You don't need to choose one or the other. Keep the scheduler enabled and add webhooks for faster updates.

## Troubleshooting

### Webhook not triggering re-index

1. Check the webhook delivery logs in your code host — look for HTTP 200 responses
2. Verify the repository name in the webhook payload matches the name in Code Search
3. Check that the repository is not excluded
4. Look at the API server logs for webhook processing details

### Duplicate index jobs

This shouldn't happen — the webhook handler checks for active jobs before queuing. If you see duplicates, check that you don't have multiple webhooks configured for the same repository.
