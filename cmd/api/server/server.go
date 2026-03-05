package server

import (
	"context"
	_ "embed"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/techquestsdev/code-search/cmd/api/handlers"
	"github.com/techquestsdev/code-search/internal/audit"
	"github.com/techquestsdev/code-search/internal/config"
	"github.com/techquestsdev/code-search/internal/metrics"
	authmw "github.com/techquestsdev/code-search/internal/middleware"
	"github.com/techquestsdev/code-search/internal/ratelimit"
	"github.com/techquestsdev/code-search/internal/scip"
	"github.com/techquestsdev/code-search/internal/search"
	"github.com/techquestsdev/code-search/internal/tracing"
)

//go:embed openapi.yaml
var openapiSpec []byte

// Swagger UI HTML template.
const swaggerUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Code Search API - Documentation</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
    <style>
        body { margin: 0; padding: 0; }
        .topbar { display: none; }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script>
        window.onload = function() {
            SwaggerUIBundle({
                url: "/openapi.yaml",
                dom_id: '#swagger-ui',
                presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
                layout: "BaseLayout",
                deepLinking: true,
                showExtensions: true,
                showCommonExtensions: true
            });
        };
    </script>
</body>
</html>`

// newRouter creates the chi.Router with all middleware and routes configured.
// This is extracted so that Builder.Build() can add extra routes before wrapping
// the router in an *http.Server.
func newRouter(
	cfg *config.Config,
	services *handlers.Services,
	scipSvc *scip.Service,
	logger *zap.Logger,
) chi.Router {
	r := chi.NewRouter()
	h := handlers.NewHandlerWithOptions(services, handlers.HandlerOptions{
		ConnectionsReadOnly: cfg.ConnectionsReadOnly,
		ReposReadOnly:       cfg.ReposReadOnly,
		SchedulerConfig: handlers.SchedulerConfig{
			Enabled:       cfg.Scheduler.Enabled,
			PollInterval:  cfg.Scheduler.PollInterval,
			CheckInterval: cfg.Scheduler.CheckInterval,
		},
		UIConfig: handlers.UIConfig{
			HideReadOnlyBanner:  cfg.UI.HideReadOnlyBanner,
			HideFileNavigator:   cfg.UI.HideFileNavigator,
			DisableBrowseAPI:    cfg.UI.DisableBrowseAPI,
			HideReposPage:       cfg.UI.HideReposPage,
			HideConnectionsPage: cfg.UI.HideConnectionsPage,
			HideJobsPage:        cfg.UI.HideJobsPage,
			HideReplacePage:     cfg.UI.HideReplacePage,
			AuthEnabled:         !isNoOpAuthenticator(services.Authenticator),
		},
		SearchConfig: handlers.SearchConfig{
			EnableStreaming: cfg.Search.EnableStreaming,
		},
	})

	// Create SCIP handler if service is available
	var scipHandler *handlers.SCIPHandler
	if scipSvc != nil {
		scipHandler = handlers.NewSCIPHandler(services, scipSvc)
	}

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(corsMiddleware)
	r.Use(securityHeaders)

	// Add authentication middleware (no-op by default, enterprise can provide SSO)
	r.Use(authmw.AuthMiddleware(services.Authenticator))

	// Add OpenTelemetry tracing middleware if enabled
	if cfg.Tracing.Enabled {
		r.Use(tracing.HTTPMiddleware)
	}

	// Add Prometheus metrics middleware if enabled
	if cfg.Metrics.Enabled {
		r.Use(metrics.HTTPMiddleware)
	}

	// Add rate limiting middleware if enabled
	if cfg.RateLimit.Enabled {
		r.Use(ratelimit.MiddlewareFunc(cfg.RateLimit.RequestsPerSecond, cfg.RateLimit.BurstSize))
	}

	// Health check (liveness - don't need services)
	r.Get("/health", handlers.Health)
	// Readiness check (uses handler to check dependencies)
	r.Get("/ready", h.Ready)

	// Prometheus metrics endpoint
	if cfg.Metrics.Enabled {
		metricsPath := cfg.Metrics.Path
		if metricsPath == "" {
			metricsPath = "/metrics"
		}

		r.Handle(metricsPath, promhttp.Handler())
	}

	// OpenAPI spec and docs
	r.Get("/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.Write(openapiSpec)
	})
	r.Get("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Redirect to YAML - could convert if needed
		http.Redirect(w, r, "/openapi.yaml", http.StatusTemporaryRedirect)
	})
	r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(swaggerUIHTML))
	})

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		// Search endpoints
		r.Route("/search", func(r chi.Router) {
			r.Post("/", h.Search)
			r.Post("/stream", h.SearchStream)
			r.Get("/suggestions", h.SearchSuggestions)
		})

		// Repository management - use ID-based routes to avoid slash issues in repo names
		r.Route("/repos", func(r chi.Router) {
			r.Get("/status", h.ReposStatus)
			r.Get("/stats", h.GetRepoStats)
			r.Get("/", h.ListRepos)
			r.Post("/", h.AddRepo)
			r.Get("/lookup", h.LookupRepoByName) // Lookup by Zoekt name
			r.Get("/by-id/{id}", h.GetRepoByID)
			r.Delete("/by-id/{id}", h.DeleteRepoByID)
			r.Post("/by-id/{id}/sync", h.SyncRepoByID)
			r.Put("/by-id/{id}/poll-interval", h.SetRepoPollInterval)
			r.Put("/by-id/{id}/branches", h.SetRepoBranches)
			r.Post("/by-id/{id}/exclude", h.ExcludeRepoByID)
			r.Post("/by-id/{id}/include", h.IncludeRepoByID)
			r.Post("/by-id/{id}/restore", h.RestoreRepoByID)
			// File browsing endpoints (can be disabled via config)
			if !cfg.UI.DisableBrowseAPI {
				r.Get("/by-id/{id}/tree", h.ListTree)
				r.Get("/by-id/{id}/blob", h.GetBlob)
				r.Get("/by-id/{id}/raw", h.GetRaw)
				r.Get("/by-id/{id}/refs", h.GetBranchesAndTags)
				r.Get("/by-id/{id}/symbols", h.GetFileSymbols)
			}
		})

		// Scheduler management
		r.Route("/scheduler", func(r chi.Router) {
			r.Get("/stats", h.GetSchedulerStats)
			r.Post("/sync-all", h.TriggerSyncAll)
			r.Post("/cleanup", h.CleanupStaleIndexing)
		})

		// Replace operations
		r.Route("/replace", func(r chi.Router) {
			r.Post("/preview", h.ReplacePreview)
			r.Post("/execute", h.ReplaceExecute)
			r.Get("/jobs", h.ListReplaceJobs)
			r.Get("/jobs/{id}", h.GetReplaceJob)
		})

		// Symbol navigation
		r.Route("/symbols", func(r chi.Router) {
			r.Post("/find", h.FindSymbols)
			r.Post("/refs", h.FindRefs)
		})

		// SCIP code intelligence (precise navigation)
		if scipHandler != nil {
			r.Route("/scip", func(r chi.Router) {
				r.Get("/indexers", scipHandler.GetAvailableIndexers)
				r.Route("/repos/{id}", func(r chi.Router) {
					r.Get("/status", scipHandler.GetSCIPStatus)
					r.Post("/definition", scipHandler.GoToDefinition)
					r.Post("/references", scipHandler.FindReferences)
					r.Post("/index", scipHandler.IndexRepository)
					r.Post("/upload", scipHandler.UploadSCIPIndex)
					r.Delete("/index", scipHandler.ClearSCIPIndex)
					// Debug/lookup endpoints
					r.Post("/symbols/search", scipHandler.SearchSymbols)
					r.Get("/files", scipHandler.ListFiles)
				})
			})
		}

		// Connections
		r.Route("/connections", func(r chi.Router) {
			r.Get("/status", h.GetConnectionsStatus)
			r.Get("/", h.ListConnections)
			r.Post("/", h.CreateConnection)
			r.Get("/{id}", h.GetConnection)
			r.Put("/{id}", h.UpdateConnection)
			r.Delete("/{id}", h.DeleteConnection)
			r.Post("/{id}/test", h.TestConnection)
			r.Post("/{id}/sync", h.SyncConnection)
			r.Get("/{id}/repos", h.ListConnectionRepos)
			r.Get("/{id}/stats", h.GetConnectionStats)
		})

		// Jobs queue
		r.Route("/jobs", func(r chi.Router) {
			r.Get("/stats", h.GetQueueStats)
			r.Get("/", h.ListJobs)
			r.Get("/{id}", h.GetJob)
			r.Post("/{id}/cancel", h.CancelJob)
			r.Post("/cleanup", h.CleanupJobs)
			r.Post("/cancel-all", h.BulkCancelJobs)
			r.Post("/delete-all", h.BulkDeleteJobs)
			r.Post("/rebuild-indexes", h.RebuildJobIndexes)
			r.Delete("/", h.DeleteAllJobs)
		})

		// Webhooks
		r.Post("/webhooks/{provider}", h.HandleWebhook)

		// UI settings
		r.Get("/ui/settings", h.GetUISettings)
	})

	return r
}

// New creates a new HTTP server with all routes configured.
func New(
	cfg *config.Config,
	services *handlers.Services,
	scipSvc *scip.Service,
	logger *zap.Logger,
) *http.Server {
	r := newRouter(cfg, services, scipSvc, logger)

	return &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      r,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

func isNoOpAuthenticator(a authmw.Authenticator) bool {
	_, ok := a.(*authmw.NoOpAuthenticator)
	return ok
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// When an Origin header is present, echo it back and allow credentials
		// so that fetch with credentials: "include" works (needed for session cookies).
		// Wildcard "*" is not allowed when credentials mode is "include".
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// Builder provides a fluent API for constructing the HTTP server with
// optional enterprise extension points. This is the primary integration
// surface for the enterprise repository.
type Builder struct {
	cfg         *config.Config
	services    *handlers.Services
	scipSvc     *scip.Service
	logger      *zap.Logger
	routeAdders []func(chi.Router)
}

// NewBuilder creates a server builder with the required dependencies.
func NewBuilder(cfg *config.Config, services *handlers.Services, logger *zap.Logger) *Builder {
	return &Builder{
		cfg:      cfg,
		services: services,
		logger:   logger,
	}
}

// WithSCIP sets the SCIP service for precise code navigation.
func (b *Builder) WithSCIP(svc *scip.Service) *Builder {
	b.scipSvc = svc
	return b
}

// WithAuthenticator sets a custom authenticator (e.g., SAML, OIDC).
// If not called, the default NoOpAuthenticator is used.
func (b *Builder) WithAuthenticator(auth authmw.Authenticator) *Builder {
	b.services.Authenticator = auth
	return b
}

// WithAuthorizer sets a custom authorizer (e.g., RBAC).
// If not called, the default NoOpAuthorizer is used.
func (b *Builder) WithAuthorizer(authz authmw.Authorizer) *Builder {
	b.services.Authorizer = authz
	return b
}

// WithAuditLogger sets a custom audit logger for compliance logging.
// If not called, the default NoOpAuditLogger is used.
func (b *Builder) WithAuditLogger(al audit.AuditLogger) *Builder {
	b.services.AuditLogger = al
	return b
}

// WithSearchResultFilter sets a function to filter search results based on
// user access. This is a convenience that wraps authorizer-level filtering
// into the search pipeline.
func (b *Builder) WithSearchResultFilter(fn func(ctx context.Context, results []search.SearchResult) []search.SearchResult) *Builder {
	b.services.SearchResultFilter = fn
	return b
}

// WithRoutes adds a function that registers additional routes on the router.
// This allows enterprise to add auth, admin, and other routes to the same
// server without modifying the core router setup.
func (b *Builder) WithRoutes(fn func(chi.Router)) *Builder {
	b.routeAdders = append(b.routeAdders, fn)
	return b
}

// Build creates the configured HTTP server.
func (b *Builder) Build() *http.Server {
	r := newRouter(b.cfg, b.services, b.scipSvc, b.logger)

	for _, fn := range b.routeAdders {
		fn(r)
	}

	return &http.Server{
		Addr:         b.cfg.Server.Addr,
		Handler:      r,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}
