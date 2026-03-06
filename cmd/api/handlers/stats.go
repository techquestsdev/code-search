package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// RepoStatsResponse represents overall repository statistics.
type RepoStatsResponse struct {
	Total    int `json:"total"`
	Indexed  int `json:"indexed"`
	Pending  int `json:"pending"`
	Indexing int `json:"indexing"`
	Failed   int `json:"failed"`
	Excluded int `json:"excluded"`
}

// GetRepoStats returns overall repository statistics.
// GET /api/v1/repos/stats.
func (h *Handler) GetRepoStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.services.Repos.GetStats(r.Context())
	if err != nil {
		http.Error(w, "Failed to get stats: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := RepoStatsResponse{
		Total:    stats.Total,
		Indexed:  stats.Indexed,
		Pending:  stats.Pending,
		Indexing: stats.Indexing,
		Failed:   stats.Failed,
		Excluded: stats.Excluded,
	}

	writeJSON(w, response)
}

// GetConnectionStats returns statistics for a specific connection.
// GET /api/v1/connections/{id}/stats.
func (h *Handler) GetConnectionStats(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid connection ID", http.StatusBadRequest)
		return
	}

	// Verify connection exists
	conn, err := h.services.Repos.GetConnection(r.Context(), id)
	if err != nil {
		http.Error(w, "Failed to get connection: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if conn == nil {
		http.Error(w, "Connection not found", http.StatusNotFound)
		return
	}

	stats, err := h.services.Repos.GetConnectionStats(r.Context(), id)
	if err != nil {
		http.Error(
			w,
			"Failed to get connection stats: "+err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	response := RepoStatsResponse{
		Total:    stats.Total,
		Indexed:  stats.Indexed,
		Pending:  stats.Pending,
		Indexing: stats.Indexing,
		Failed:   stats.Failed,
		Excluded: stats.Excluded,
	}

	writeJSON(w, response)
}

// QueueStatsResponse represents queue statistics.
type QueueStatsResponse struct {
	PendingJobs int64 `json:"pending_jobs"`
}

// GetQueueStats returns queue statistics.
// GET /api/v1/jobs/stats.
func (h *Handler) GetQueueStats(w http.ResponseWriter, r *http.Request) {
	length, err := h.services.Queue.QueueLength(r.Context())
	if err != nil {
		http.Error(w, "Failed to get queue stats: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := QueueStatsResponse{
		PendingJobs: length,
	}

	writeJSON(w, response)
}

// CleanupStaleIndexing resets repositories stuck in "indexing" status.
// POST /api/v1/scheduler/cleanup.
func (h *Handler) CleanupStaleIndexing(w http.ResponseWriter, r *http.Request) {
	// Default to 1 hour - repos stuck indexing for more than 1 hour are considered stale
	staleThreshold := 1 * time.Hour

	// Allow override via query param
	if thresholdStr := r.URL.Query().Get("threshold_minutes"); thresholdStr != "" {
		if minutes, err := strconv.Atoi(thresholdStr); err == nil && minutes > 0 {
			staleThreshold = time.Duration(minutes) * time.Minute
		}
	}

	count, err := h.services.Repos.CleanupStaleIndexing(r.Context(), staleThreshold)
	if err != nil {
		http.Error(
			w,
			"Failed to cleanup stale indexing: "+err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	writeJSON(w, map[string]any{
		"status":            "ok",
		"repos_cleaned":     count,
		"threshold_minutes": int(staleThreshold.Minutes()),
		"message":           "Stale indexing jobs cleaned up",
	})
}
