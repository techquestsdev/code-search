package metrics

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// HTTP metrics.
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "codesearch_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "codesearch_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	httpRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "codesearch_http_requests_in_flight",
			Help: "Number of HTTP requests currently being processed",
		},
	)

	// Rate limiting metrics.
	rateLimitRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "codesearch_rate_limit_requests_total",
			Help: "Total rate-limited requests by result",
		},
		[]string{"result"}, // allowed, rejected
	)

	rateLimitBucketsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "codesearch_rate_limit_buckets_active",
			Help: "Number of active rate limit buckets (unique IPs)",
		},
	)

	// Code host API metrics.
	codeHostRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "codesearch_codehost_requests_total",
			Help: "Total requests to code host APIs",
		},
		[]string{
			"host_type",
			"endpoint",
			"status",
		}, // github, gitlab, etc. / repos, user / success, error
	)

	codeHostRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "codesearch_codehost_request_duration_seconds",
			Help:    "Code host API request duration in seconds",
			Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
		},
		[]string{"host_type", "endpoint"},
	)

	codeHostRateLimitRemaining = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "codesearch_codehost_rate_limit_remaining",
			Help: "Remaining API rate limit quota",
		},
		[]string{"host_type"},
	)

	codeHostRateLimitReset = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "codesearch_codehost_rate_limit_reset_timestamp",
			Help: "Unix timestamp when rate limit resets",
		},
		[]string{"host_type"},
	)

	// Git operation metrics.
	gitCloneDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "codesearch_git_clone_duration_seconds",
			Help:    "Git clone operation duration in seconds",
			Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600},
		},
		[]string{"status"}, // success, failed
	)

	gitFetchDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "codesearch_git_fetch_duration_seconds",
			Help:    "Git fetch operation duration in seconds",
			Buckets: []float64{0.5, 1, 5, 10, 30, 60, 120},
		},
		[]string{"status"}, // success, failed
	)

	gitOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "codesearch_git_operations_total",
			Help: "Total git operations by type and status",
		},
		[]string{"operation", "status"}, // clone, fetch / success, failed
	)

	// Branch indexing metrics.
	branchesIndexedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "codesearch_branches_indexed_total",
			Help: "Total branches indexed",
		},
		[]string{"status"}, // success, failed
	)

	branchesPerRepo = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "codesearch_branches_per_repo",
			Help:    "Number of branches indexed per repository",
			Buckets: []float64{1, 2, 3, 5, 10, 20, 50},
		},
		[]string{},
	)

	// Error metrics by type.
	errorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "codesearch_errors_total",
			Help: "Total errors by category and type",
		},
		[]string{
			"category",
			"error_type",
		}, // db, git, codehost, search / connection, timeout, auth, etc.
	)

	// Search metrics.
	searchRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "codesearch_search_requests_total",
			Help: "Total number of search requests",
		},
		[]string{"type"}, // text, regex, symbol
	)

	searchDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "codesearch_search_duration_seconds",
			Help:    "Search request duration in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"type"},
	)

	searchResultsCount = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "codesearch_search_results_count",
			Help:    "Number of search results returned",
			Buckets: []float64{0, 1, 10, 50, 100, 500, 1000, 5000, 10000},
		},
		[]string{"type"},
	)

	// Repository metrics.
	repositoriesTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "codesearch_repositories",
			Help: "Number of repositories by status",
		},
		[]string{"status"},
	)

	repositoryIndexDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "codesearch_repository_index_duration_seconds",
			Help:    "Repository indexing duration in seconds",
			Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600},
		},
		[]string{"status"}, // success, failed
	)

	repositorySyncDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "codesearch_repository_sync_duration_seconds",
			Help:    "Repository sync duration in seconds",
			Buckets: []float64{0.5, 1, 5, 10, 30, 60, 120},
		},
		[]string{"status"},
	)

	// Job metrics.
	jobsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "codesearch_jobs_total",
			Help: "Total number of jobs processed",
		},
		[]string{"type", "status"},
	)

	jobDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "codesearch_job_duration_seconds",
			Help:    "Job processing duration in seconds",
			Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600, 1800},
		},
		[]string{"type"},
	)

	jobsInQueue = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "codesearch_jobs_in_queue",
			Help: "Number of jobs in queue by type",
		},
		[]string{"type"},
	)

	// Replace metrics.
	replaceOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "codesearch_replace_operations_total",
			Help: "Total number of replace operations",
		},
		[]string{"status"}, // success, failed
	)

	replaceFilesModified = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "codesearch_replace_files_modified",
			Help:    "Number of files modified per replace operation",
			Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500},
		},
		[]string{"status"},
	)

	// Connection metrics.
	connectionsTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "codesearch_connections",
			Help: "Number of code host connections by type",
		},
		[]string{"type"},
	)

	// Database metrics.
	dbConnectionsOpen = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "codesearch_db_connections_open",
			Help: "Number of open database connections",
		},
	)

	dbConnectionsInUse = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "codesearch_db_connections_in_use",
			Help: "Number of database connections currently in use",
		},
	)

	// Redis metrics.
	redisConnectionsOpen = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "codesearch_redis_connections_open",
			Help: "Number of open Redis connections",
		},
	)

	// Zoekt metrics.
	zoektShardsTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "codesearch_zoekt_shards",
			Help: "Number of Zoekt shards",
		},
	)

	zoektIndexSizeBytes = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "codesearch_zoekt_index_size_bytes",
			Help: "Total size of Zoekt index in bytes",
		},
	)

	// Enhanced indexing observability metrics.
	repositorySizeBytes = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "codesearch_repository_size_bytes",
			Help: "Repository size in bytes before indexing",
			Buckets: []float64{
				1 << 20,   // 1MB
				10 << 20,  // 10MB
				100 << 20, // 100MB
				500 << 20, // 500MB
				1 << 30,   // 1GB
				2 << 30,   // 2GB
				5 << 30,   // 5GB
				10 << 30,  // 10GB
			},
		},
		[]string{"status"}, // success, failed
	)

	indexFailureReasons = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "codesearch_index_failures_total",
			Help: "Index failures categorized by reason",
		},
		[]string{"reason"}, // oom_killed, timeout, git_error, zoekt_error, unknown
	)

	orphanedTempFiles = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "codesearch_orphaned_temp_files",
			Help: "Number of orphaned .tmp index files",
		},
	)

	stuckIndexJobs = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "codesearch_stuck_index_jobs",
			Help: "Number of repos stuck in indexing state by duration threshold",
		},
		[]string{"duration"}, // 1h, 6h, 24h, 48h
	)

	activeIndexRepos = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "codesearch_active_index_repos",
			Help: "Number of repositories currently being indexed (in active set)",
		},
	)

	orphanedActiveMarkers = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "codesearch_orphaned_active_markers_total",
			Help: "Total number of orphaned active index markers recovered",
		},
	)
)

