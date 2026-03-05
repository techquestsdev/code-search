package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application.
type Config struct {
	Server              ServerConfig        `mapstructure:"server"`
	Database            DatabaseConfig      `mapstructure:"database"`
	Redis               RedisConfig         `mapstructure:"redis"`
	Zoekt               ZoektConfig         `mapstructure:"zoekt"`
	Indexer             IndexerConfig       `mapstructure:"indexer"`
	Repos               ReposConfig         `mapstructure:"repos"`
	Scheduler           SchedulerConfig     `mapstructure:"scheduler"`
	Replace             ReplaceConfig       `mapstructure:"replace"`
	Sharding            ShardingConfig      `mapstructure:"sharding"`
	Search              SearchConfig        `mapstructure:"search"`
	RateLimit           RateLimitConfig     `mapstructure:"rate_limit"`
	Metrics             MetricsConfig       `mapstructure:"metrics"`
	Tracing             TracingConfig       `mapstructure:"tracing"`
	UI                  UIConfig            `mapstructure:"ui"`
	Security            SecurityConfig      `mapstructure:"security"`
	SCIP                SCIPConfig          `mapstructure:"scip"`
	CodeHosts           map[string]CodeHost `mapstructure:"codehosts"`
	ConnectionsReadOnly bool                `mapstructure:"connections_readonly"` // When true, connections can only be managed via config
	ReposReadOnly       bool                `mapstructure:"repos_readonly"`       // When true, repos can only be managed via sync (no manual add/delete)
}

// ConfigFile is the path to the config file (can be set via CONFIG_FILE env var or -config flag).
var ConfigFile string

// RepoConfig represents per-repo configuration options.
// Used within CodeHost.RepoConfigs for detailed repo settings.
type RepoConfig struct {
	Name     string   `mapstructure:"name"`     // Repository name (e.g., "owner/repo")
	Branches []string `mapstructure:"branches"` // Specific branches to index (empty = default branch only)
	Exclude  bool     `mapstructure:"exclude"`  // If true, exclude this repo from indexing (can be re-included)
	Delete   bool     `mapstructure:"delete"`   // If true, permanently delete this repo (won't be re-added on sync)
}

// CodeHost represents a code hosting provider configuration
// Token can be either a literal value or a reference to an environment variable (e.g., "$CS_GITHUB_TOKEN").
type CodeHost struct {
	Type            string       `mapstructure:"type"`             // github, gitlab, bitbucket, github_enterprise, gitlab_self_hosted
	URL             string       `mapstructure:"url"`              // Base URL (required for self-hosted instances)
	Token           string       `mapstructure:"token"`            // Token or env var reference (e.g., "$CS_GITHUB_TOKEN")
	ExcludeArchived bool         `mapstructure:"exclude_archived"` // When true, archived repos are excluded from sync
	CleanupArchived bool         `mapstructure:"cleanup_archived"` // When true, auto-cleanup index for archived repos
	Repos           []string     `mapstructure:"repos"`            // Specific repos to index (optional, if empty syncs all accessible repos)
	RepoConfigs     []RepoConfig `mapstructure:"repo_configs"`     // Detailed per-repo configuration (branches, exclude, etc.)
}

