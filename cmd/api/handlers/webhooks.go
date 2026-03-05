package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/techquestsdev/code-search/internal/log"
	"github.com/techquestsdev/code-search/internal/queue"
)

// WebhookConfig holds webhook configuration.
type WebhookConfig struct {
	Secret string // Shared secret for webhook signature validation
}

// HandleWebhook processes incoming webhooks from code hosts.
// POST /api/v1/webhooks/{provider}
// Supported providers: github, gitlab, gitea, bitbucket.
func (h *Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	if provider == "" {
		// Fallback for chi router
		provider = strings.TrimPrefix(r.URL.Path, "/api/v1/webhooks/")
		provider = strings.TrimSuffix(provider, "/")
	}

	// Read body
	body, err := io.ReadAll(io.LimitReader(r.Body, 1*1024*1024)) // 1MB limit
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Extract repo name based on provider
	var repoName string
	var isPush bool

	switch provider {
	case "github":
		repoName, isPush = parseGitHubWebhook(r, body)
	case "gitlab":
		repoName, isPush = parseGitLabWebhook(r, body)
	case "gitea":
		repoName, isPush = parseGiteaWebhook(r, body)
	case "bitbucket":
		repoName, isPush = parseBitbucketWebhook(r, body)
	default:
		http.Error(w, "Unsupported provider: "+provider, http.StatusBadRequest)
		return
	}

	if !isPush {
		// Not a push event — acknowledge but don't act
		writeJSON(w, map[string]any{
			"received": true,
			"action":   "ignored",
			"reason":   "not a push event",
		})

		return
	}

	if repoName == "" {
		http.Error(w, "Could not extract repository name from webhook payload", http.StatusBadRequest)
		return
	}

	logger := log.L.With(
		zap.String("provider", provider),
		zap.String("repo", repoName),
	)

	// Look up the repository in the database
	repo, err := h.services.Repos.GetRepositoryByName(r.Context(), repoName)
	if err != nil {
		logger.Warn("Webhook repo lookup failed", zap.Error(err))
		http.Error(w, "Failed to look up repository", http.StatusInternalServerError)

		return
	}

	if repo == nil {
		logger.Debug("Webhook received for unknown repo")
		writeJSON(w, map[string]any{
			"received": true,
			"action":   "ignored",
			"reason":   "repository not found",
		})

		return
	}

	if repo.Excluded {
		writeJSON(w, map[string]any{
			"received": true,
			"action":   "ignored",
			"reason":   "repository is excluded",
		})

		return
	}

	// Check if an index job is already active for this repo
	acquired, err := h.services.Queue.TryAcquireIndexJob(r.Context(), repo.ID)
	if err != nil {
		logger.Warn("Failed to check active index job", zap.Error(err))
	}

	if !acquired {
		logger.Debug("Webhook skipped — index job already active")
		writeJSON(w, map[string]any{
			"received": true,
			"action":   "skipped",
			"reason":   "index job already active",
		})

		return
	}

	// Queue an index job for immediate re-indexing
	payload := queue.IndexPayload{
		RepositoryID: repo.ID,
		ConnectionID: repo.ConnectionID,
		RepoName:     repo.Name,
		CloneURL:     repo.CloneURL,
		Branch:       repo.DefaultBranch,
	}

	_, err = h.services.Queue.Enqueue(r.Context(), queue.JobTypeIndex, payload)
	if err != nil {
		_ = h.services.Queue.MarkIndexJobInactive(r.Context(), repo.ID)
		logger.Error("Failed to queue index job from webhook", zap.Error(err))
		http.Error(w, "Failed to queue index job", http.StatusInternalServerError)

		return
	}

	logger.Info("Webhook triggered re-index")

	writeJSON(w, map[string]any{
		"received": true,
		"action":   "queued",
		"repo":     repoName,
	})
}

// parseGitHubWebhook extracts repo name from a GitHub push webhook.
func parseGitHubWebhook(r *http.Request, body []byte) (string, bool) {
	event := r.Header.Get("X-GitHub-Event")
	if event != "push" {
		return "", false
	}

	var payload struct {
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return "", true
	}

	return payload.Repository.FullName, true
}

// parseGitLabWebhook extracts repo name from a GitLab push webhook.
func parseGitLabWebhook(r *http.Request, body []byte) (string, bool) {
	event := r.Header.Get("X-Gitlab-Event")
	if event != "Push Hook" && event != "Tag Push Hook" {
		return "", false
	}

	var payload struct {
		Project struct {
			PathWithNamespace string `json:"path_with_namespace"`
		} `json:"project"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return "", true
	}

	return payload.Project.PathWithNamespace, true
}

// parseGiteaWebhook extracts repo name from a Gitea push webhook.
func parseGiteaWebhook(r *http.Request, body []byte) (string, bool) {
	event := r.Header.Get("X-Gitea-Event")
	if event != "push" {
		return "", false
	}

	var payload struct {
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return "", true
	}

	return payload.Repository.FullName, true
}

// parseBitbucketWebhook extracts repo name from a Bitbucket push webhook.
func parseBitbucketWebhook(r *http.Request, body []byte) (string, bool) {
	event := r.Header.Get("X-Event-Key")
	if event != "repo:push" {
		return "", false
	}

	var payload struct {
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return "", true
	}

	return payload.Repository.FullName, true
}

// ValidateGitHubSignature validates a GitHub webhook signature.
func ValidateGitHubSignature(body []byte, signature, secret string) bool {
	if secret == "" || signature == "" {
		return secret == "" // Accept if no secret configured
	}

	sig := strings.TrimPrefix(signature, "sha256=")

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(sig), []byte(expected))
}