// Handler returns the Prometheus metrics handler.
func Handler() http.Handler {
	return promhttp.Handler()
}

// HTTPMiddleware is middleware that records HTTP metrics.
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		httpRequestsInFlight.Inc()
		defer httpRequestsInFlight.Dec()

		// Wrap response writer to capture status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rw, r)

		duration := time.Since(start).Seconds()
		path := normalizePath(r.URL.Path)

		httpRequestsTotal.WithLabelValues(r.Method, path, strconv.Itoa(rw.statusCode)).Inc()
		httpRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter

	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// normalizePath normalizes URL paths to reduce cardinality.
func normalizePath(path string) string {
	// Replace dynamic segments with placeholders
	// /api/repos/123 -> /api/repos/:id
	// /api/connections/456 -> /api/connections/:id
	segments := []string{}

	for _, seg := range splitPath(path) {
		if isNumeric(seg) {
			segments = append(segments, ":id")
		} else {
			segments = append(segments, seg)
		}
	}

	if len(segments) == 0 {
		return "/"
	}

	return "/" + joinPath(segments)
}

func splitPath(path string) []string {
	var segments []string

	for _, s := range []byte(path) {
		if s == '/' {
			continue
		}

		if len(segments) == 0 || path[len(path)-1] == '/' {
			segments = append(segments, "")
		}
	}
	// Simple split
	result := []string{}
	current := ""

	for _, c := range path {
		if c == '/' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}

	if current != "" {
		result = append(result, current)
	}

	return result
}

func joinPath(segments []string) string {
	result := ""

	var resultSb378 strings.Builder

	for i, s := range segments {
		if i > 0 {
			resultSb378.WriteString("/")
		}

		resultSb378.WriteString(s)
	}

	result += resultSb378.String()

	return result
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}

	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}

	return true
}

// RecordSearch records search metrics.
func RecordSearch(searchType string, duration time.Duration, resultCount int) {
	searchRequestsTotal.WithLabelValues(searchType).Inc()
	searchDuration.WithLabelValues(searchType).Observe(duration.Seconds())
	searchResultsCount.WithLabelValues(searchType).Observe(float64(resultCount))
}

// RecordRepoIndex records repository indexing metrics.
func RecordRepoIndex(duration time.Duration, success bool) {
	status := "success"
	if !success {
		status = "failed"
	}

	repositoryIndexDuration.WithLabelValues(status).Observe(duration.Seconds())
}

// RecordRepoSync records repository sync metrics.
func RecordRepoSync(duration time.Duration, success bool) {
	status := "success"
	if !success {
		status = "failed"
	}

	repositorySyncDuration.WithLabelValues(status).Observe(duration.Seconds())
}

// RecordJob records job processing metrics.
func RecordJob(jobType string, duration time.Duration, success bool) {
	status := "success"
	if !success {
		status = "failed"
	}

	jobsTotal.WithLabelValues(jobType, status).Inc()
	jobDuration.WithLabelValues(jobType).Observe(duration.Seconds())
}

// RecordReplace records replace operation metrics.
func RecordReplace(filesModified int, success bool) {
	status := "success"
	if !success {
		status = "failed"
	}

	replaceOperationsTotal.WithLabelValues(status).Inc()
	replaceFilesModified.WithLabelValues(status).Observe(float64(filesModified))
}

