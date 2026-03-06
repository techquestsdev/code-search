package handlers

import (
	"encoding/json"
	"net/http"
	"time"
)

// SchedulerStatsResponse represents scheduler statistics.
type SchedulerStatsResponse struct {
	Enabled       bool      `json:"enabled"`
	TotalRepos    int       `json:"total_repos"`
	IndexedRepos  int       `json:"indexed_repos"`
	PendingRepos  int       `json:"pending_repos"`
	IndexingRepos int       `json:"indexing_repos"`
	FailedRepos   int       `json:"failed_repos"`
	StaleRepos    int       `json:"stale_repos"`
	NextCheckAt   time.Time `json:"next_check_at"`
	PollInterval  string    `json:"default_poll_interval"`
	CheckInterval string    `json:"check_interval"`
}

// GetSchedulerStats returns scheduler statistics.
func (h *Handler) GetSchedulerStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get poll interval from config (default to 6 hours if not set)
	pollInterval := h.schedulerConfig.PollInterval
	if pollInterval == 0 {
		pollInterval = 6 * time.Hour
	}

	staleThreshold := time.Now().Add(-pollInterval)

	// Get repo stats from service
	stats, err := h.services.Repos.GetRepoStats(ctx, staleThreshold)
	if err != nil {
		http.Error(w, "Failed to query stats: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get check interval from config (default to 5 minutes if not set)
	checkInterval := h.schedulerConfig.CheckInterval
	if checkInterval == 0 {
		checkInterval = 5 * time.Minute
	}

	response := SchedulerStatsResponse{
		Enabled:       h.schedulerConfig.Enabled,
		TotalRepos:    stats.Total,
		IndexedRepos:  stats.Indexed,
		PendingRepos:  stats.Pending,
		IndexingRepos: stats.Indexing,
		FailedRepos:   stats.Failed,
		StaleRepos:    stats.Stale,
		NextCheckAt:   time.Now().Add(checkInterval),
		PollInterval:  pollInterval.String(),
		CheckInterval: checkInterval.String(),
	}

	writeJSON(w, response)
}

// TriggerSyncAllRequest represents a request to sync all repos.
type TriggerSyncAllRequest struct {
	Force bool `json:"force"` // If true, sync all regardless of last_indexed
}

// TriggerSyncAll triggers a sync for all repositories or stale ones.
func (h *Handler) TriggerSyncAll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req TriggerSyncAllRequest
	if r.Body != nil && r.ContentLength > 0 {
		json.NewDecoder(r.Body).Decode(&req)
	}

	var (
		count int
		err   error
	)

	if req.Force {
		// Force sync all repos
		count, err = h.services.Repos.TriggerSyncAllRepos(ctx)
	} else {
		// Only sync stale repos (use poll interval from config, default 6 hours)
		pollInterval := h.schedulerConfig.PollInterval
		if pollInterval == 0 {
			pollInterval = 6 * time.Hour
		}

		staleThreshold := time.Now().Add(-pollInterval)
		count, err = h.services.Repos.TriggerSyncStaleRepos(ctx, staleThreshold)
	}

	if err != nil {
		http.Error(w, "Failed to trigger sync: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"status":       "triggered",
		"repos_queued": count,
		"force":        req.Force,
	})
}