type ServerConfig struct {
	Addr         string        `mapstructure:"addr"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

type DatabaseConfig struct {
	Driver          string        `mapstructure:"driver"` // postgres, mysql (auto-detected from URL if not set)
	URL             string        `mapstructure:"url"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

type RedisConfig struct {
	URL           string `mapstructure:"url"`
	Addr          string `mapstructure:"addr"`
	Password      string `mapstructure:"password"`
	DB            int    `mapstructure:"db"`
	TLSEnabled    bool   `mapstructure:"tls_enabled"`      // Enable TLS connection to Redis
	TLSSkipVerify bool   `mapstructure:"tls_skip_verify"`  // Skip TLS certificate verification (insecure)
	TLSCertFile   string `mapstructure:"tls_cert_file"`    // Path to client certificate file (for mTLS)
	TLSKeyFile    string `mapstructure:"tls_key_file"`     // Path to client key file (for mTLS)
	TLSCACertFile string `mapstructure:"tls_ca_cert_file"` // Path to CA certificate file
	TLSServerName string `mapstructure:"tls_server_name"`  // Override server name for TLS verification
}

type ZoektConfig struct {
	URL       string `mapstructure:"url"` // Single URL or comma-separated URLs for sharding
	IndexPath string `mapstructure:"index_path"`
	Shards    int    `mapstructure:"shards"` // Number of shards (for auto-discovery in K8s)
}

type IndexerConfig struct {
	Concurrency      int           `mapstructure:"concurrency"`
	IndexPath        string        `mapstructure:"index_path"`
	ReposPath        string        `mapstructure:"repos_path"`
	ReindexInterval  time.Duration `mapstructure:"reindex_interval"`
	ZoektBin         string        `mapstructure:"zoekt_bin"`
	CtagsBin         string        `mapstructure:"ctags_bin"`          // Path to ctags binary (for symbol indexing)
	RequireCtags     bool          `mapstructure:"require_ctags"`      // If true, ctags failures will fail indexing
	IndexAllBranches bool          `mapstructure:"index_all_branches"` // If true, index all branches (not just default)
	IndexTimeout     time.Duration `mapstructure:"index_timeout"`      // Timeout for zoekt-git-index operations (0 = no timeout/infinite, default: 0)
	MaxRepoSizeMB    int64         `mapstructure:"max_repo_size_mb"`   // Skip indexing repos larger than this size in MB (0 = no limit, default: 0)
}

type ReposConfig struct {
	BasePath string `mapstructure:"base_path"`
}

type SchedulerConfig struct {
	Enabled               bool          `mapstructure:"enabled"`
	PollInterval          time.Duration `mapstructure:"poll_interval"`           // Default time between syncs
	CheckInterval         time.Duration `mapstructure:"check_interval"`          // How often to check for repos needing sync
	StaleThreshold        time.Duration `mapstructure:"stale_threshold"`         // Max time in 'indexing' before repo is reset to 'pending'
	PendingJobTimeout     time.Duration `mapstructure:"pending_job_timeout"`     // Max time in 'pending' before repo is re-queued (default: 5m)
	MaxConcurrentChecks   int           `mapstructure:"max_concurrent_checks"`   // Parallel git fetch checks
	JobRetention          time.Duration `mapstructure:"job_retention"`           // How long to keep completed/failed jobs
	OrphanCleanupInterval time.Duration `mapstructure:"orphan_cleanup_interval"` // How often to check for orphan shards (0 to disable)
}

type ReplaceConfig struct {
	Concurrency  int           `mapstructure:"concurrency"`   // Number of repositories to process in parallel (default: 3)
	CloneTimeout time.Duration `mapstructure:"clone_timeout"` // Timeout for git clone operations (default: 10m)
	PushTimeout  time.Duration `mapstructure:"push_timeout"`  // Timeout for git push operations (default: 5m)
	MaxFileSize  int64         `mapstructure:"max_file_size"` // Maximum file size to process in bytes (default: 10MB)
	WorkDir      string        `mapstructure:"work_dir"`      // Working directory for temporary clones (default: /tmp/codesearch-replace)
}

// ShardingConfig defines settings for horizontal scaling with hash-based sharding.
type ShardingConfig struct {
	Enabled         bool   `mapstructure:"enabled"`          // Enable hash-based sharding
	TotalShards     int    `mapstructure:"total_shards"`     // Total number of shards (indexer replicas)
	IndexerAPIPort  int    `mapstructure:"indexer_api_port"` // Port for indexer HTTP API (default: 8081)
	IndexerService  string `mapstructure:"indexer_service"`  // Headless service name for indexer discovery (default: code-search-indexer-headless)
	FederatedAccess bool   `mapstructure:"federated_access"` // Enable federated file browsing/replace via proxy
}

// SearchConfig defines search behavior settings.
type SearchConfig struct {
	EnableStreaming bool `mapstructure:"enable_streaming"` // Enable true streaming from Zoekt (faster time-to-first-result)
}

// UIConfig defines UI display settings.
type UIConfig struct {
	HideReadOnlyBanner  bool `mapstructure:"hide_readonly_banner"`  // Hide the read-only mode banner in the UI
	HideFileNavigator   bool `mapstructure:"hide_file_navigator"`   // Hide the browse links in search results
	DisableBrowseAPI    bool `mapstructure:"disable_browse_api"`    // Completely disable the browse API endpoints
	HideReposPage       bool `mapstructure:"hide_repos_page"`       // Hide the Repositories page from navigation
	HideConnectionsPage bool `mapstructure:"hide_connections_page"` // Hide the Connections page from navigation
	HideJobsPage        bool `mapstructure:"hide_jobs_page"`        // Hide the Jobs page from navigation
	HideReplacePage     bool `mapstructure:"hide_replace_page"`     // Hide the Replace page from navigation
}

// RateLimitConfig defines rate limiting settings.
type RateLimitConfig struct {
	Enabled           bool    `mapstructure:"enabled"`             // Enable rate limiting
	RequestsPerSecond float64 `mapstructure:"requests_per_second"` // Requests per second per client
	BurstSize         int     `mapstructure:"burst_size"`          // Maximum burst size
}

// MetricsConfig defines Prometheus metrics settings.
type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"` // Enable Prometheus metrics endpoint
	Path    string `mapstructure:"path"`    // Path to expose metrics (default: /metrics)
}

