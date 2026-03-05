package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/aanogueira/code-search/internal/audit"
	"github.com/aanogueira/code-search/internal/crypto"
	"github.com/aanogueira/code-search/internal/db"
	"github.com/aanogueira/code-search/internal/files"
	"github.com/aanogueira/code-search/internal/log"
	"github.com/aanogueira/code-search/internal/middleware"
	"github.com/aanogueira/code-search/internal/queue"
	"github.com/aanogueira/code-search/internal/replace"
	"github.com/aanogueira/code-search/internal/repos"
	"github.com/aanogueira/code-search/internal/scip"
	"github.com/aanogueira/code-search/internal/search"
	"github.com/aanogueira/code-search/internal/symbols"
)

// Services holds all service dependencies for handlers.
type Services struct {
	Search         *search.Service
	Repos          *repos.Service
	Files          *files.Service
	FederatedFiles *files.FederatedClient // For sharded deployments
	FederatedSCIP  *scip.FederatedClient  // For sharded SCIP access
	Symbols        *symbols.Service
	Replace        *replace.Service
	Queue          *queue.Queue
	Pool           db.Pool
	Redis          *redis.Client
	Logger         *zap.Logger
	IndexPath      string // Path to Zoekt index shards
	ReposPath      string // Path to cloned repositories

	// Extension points for enterprise features.
	// These default to no-op implementations in the open-source core.
	Authenticator      middleware.Authenticator
	Authorizer         middleware.Authorizer
	AuditLogger        audit.AuditLogger
	SearchResultFilter func(ctx context.Context, results []search.SearchResult) []search.SearchResult // Optional enterprise result filtering
}

// NewServices creates all services with their dependencies.
func NewServices(
	pool db.Pool,
	redisClient *redis.Client,
	zoektURL, indexPath, reposPath string,
	tokenEncryptor *crypto.TokenEncryptor,
	logger *zap.Logger,
) *Services {
	// Check if sharding is enabled via environment or multiple URLs
	var searchSvc *search.Service
	if zoektShards := os.Getenv("ZOEKT_SHARDS"); zoektShards != "" {
		// Use explicit shard URLs from environment
		searchSvc = search.NewShardedService(zoektShards)
	} else if strings.Contains(zoektURL, ",") {
		// URL contains multiple comma-separated addresses
		searchSvc = search.NewShardedService(zoektURL)
	} else {
		// Single Zoekt instance
		searchSvc = search.NewService(zoektURL)
	}

	repoSvc := repos.NewService(pool, tokenEncryptor)
	filesSvc := files.NewService(reposPath)
	queueSvc := queue.NewQueue(redisClient)
	replaceSvc := replace.NewService(searchSvc, repoSvc, "", nil, nil) // nil uses defaults

	// Initialize symbols service with cache in data/symbols directory
	symbolsCacheDir := filepath.Join(filepath.Dir(reposPath), "symbols")

	symbolsSvc, err := symbols.NewService(symbolsCacheDir)
	if err != nil {
		logger.Warn("Failed to initialize symbols cache, using uncached mode", zap.Error(err))

		symbolsSvc = symbols.NewServiceWithoutCache()
	}

	return &Services{
		Search:         searchSvc,
		Repos:          repoSvc,
		Files:          filesSvc,
		FederatedFiles: nil, // Set externally if sharding is enabled
		Symbols:        symbolsSvc,
		Replace:        replaceSvc,
		Queue:          queueSvc,
		Pool:           pool,
		Redis:          redisClient,
		Logger:         logger,
		IndexPath:      indexPath,
		ReposPath:      reposPath,
		Authenticator:  &middleware.NoOpAuthenticator{},
		Authorizer:     &middleware.NoOpAuthorizer{},
		AuditLogger:    &audit.NoOpAuditLogger{},
	}
}

// SchedulerConfig holds scheduler configuration for handlers.
type SchedulerConfig struct {
	Enabled       bool
	PollInterval  time.Duration
	CheckInterval time.Duration
}

