package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"github.com/aanogueira/code-search/cmd/api/handlers"
	"github.com/aanogueira/code-search/cmd/api/server"
	"github.com/aanogueira/code-search/internal/cache"
	"github.com/aanogueira/code-search/internal/config"
	"github.com/aanogueira/code-search/internal/crypto"
	"github.com/aanogueira/code-search/internal/db"
	"github.com/aanogueira/code-search/internal/files"
	"github.com/aanogueira/code-search/internal/log"
	"github.com/aanogueira/code-search/internal/metrics"
	"github.com/aanogueira/code-search/internal/queue"
	"github.com/aanogueira/code-search/internal/repos"
	"github.com/aanogueira/code-search/internal/scheduler"
	"github.com/aanogueira/code-search/internal/scip"
	"github.com/aanogueira/code-search/internal/tracing"
)

func main() {
	// Load .env files (optional, won't error if not found)
	_ = godotenv.Load(".env.local", ".env")

	// Initialize logger
	if err := log.InitFromEnv(); err != nil {
		panic("failed to initialize logger: " + err.Error())
	}

	defer log.Sync()

	// Load secrets from mounted directories (e.g., Kubernetes secrets)
	// This allows tokens like "$GITHUB_TOKEN" to be resolved from secret files
	// Check CS_SECRETS_PATH env var for custom path(s), otherwise use defaults
	secretsPaths := config.DefaultSecretsPaths()

	if customPath := os.Getenv("CS_SECRETS_PATH"); customPath != "" {
		// Custom path can be comma-separated or colon-separated for multiple paths
		var customPaths []string
		if strings.Contains(customPath, ",") {
			customPaths = strings.Split(customPath, ",")
		} else if strings.Contains(customPath, ":") {
			customPaths = strings.Split(customPath, ":")
		} else {
			customPaths = []string{customPath}
		}
		// Prepend custom paths (higher priority)
		secretsPaths = append(customPaths, secretsPaths...)
	}
	// Load secrets without prefix (so GITHUB_TOKEN file becomes GITHUB_TOKEN env var)
	// Don't overwrite existing env vars (explicit env vars take precedence)
	if err := config.LoadSecretsFromPaths(secretsPaths, "", false); err != nil {
		log.Warn("Failed to load secrets from paths", log.Err(err))
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config", log.Err(err))
	}

	// Initialize token encryptor for encrypting tokens at rest
	tokenEncryptor, err := crypto.NewTokenEncryptor(cfg.Security.EncryptionKey)
	if err != nil {
		log.Fatal("Failed to create token encryptor", log.Err(err))
	}

	if tokenEncryptor.IsActive() {
		log.Info("Token encryption enabled")
	}

	// Initialize tracing (before other services for proper span propagation)
	tracingCfg := &tracing.Config{
		Enabled:        cfg.Tracing.Enabled,
		ServiceName:    cfg.Tracing.ServiceName,
		ServiceVersion: cfg.Tracing.ServiceVersion,
		Environment:    cfg.Tracing.Environment,
		Endpoint:       cfg.Tracing.Endpoint,
		Protocol:       cfg.Tracing.Protocol,
		SampleRate:     cfg.Tracing.SampleRate,
		Insecure:       cfg.Tracing.Insecure,
	}

	tracingProvider, err := tracing.InitTracing(context.Background(), tracingCfg, log.L)
	if err != nil {
		log.Error("Failed to initialize tracing", log.Err(err))
		// Continue without tracing - not fatal
	}

	if tracingProvider != nil {
		defer tracingProvider.Shutdown(context.Background())
	}

	// Connect to database using driver abstraction
	// Supports both PostgreSQL and MySQL based on URL or explicit driver config
	dbPool, err := db.ConnectWithDriver(context.Background(), cfg.Database.URL, cfg.Database.Driver)
	if err != nil {
		log.Fatal("Failed to connect to database", log.Err(err))
	}
	defer dbPool.Close()

	log.Info("Connected to database", log.String("driver", string(dbPool.Driver())))

	// Connect to Redis
	redisTLSCfg := &queue.TLSConfig{
		Enabled:    cfg.Redis.TLSEnabled,
		SkipVerify: cfg.Redis.TLSSkipVerify,
		CertFile:   cfg.Redis.TLSCertFile,
		KeyFile:    cfg.Redis.TLSKeyFile,
		CACertFile: cfg.Redis.TLSCACertFile,
		ServerName: cfg.Redis.TLSServerName,
	}

	redisClient, err := queue.ConnectWithTLS(
		cfg.Redis.Addr,
		cfg.Redis.Password,
		cfg.Redis.DB,
		redisTLSCfg,
	)
	if err != nil {
		log.Fatal("Failed to connect to Redis", log.Err(err))
	}
	defer redisClient.Close()

	log.Info("Connected to Redis", log.Bool("tls", cfg.Redis.TLSEnabled))

	// Initialize queue
	q := queue.NewQueue(redisClient)

	// Sync code hosts from config to database and queue sync jobs
	// Config-defined code hosts take precedence over UI-created ones
	if len(cfg.CodeHosts) > 0 {
		repoService := repos.NewService(dbPool, tokenEncryptor)

		codeHostConfigs := make(map[string]repos.CodeHostConfig)
		for name, ch := range cfg.CodeHosts {
			codeHostConfigs[name] = repos.CodeHostConfig{
				Type:            ch.Type,
				URL:             ch.URL,
				Token:           ch.Token,
				ExcludeArchived: ch.ExcludeArchived,
				CleanupArchived: ch.CleanupArchived,
				Repos:           ch.Repos,
			}
		}

		syncedIDs, err := repoService.SyncCodeHostsFromConfig(context.Background(), codeHostConfigs)
		if err != nil {
			log.Error("Failed to sync code hosts from config", log.Err(err))
			// Don't exit - this is not fatal, UI connections still work
		} else {
			log.Info("Synced code hosts from config", log.Int("count", len(cfg.CodeHosts)))

			// Queue sync jobs to discover repos for each config-defined connection
			for _, connID := range syncedIDs {
				// Mark job as active BEFORE enqueueing
				if err := q.MarkSyncJobActive(context.Background(), connID); err != nil {
					log.Warn("Failed to mark sync job active", log.Int64("connection_id", connID), log.Err(err))
				}

				_, err := q.Enqueue(context.Background(), queue.JobTypeSync, queue.SyncPayload{ConnectionID: connID})
				if err != nil {
					_ = q.MarkSyncJobInactive(context.Background(), connID)
					log.Error("Failed to queue sync job for connection",
						log.Int64("connection_id", connID),
						log.Err(err))
				} else {
					log.Info("Queued sync job for config connection", log.Int64("connection_id", connID))
				}
			}
		}
	}

	// Initialize services
	services := handlers.NewServices(
		dbPool,
		redisClient,
		cfg.Zoekt.URL,
		cfg.Indexer.IndexPath,
		cfg.Indexer.ReposPath,
		tokenEncryptor,
		log.L,
	)

	// Initialize federated access for sharded deployments
	if cfg.Sharding.Enabled && cfg.Sharding.FederatedAccess && cfg.Sharding.TotalShards > 1 {
		services.FederatedFiles = files.NewFederatedClient(
			cfg.Sharding.IndexerService,
			cfg.Sharding.IndexerAPIPort,
			cfg.Sharding.TotalShards,
		)
		services.FederatedSCIP = scip.NewFederatedClient(
			cfg.Sharding.IndexerService,
			cfg.Sharding.IndexerAPIPort,
			cfg.Sharding.TotalShards,
		)
		log.Info("Federated access enabled (files + SCIP)",
			log.String("indexer_service", cfg.Sharding.IndexerService),
			log.Int("total_shards", cfg.Sharding.TotalShards),
			log.Int("indexer_port", cfg.Sharding.IndexerAPIPort),
		)
	}

	// Initialize SCIP service for precise code navigation
	scipCacheDir := filepath.Join(filepath.Dir(cfg.Indexer.ReposPath), "scip")

	scipSvc, err := scip.NewService(scipCacheDir, log.L.With(zap.String("component", "scip")))
	if err != nil {
		log.Warn("Failed to initialize SCIP service, precise navigation disabled", log.Err(err))
	} else {
		defer scipSvc.Close()

		log.Info("SCIP service initialized", log.String("cache_dir", scipCacheDir))
	}

	// Start cache invalidation subscriber (Redis pub/sub)
	// Listens for repo re-index events published by indexers and evicts local caches
	invalidatorCtx, cancelInvalidator := context.WithCancel(context.Background())
	defer cancelInvalidator()

	cacheInvalidator := cache.NewInvalidator(redisClient, scipSvc, services.Symbols, log.L)
	go cacheInvalidator.Start(invalidatorCtx)

	// Initialize and start scheduler
	schedulerCfg := scheduler.Config{
		Enabled:               cfg.Scheduler.Enabled,
		DefaultPollInterval:   cfg.Scheduler.PollInterval,
		CheckInterval:         cfg.Scheduler.CheckInterval,
		StaleThreshold:        cfg.Scheduler.StaleThreshold,
		PendingJobTimeout:     cfg.Scheduler.PendingJobTimeout,
		MaxConcurrentChecks:   cfg.Scheduler.MaxConcurrentChecks,
		ReposPath:             cfg.Indexer.ReposPath,
		IndexPath:             cfg.Indexer.IndexPath,
		JobRetentionPeriod:    cfg.Scheduler.JobRetention,
		OrphanCleanupInterval: cfg.Scheduler.OrphanCleanupInterval,
	}

	syncScheduler := scheduler.New(schedulerCfg, dbPool, q, redisClient, tokenEncryptor, log.L)
	if scipSvc != nil {
		syncScheduler.SetSCIPService(scipSvc)
	}
	if err := syncScheduler.Start(context.Background()); err != nil {
		log.Fatal("Failed to start scheduler", log.Err(err))
	}
	defer syncScheduler.Stop()

	// Start metrics collector for infrastructure stats
	dbAdapter := &metrics.DBPoolAdapter{
		StatsFunc: func() metrics.PoolStats {
			stats := dbPool.Stats()

			return metrics.PoolStats{
				MaxOpenConnections: stats.MaxOpenConnections,
				OpenConnections:    stats.OpenConnections,
				InUse:              stats.InUse,
				Idle:               stats.Idle,
			}
		},
		QueryFunc: func(ctx context.Context, sql string, args ...any) (metrics.Rows, error) {
			return dbPool.Query(ctx, sql, args...)
		},
	}
	metricsCollector := metrics.NewCollector(
		dbAdapter,
		redisClient,
		cfg.Indexer.IndexPath,
		30*time.Second,
	)

	metricsCollector.Start(context.Background())
	defer metricsCollector.Stop()

	// Create server
	srv := server.New(cfg, services, scipSvc, log.L)

	// Start server in goroutine
	go func() {
		log.Info("Starting API server", log.String("addr", cfg.Server.Addr))

		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("Server error", log.Err(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", log.Err(err))
	}

	log.Info("Server stopped")
}