// TracingConfig defines OpenTelemetry tracing settings.
type TracingConfig struct {
	Enabled        bool    `mapstructure:"enabled"`         // Enable OpenTelemetry tracing
	ServiceName    string  `mapstructure:"service_name"`    // Service name for traces (default: code-search)
	ServiceVersion string  `mapstructure:"service_version"` // Service version (default: 1.0.0)
	Environment    string  `mapstructure:"environment"`     // Deployment environment (default: development)
	Endpoint       string  `mapstructure:"endpoint"`        // OTLP endpoint (default: localhost:4317)
	Protocol       string  `mapstructure:"protocol"`        // Protocol: grpc or http (default: grpc)
	SampleRate     float64 `mapstructure:"sample_rate"`     // Sampling rate 0.0-1.0 (default: 1.0)
	Insecure       bool    `mapstructure:"insecure"`        // Disable TLS (default: true for local dev)
}

// SecurityConfig defines security settings.
type SecurityConfig struct {
	EncryptionKey string `mapstructure:"encryption_key"` // Key for encrypting tokens at rest (empty = disabled)
}

// SCIPConfig defines settings for automatic SCIP code intelligence indexing.
type SCIPConfig struct {
	Enabled   bool                          `mapstructure:"enabled"`    // Enable SCIP indexing
	AutoIndex bool                          `mapstructure:"auto_index"` // Auto-index after Zoekt indexing
	Timeout   time.Duration                 `mapstructure:"timeout"`    // Timeout for SCIP indexing operations
	WorkDir   string                        `mapstructure:"work_dir"`   // Working directory for temporary files (empty = system temp)
	CacheDir  string                        `mapstructure:"cache_dir"`  // Directory for SCIP SQLite databases (empty = derived from repos path)
	Languages map[string]SCIPLanguageConfig `mapstructure:"languages"`  // Per-language configuration
}

// SCIPLanguageConfig defines per-language SCIP indexer settings.
type SCIPLanguageConfig struct {
	Enabled    bool   `mapstructure:"enabled"`     // Enable this language's indexer
	BinaryPath string `mapstructure:"binary_path"` // Path to the indexer binary (empty = look in PATH)
}

// ResolveToken resolves the token value, expanding environment variable references
// If token starts with "$", it's treated as an env var reference (e.g., "$CS_GITHUB_TOKEN").
func (ch *CodeHost) ResolveToken() string {
	if after, ok := strings.CutPrefix(ch.Token, "$"); ok {
		envVar := after
		return os.Getenv(envVar)
	}

	return ch.Token
}

// GetCodeHost returns a code host configuration by name.
func (c *Config) GetCodeHost(name string) (*CodeHost, bool) {
	if c.CodeHosts == nil {
		return nil, false
	}

	ch, ok := c.CodeHosts[name]
	if !ok {
		return nil, false
	}

	return &ch, true
}