// UIConfig holds UI display settings.
type UIConfig struct {
	HideReadOnlyBanner  bool
	HideFileNavigator   bool
	DisableBrowseAPI    bool
	HideReposPage       bool
	HideConnectionsPage bool
	HideJobsPage        bool
	HideReplacePage     bool
	AuthEnabled         bool
}

// SearchConfig holds search behavior settings.
type SearchConfig struct {
	EnableStreaming bool // Enable true streaming from Zoekt
}

// Handler provides access to services for HTTP handlers.
type Handler struct {
	services            *Services
	connectionsReadOnly bool
	reposReadOnly       bool
	schedulerConfig     SchedulerConfig
	uiConfig            UIConfig
	searchConfig        SearchConfig
}

// HandlerOptions contains options for creating a handler.
type HandlerOptions struct {
	ConnectionsReadOnly bool
	ReposReadOnly       bool
	SchedulerConfig     SchedulerConfig
	UIConfig            UIConfig
	SearchConfig        SearchConfig
}

// NewHandlerWithOptions creates a new handler with services and options.
func NewHandlerWithOptions(services *Services, opts HandlerOptions) *Handler {
	return &Handler{
		services:            services,
		connectionsReadOnly: opts.ConnectionsReadOnly,
		reposReadOnly:       opts.ReposReadOnly,
		schedulerConfig:     opts.SchedulerConfig,
		uiConfig:            opts.UIConfig,
		searchConfig:        opts.SearchConfig,
	}
}

// writeJSON writes a JSON response with proper error handling.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")

	err := json.NewEncoder(w).Encode(v)
	if err != nil {
		// Can't write error response as headers may already be sent
		log.L.Debug("Failed to encode JSON response", zap.Error(err))
	}
}

// writeJSONWithStatus writes a JSON response with a specific status code.
func writeJSONWithStatus(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	err := json.NewEncoder(w).Encode(v)
	if err != nil {
		log.L.Debug("Failed to encode JSON response", zap.Error(err))
	}
}

// UISettingsResponse contains UI settings for the frontend.
type UISettingsResponse struct {
	HideReadOnlyBanner  bool `json:"hide_readonly_banner"`
	HideFileNavigator   bool `json:"hide_file_navigator"`
	DisableBrowseAPI    bool `json:"disable_browse_api"`
	ConnectionsReadOnly bool `json:"connections_readonly"`
	ReposReadOnly       bool `json:"repos_readonly"`
	EnableStreaming     bool `json:"enable_streaming"`
	HideReposPage       bool `json:"hide_repos_page"`
	HideConnectionsPage bool `json:"hide_connections_page"`
	HideJobsPage        bool `json:"hide_jobs_page"`
	HideReplacePage     bool `json:"hide_replace_page"`
	AuthEnabled         bool `json:"auth_enabled"`
}

// GetUISettings returns UI settings for the frontend.
func (h *Handler) GetUISettings(w http.ResponseWriter, r *http.Request) {
	response := UISettingsResponse{
		HideReadOnlyBanner:  h.uiConfig.HideReadOnlyBanner,
		HideFileNavigator:   h.uiConfig.HideFileNavigator,
		DisableBrowseAPI:    h.uiConfig.DisableBrowseAPI,
		ConnectionsReadOnly: h.connectionsReadOnly,
		ReposReadOnly:       h.reposReadOnly,
		EnableStreaming:     h.searchConfig.EnableStreaming,
		HideReposPage:       h.uiConfig.HideReposPage,
		HideConnectionsPage: h.uiConfig.HideConnectionsPage,
		HideJobsPage:        h.uiConfig.HideJobsPage,
		HideReplacePage:     h.uiConfig.HideReplacePage,
		AuthEnabled:         h.uiConfig.AuthEnabled,
	}
	writeJSON(w, response)
}
