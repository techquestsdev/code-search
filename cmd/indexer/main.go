package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"github.com/techquestsdev/code-search/internal/config"
	"github.com/techquestsdev/code-search/internal/crypto"
	"github.com/techquestsdev/code-search/internal/db"
	"github.com/techquestsdev/code-search/internal/files"
	"github.com/techquestsdev/code-search/internal/indexer"
	"github.com/techquestsdev/code-search/internal/log"
	"github.com/techquestsdev/code-search/internal/queue"
	"github.com/techquestsdev/code-search/internal/scip"
)

func main() {
	// Load .env files
	_ = godotenv.Load(".env.local", ".env")

	// Initialize logger
	if err := log.InitFromEnv(); err != nil {
		panic("failed to initialize logger: " + err.Error())
	}

	defer log.Sync()

	// Load secrets from mounted directories (e.g., Kubernetes secrets)
	// This allows tokens like "$GITHUB_TOKEN" to be resolved from secret files
	secretsPaths := config.DefaultSecretsPaths()

	if customPath := os.Getenv("CS_SECRETS_PATH"); customPath != "" {
		var customPaths []string
		if strings.Contains(customPath, ",") {
			customPaths = strings.Split(customPath, ",")
		} else if strings.Contains(customPath, ":") {
			customPaths = strings.Split(customPath, ":")
		} else {
			customPaths = []string{customPath}
		}

		secretsPaths = append(customPaths, secretsPaths...)
	}

	if err := config.LoadSecretsFromPaths(secretsPaths, "", false); err != nil {
		log.Warn("Failed to load secrets from paths", log.Err(err))
	}

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

	// Connect to database using driver abstraction
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

	// Create job queue
	jobQueue := queue.NewQueue(redisClient)

	// Create worker
	worker := indexer.NewWorker(cfg, log.L, jobQueue, dbPool, tokenEncryptor)

	// Initialize SCIP service for automatic code intelligence indexing
	var scipSvc *scip.Service

	if cfg.SCIP.Enabled {
		scipCacheDir := cfg.SCIP.CacheDir
		if scipCacheDir == "" {
			scipCacheDir = filepath.Join(filepath.Dir(cfg.Indexer.ReposPath), "scip")
		}

		// Build SCIP IndexerConfig from app config
		indexerCfg := scip.DefaultIndexerConfig()
		if cfg.SCIP.Timeout > 0 {
			indexerCfg.Timeout = cfg.SCIP.Timeout
		}

		if cfg.SCIP.WorkDir != "" {
			indexerCfg.WorkDir = cfg.SCIP.WorkDir
		}

		// Apply per-language binary paths from config
		if cfg.SCIP.Languages != nil {
			for lang, langCfg := range cfg.SCIP.Languages {
				if langCfg.BinaryPath == "" {
					continue
				}

				switch lang {
				case "go":
					indexerCfg.SCIPGo = langCfg.BinaryPath
				case "typescript", "javascript":
					indexerCfg.SCIPTypeScript = langCfg.BinaryPath
				case "java":
					indexerCfg.SCIPJava = langCfg.BinaryPath
				case "rust":
					indexerCfg.SCIPRust = langCfg.BinaryPath
				case "python":
					indexerCfg.SCIPPython = langCfg.BinaryPath
				}
			}
		}

		svc, scipErr := scip.NewServiceWithConfig(
			scipCacheDir,
			indexerCfg,
			log.L.With(zap.String("component", "scip")),
		)
		if scipErr != nil {
			log.Warn("Failed to initialize SCIP service, auto-indexing disabled", log.Err(scipErr))
		} else {
			scipSvc = svc
			worker.SetSCIPService(scipSvc)

			// Log available indexers for visibility
			available := scipSvc.GetAvailableIndexers()

			var enabledLangs []string

			for lang, avail := range available {
				if avail {
					enabledLangs = append(enabledLangs, lang)
				}
			}

			log.Info("SCIP service initialized",
				log.String("cache_dir", scipCacheDir),
				log.Bool("auto_index", cfg.SCIP.AutoIndex),
			)

			if len(enabledLangs) > 0 {
				log.Info("Available SCIP indexers", log.String("languages", strings.Join(enabledLangs, ", ")))
			} else {
				log.Warn("No SCIP indexer binaries found in PATH")
			}
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Info("Received shutdown signal, initiating graceful shutdown...")
		log.Info("Workers will complete current jobs before stopping (up to 5 minutes per job)")
		cancel()
	}()

	// Ensure directories exist
	if err := os.MkdirAll(cfg.Indexer.ReposPath, 0o755); err != nil {
		log.Fatal("Failed to create repos directory", log.Err(err))
	}

	if err := os.MkdirAll(cfg.Indexer.IndexPath, 0o755); err != nil {
		log.Fatal("Failed to create index directory", log.Err(err))
	}

	// Log shard configuration
	shardIndex := os.Getenv("SHARD_INDEX")

	totalShards := os.Getenv("TOTAL_SHARDS")
	if totalShards != "" && shardIndex != "" {
		log.Info("Sharding enabled",
			log.String("shard_index", shardIndex),
			log.String("total_shards", totalShards),
		)
	} else {
		log.Info("Running in single-instance mode (no sharding)")
	}

	log.Info("Starting indexer worker",
		log.Int("concurrency", cfg.Indexer.Concurrency),
		log.String("index_path", cfg.Indexer.IndexPath),
		log.String("repos_path", cfg.Indexer.ReposPath),
	)

	// Start indexer HTTP server for federated access
	shardCfg := indexer.GetShardConfig()

	apiPort := cfg.Sharding.IndexerAPIPort
	if apiPort == 0 {
		apiPort = 8081
	}

	// Also allow override via environment variable
	if portStr := os.Getenv("INDEXER_API_PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			apiPort = p
		}
	}

	filesService := files.NewService(cfg.Indexer.ReposPath)

	srv := indexer.NewServer(
		indexer.ServerConfig{
			Addr:        fmt.Sprintf(":%d", apiPort),
			ReposPath:   cfg.Indexer.ReposPath,
			ShardIndex:  shardCfg.ShardIndex,
			TotalShards: shardCfg.TotalShards,
		},
		log.L,
		filesService,
		nil, // replace service created on-demand by worker
		scipSvc,
		jobQueue,
	)

	var wg sync.WaitGroup

	// Start HTTP server in background

	wg.Go(func() {
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("Indexer HTTP server error", log.Err(err))
		}
	})

	// Shutdown server on context cancellation
	go func() {
		<-ctx.Done()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Error("Indexer HTTP server shutdown error", log.Err(err))
		}
	}()

	// Start multiple workers based on concurrency setting
	concurrency := max(cfg.Indexer.Concurrency, 1)

	for i := range concurrency {
		wg.Add(1)

		workerNum := i + 1

		go func() {
			defer wg.Done()

			log.Info("Starting worker goroutine", log.Int("worker_num", workerNum))

			err := worker.Run(ctx)
			if err != nil {
				log.Error("Worker error", log.Int("worker_num", workerNum), log.Err(err))
			}
		}()
	}

	// Wait for all workers and HTTP server to complete
	wg.Wait()

	log.Info("Worker stopped")
}