// Load loads configuration with the following priority (highest to lowest):
// 1. Environment variables
// 2. Config file (config.yaml)
// 3. Default values.
func Load() (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Bind environment variables BEFORE reading config file
	// This ensures env vars take priority
	bindEnvVars(v)

	// Try to read config file
	err := loadConfigFile(v)
	if err != nil {
		return nil, err
	}

	var cfg Config

	err = v.Unmarshal(&cfg)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	err = cfg.validate()
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.addr", ":8080")
	v.SetDefault("server.read_timeout", 15*time.Second)
	v.SetDefault("server.write_timeout", 60*time.Second)

	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_lifetime", 5*time.Minute)

	v.SetDefault("redis.addr", "localhost:6379")
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.tls_enabled", false)
	v.SetDefault("redis.tls_skip_verify", false)

	v.SetDefault("zoekt.url", "http://localhost:6070")
	v.SetDefault("zoekt.index_path", "/data/index") // Default path for Zoekt index shards

	v.SetDefault("indexer.concurrency", 2)
	v.SetDefault("indexer.index_path", "/data/index") // Default path for index shards
	v.SetDefault("indexer.repos_path", "/data/repos") // Default path for cloned repos
	v.SetDefault("indexer.reindex_interval", 1*time.Hour)
	v.SetDefault("indexer.zoekt_bin", "zoekt-git-index")
	v.SetDefault("indexer.ctags_bin", "ctags")  // universal-ctags or ctags
	v.SetDefault("indexer.require_ctags", true) // Enable symbol indexing by default

	v.SetDefault("repos.base_path", "/data/repos") // Default path for repository storage

	v.SetDefault("scheduler.enabled", true)
	v.SetDefault("scheduler.poll_interval", 6*time.Hour)
	v.SetDefault("scheduler.check_interval", 5*time.Minute)
	v.SetDefault("scheduler.stale_threshold", 24*time.Hour)
	v.SetDefault("scheduler.max_concurrent_checks", 5)
	v.SetDefault("scheduler.job_retention", 1*time.Hour)
	v.SetDefault("scheduler.orphan_cleanup_interval", 1*time.Hour)

	v.SetDefault("replace.concurrency", 3)
	v.SetDefault("replace.clone_timeout", 10*time.Minute)
	v.SetDefault("replace.push_timeout", 5*time.Minute)
	v.SetDefault("replace.max_file_size", 10*1024*1024)         // 10MB
	v.SetDefault("replace.work_dir", "/tmp/codesearch-replace") // Temporary work directory

	// Sharding defaults (for horizontal scaling)
	v.SetDefault("sharding.enabled", false)
	v.SetDefault("sharding.total_shards", 1)
	v.SetDefault("sharding.indexer_api_port", 8081)
	v.SetDefault("sharding.indexer_service", "code-search-indexer-headless")
	v.SetDefault("sharding.federated_access", false)

	// Search defaults
	v.SetDefault("search.enable_streaming", false) // Opt-in for true streaming

	// UI defaults
	v.SetDefault("ui.hide_readonly_banner", false)
	v.SetDefault("ui.hide_file_navigator", false)
	v.SetDefault("ui.disable_browse_api", false)
	v.SetDefault("ui.hide_repos_page", false)
	v.SetDefault("ui.hide_connections_page", false)
	v.SetDefault("ui.hide_jobs_page", false)
	v.SetDefault("ui.hide_replace_page", false)

	// Security defaults (encryption disabled by default)
	v.SetDefault("security.encryption_key", "")

	// SCIP defaults (disabled by default)
	v.SetDefault("scip.enabled", false)
	v.SetDefault("scip.auto_index", true)
	v.SetDefault("scip.timeout", 10*time.Minute)
	v.SetDefault("scip.work_dir", "")  // Empty = system temp
	v.SetDefault("scip.cache_dir", "") // Empty = derived from repos path

	// Tracing defaults (disabled by default, configure via env vars for Datadog/OTLP)
	v.SetDefault("tracing.enabled", false)
	v.SetDefault("tracing.service_name", "code-search")
	v.SetDefault("tracing.service_version", "1.0.0")
	v.SetDefault("tracing.environment", "development")
	v.SetDefault("tracing.endpoint", "localhost:4317")
	v.SetDefault("tracing.protocol", "grpc")
	v.SetDefault("tracing.sample_rate", 1.0)
	v.SetDefault("tracing.insecure", true)
}