// SetRepositoryCounts sets the repository count gauges.
func SetRepositoryCounts(counts map[string]int) {
	for status, count := range counts {
		repositoriesTotal.WithLabelValues(status).Set(float64(count))
	}
}

// SetConnectionCounts sets the connection count gauges.
func SetConnectionCounts(counts map[string]int) {
	for connType, count := range counts {
		connectionsTotal.WithLabelValues(connType).Set(float64(count))
	}
}

// SetJobQueueCounts sets the job queue count gauges.
func SetJobQueueCounts(counts map[string]int) {
	for jobType, count := range counts {
		jobsInQueue.WithLabelValues(jobType).Set(float64(count))
	}
}

// SetDBConnections sets the database connection gauges.
func SetDBConnections(open, inUse int) {
	dbConnectionsOpen.Set(float64(open))
	dbConnectionsInUse.Set(float64(inUse))
}

// SetRedisConnections sets the Redis connection gauge.
func SetRedisConnections(open int) {
	redisConnectionsOpen.Set(float64(open))
}

// SetZoektStats sets the Zoekt stats gauges.
func SetZoektStats(shards int, indexSizeBytes int64) {
	zoektShardsTotal.Set(float64(shards))
	zoektIndexSizeBytes.Set(float64(indexSizeBytes))
}

// RecordRateLimit records rate limiting metrics.
func RecordRateLimit(allowed bool) {
	result := "allowed"
	if !allowed {
		result = "rejected"
	}

	rateLimitRequestsTotal.WithLabelValues(result).Inc()
}

// SetRateLimitBuckets sets the number of active rate limit buckets.
func SetRateLimitBuckets(count int) {
	rateLimitBucketsActive.Set(float64(count))
}

// RecordCodeHostRequest records code host API metrics.
func RecordCodeHostRequest(hostType, endpoint string, duration time.Duration, success bool) {
	status := "success"
	if !success {
		status = "error"
	}

	codeHostRequestsTotal.WithLabelValues(hostType, endpoint, status).Inc()
	codeHostRequestDuration.WithLabelValues(hostType, endpoint).Observe(duration.Seconds())
}

// SetCodeHostRateLimit sets the code host rate limit gauges.
func SetCodeHostRateLimit(hostType string, remaining int, resetTime time.Time) {
	codeHostRateLimitRemaining.WithLabelValues(hostType).Set(float64(remaining))
	codeHostRateLimitReset.WithLabelValues(hostType).Set(float64(resetTime.Unix()))
}

// RecordGitClone records git clone operation metrics.
func RecordGitClone(duration time.Duration, success bool) {
	status := "success"
	if !success {
		status = "failed"
	}

	gitCloneDuration.WithLabelValues(status).Observe(duration.Seconds())
	gitOperationsTotal.WithLabelValues("clone", status).Inc()
}

// RecordGitFetch records git fetch operation metrics.
func RecordGitFetch(duration time.Duration, success bool) {
	status := "success"
	if !success {
		status = "failed"
	}

	gitFetchDuration.WithLabelValues(status).Observe(duration.Seconds())
	gitOperationsTotal.WithLabelValues("fetch", status).Inc()
}

// RecordBranchesIndexed records branch indexing metrics.
func RecordBranchesIndexed(branchCount int, success bool) {
	status := "success"
	if !success {
		status = "failed"
	}

	for range branchCount {
		branchesIndexedTotal.WithLabelValues(status).Inc()
	}

	branchesPerRepo.WithLabelValues().Observe(float64(branchCount))
}

// RecordError records an error by category and type.
func RecordError(category, errorType string) {
	errorsTotal.WithLabelValues(category, errorType).Inc()
}

// RecordRepoSize records repository size before indexing.
func RecordRepoSize(sizeBytes int64, success bool) {
	status := "success"
	if !success {
		status = "failed"
	}

	repositorySizeBytes.WithLabelValues(status).Observe(float64(sizeBytes))
}

// RecordIndexFailure records an index failure with categorized reason.
// Reasons: oom_killed, timeout, git_error, zoekt_error, unknown.
func RecordIndexFailure(reason string) {
	indexFailureReasons.WithLabelValues(reason).Inc()
}

// SetOrphanedTempFiles sets the current count of orphaned .tmp files.
func SetOrphanedTempFiles(count int) {
	orphanedTempFiles.Set(float64(count))
}

// SetStuckIndexJobs sets the count of repos stuck in indexing state by duration.
func SetStuckIndexJobs(duration string, count int) {
	stuckIndexJobs.WithLabelValues(duration).Set(float64(count))
}

// SetActiveIndexRepos sets the number of repos currently in the active index set.
func SetActiveIndexRepos(count int) {
	activeIndexRepos.Set(float64(count))
}

// RecordOrphanedActiveMarkerRecovered increments the counter when an orphaned active marker is cleaned up.
func RecordOrphanedActiveMarkerRecovered() {
	orphanedActiveMarkers.Inc()
}