func bindEnvVars(v *viper.Viper) {
	// Enable automatic env binding with CS_ prefix
	v.SetEnvPrefix("CS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Explicitly bind ALL config keys to environment variables
	// AutomaticEnv only works with Get() calls, not with Unmarshal()
	// Format: config.key -> CS_CONFIG_KEY

	// Top-level config
	_ = v.BindEnv("connections_readonly", "CS_CONNECTIONS_READONLY")
	_ = v.BindEnv("repos_readonly", "CS_REPOS_READONLY")

	// UI config
	_ = v.BindEnv("ui.hide_readonly_banner", "CS_UI_HIDE_READONLY_BANNER")
	_ = v.BindEnv("ui.hide_file_navigator", "CS_UI_HIDE_FILE_NAVIGATOR")
	_ = v.BindEnv("ui.disable_browse_api", "CS_UI_DISABLE_BROWSE_API")
	_ = v.BindEnv("ui.hide_repos_page", "CS_UI_HIDE_REPOS_PAGE")
	_ = v.BindEnv("ui.hide_connections_page", "CS_UI_HIDE_CONNECTIONS_PAGE")
	_ = v.BindEnv("ui.hide_jobs_page", "CS_UI_HIDE_JOBS_PAGE")
	_ = v.BindEnv("ui.hide_replace_page", "CS_UI_HIDE_REPLACE_PAGE")

	// Server config
	_ = v.BindEnv("server.addr", "CS_SERVER_ADDR")
	_ = v.BindEnv("server.read_timeout", "CS_SERVER_READ_TIMEOUT")
	_ = v.BindEnv("server.write_timeout", "CS_SERVER_WRITE_TIMEOUT")

	// Database config
	_ = v.BindEnv("database.url", "CS_DATABASE_URL")
	_ = v.BindEnv("database.max_open_conns", "CS_DATABASE_MAX_OPEN_CONNS")
	_ = v.BindEnv("database.max_idle_conns", "CS_DATABASE_MAX_IDLE_CONNS")
	_ = v.BindEnv("database.conn_max_lifetime", "CS_DATABASE_CONN_MAX_LIFETIME")

	// Redis config
	_ = v.BindEnv("redis.url", "CS_REDIS_URL")
	_ = v.BindEnv("redis.addr", "CS_REDIS_ADDR")
	_ = v.BindEnv("redis.password", "CS_REDIS_PASSWORD")
	_ = v.BindEnv("redis.db", "CS_REDIS_DB")
	_ = v.BindEnv("redis.tls_enabled", "CS_REDIS_TLS_ENABLED")
	_ = v.BindEnv("redis.tls_skip_verify", "CS_REDIS_TLS_SKIP_VERIFY")
	_ = v.BindEnv("redis.tls_cert_file", "CS_REDIS_TLS_CERT_FILE")
	_ = v.BindEnv("redis.tls_key_file", "CS_REDIS_TLS_KEY_FILE")
	_ = v.BindEnv("redis.tls_ca_cert_file", "CS_REDIS_TLS_CA_CERT_FILE")
	_ = v.BindEnv("redis.tls_server_name", "CS_REDIS_TLS_SERVER_NAME")

	// Zoekt config
	_ = v.BindEnv("zoekt.url", "CS_ZOEKT_URL")
	_ = v.BindEnv("zoekt.index_path", "CS_ZOEKT_INDEX_PATH")
	_ = v.BindEnv("zoekt.shards", "CS_ZOEKT_SHARDS")

	// Indexer config
	_ = v.BindEnv("indexer.concurrency", "CS_INDEXER_CONCURRENCY")
	_ = v.BindEnv("indexer.index_path", "CS_INDEXER_INDEX_PATH")
	_ = v.BindEnv("indexer.repos_path", "CS_INDEXER_REPOS_PATH")
	_ = v.BindEnv("indexer.reindex_interval", "CS_INDEXER_REINDEX_INTERVAL")
	_ = v.BindEnv("indexer.zoekt_bin", "CS_INDEXER_ZOEKT_BIN")
	_ = v.BindEnv("indexer.ctags_bin", "CS_INDEXER_CTAGS_BIN")
	_ = v.BindEnv("indexer.require_ctags", "CS_INDEXER_REQUIRE_CTAGS")

	// Repos config
	_ = v.BindEnv("repos.base_path", "CS_REPOS_BASE_PATH")

	// Scheduler config
	_ = v.BindEnv("scheduler.enabled", "CS_SCHEDULER_ENABLED")
	_ = v.BindEnv("scheduler.poll_interval", "CS_SCHEDULER_POLL_INTERVAL")
	_ = v.BindEnv("scheduler.check_interval", "CS_SCHEDULER_CHECK_INTERVAL")
	_ = v.BindEnv("scheduler.stale_threshold", "CS_SCHEDULER_STALE_THRESHOLD")
	_ = v.BindEnv("scheduler.max_concurrent_checks", "CS_SCHEDULER_MAX_CONCURRENT_CHECKS")
	_ = v.BindEnv("scheduler.job_retention", "CS_SCHEDULER_JOB_RETENTION")

	// Replace config
	_ = v.BindEnv("replace.concurrency", "CS_REPLACE_CONCURRENCY")
	_ = v.BindEnv("replace.clone_timeout", "CS_REPLACE_CLONE_TIMEOUT")
	_ = v.BindEnv("replace.push_timeout", "CS_REPLACE_PUSH_TIMEOUT")
	_ = v.BindEnv("replace.max_file_size", "CS_REPLACE_MAX_FILE_SIZE")
	_ = v.BindEnv("replace.work_dir", "CS_REPLACE_WORK_DIR")

	// Sharding config (for horizontal scaling)
	_ = v.BindEnv("sharding.enabled", "CS_SHARDING_ENABLED")
	_ = v.BindEnv("sharding.total_shards", "CS_SHARDING_TOTAL_SHARDS", "TOTAL_SHARDS")
	_ = v.BindEnv("sharding.indexer_api_port", "CS_SHARDING_INDEXER_API_PORT")
	_ = v.BindEnv("sharding.indexer_service", "CS_SHARDING_INDEXER_SERVICE")
	_ = v.BindEnv("sharding.federated_access", "CS_SHARDING_FEDERATED_ACCESS")

	// Search config
	_ = v.BindEnv("search.enable_streaming", "CS_SEARCH_ENABLE_STREAMING")

	// Security config
	_ = v.BindEnv("security.encryption_key", "CS_SECURITY_ENCRYPTION_KEY")

	// SCIP config
	_ = v.BindEnv("scip.enabled", "CS_SCIP_ENABLED")
	_ = v.BindEnv("scip.auto_index", "CS_SCIP_AUTO_INDEX")
	_ = v.BindEnv("scip.timeout", "CS_SCIP_TIMEOUT")
	_ = v.BindEnv("scip.work_dir", "CS_SCIP_WORK_DIR")
	_ = v.BindEnv("scip.cache_dir", "CS_SCIP_CACHE_DIR")

	// Tracing config (supports both CS_ prefix and standard OTEL_/DD_ env vars)
	_ = v.BindEnv(
		"tracing.enabled",
		"CS_TRACING_ENABLED",
		"OTEL_TRACING_ENABLED",
		"DD_TRACE_ENABLED",
	)
	_ = v.BindEnv(
		"tracing.service_name",
		"CS_TRACING_SERVICE_NAME",
		"OTEL_SERVICE_NAME",
		"DD_SERVICE",
	)
	_ = v.BindEnv(
		"tracing.service_version",
		"CS_TRACING_SERVICE_VERSION",
		"OTEL_SERVICE_VERSION",
		"DD_VERSION",
	)
	_ = v.BindEnv("tracing.environment", "CS_TRACING_ENVIRONMENT", "DD_ENV")
	_ = v.BindEnv("tracing.endpoint", "CS_TRACING_ENDPOINT", "OTEL_EXPORTER_OTLP_ENDPOINT")
	_ = v.BindEnv("tracing.protocol", "CS_TRACING_PROTOCOL", "OTEL_EXPORTER_OTLP_PROTOCOL")
	_ = v.BindEnv("tracing.sample_rate", "CS_TRACING_SAMPLE_RATE")
	_ = v.BindEnv("tracing.insecure", "CS_TRACING_INSECURE", "OTEL_EXPORTER_OTLP_INSECURE")
}

func loadConfigFile(v *viper.Viper) error {
	v.SetConfigType("yaml")

	// Check for explicit config file path
	configPath := ConfigFile
	if configPath == "" {
		configPath = os.Getenv("CS_CONFIG_FILE")
	}

	if configPath != "" {
		// Use explicit config file
		v.SetConfigFile(configPath)

		err := v.ReadInConfig()
		if err != nil {
			return fmt.Errorf("error reading config file %s: %w", configPath, err)
		}

		log.Printf("Loaded config from: %s", configPath)

		return nil
	}

	// Search for config file in standard locations
	v.SetConfigName("config")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	v.AddConfigPath("/etc/code-search")
	v.AddConfigPath("$HOME/.code-search")

	err := v.ReadInConfig()
	if err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			// Config file not found, this is OK - use defaults and env vars
			return nil
		}

		return fmt.Errorf("error reading config file: %w", err)
	}

	log.Printf("Loaded config from: %s", v.ConfigFileUsed())

	return nil
}

func (c *Config) validate() error {
	// Database URL is optional for CLI usage
	// Allow empty for CLI or warn in future if needed
	return nil
}
