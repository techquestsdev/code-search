package tracing

import (
	"context"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const (
	// TracerName is the name of the tracer.
	TracerName = "code-search"
)

// Config holds tracing configuration.
type Config struct {
	// Enabled enables tracing.
	Enabled bool

	// ServiceName is the name of the service.
	ServiceName string

	// ServiceVersion is the version of the service.
	ServiceVersion string

	// Environment is the deployment environment (e.g., production, staging).
	Environment string

	// Endpoint is the OTLP endpoint (e.g., localhost:4317 for gRPC, localhost:4318 for HTTP).
	Endpoint string

	// Protocol is the OTLP protocol: "grpc" or "http".
	Protocol string

	// SampleRate is the sampling rate (0.0 to 1.0).
	SampleRate float64

	// Insecure disables TLS for the connection.
	Insecure bool

	// Headers are custom headers to send with requests (useful for Datadog API key).
	Headers map[string]string
}

// DefaultConfig returns the default tracing configuration.
func DefaultConfig() *Config {
	return &Config{
		Enabled:        false,
		ServiceName:    "code-search",
		ServiceVersion: "1.0.0",
		Environment:    "development",
		Endpoint:       "localhost:4317",
		Protocol:       "grpc",
		SampleRate:     1.0,
		Insecure:       true,
		Headers:        make(map[string]string),
	}
}

// ConfigFromEnv creates a config from environment variables.
// This supports standard OpenTelemetry env vars and Datadog-specific ones.
func ConfigFromEnv() *Config {
	cfg := DefaultConfig()

	// Check if tracing is enabled
	if enabled := os.Getenv("OTEL_TRACING_ENABLED"); enabled == "true" || enabled == "1" {
		cfg.Enabled = true
	}

	// Datadog-specific: DD_TRACE_ENABLED
	if enabled := os.Getenv("DD_TRACE_ENABLED"); enabled == "true" || enabled == "1" {
		cfg.Enabled = true
	}

	// Service name
	if name := os.Getenv("OTEL_SERVICE_NAME"); name != "" {
		cfg.ServiceName = name
	}

	if name := os.Getenv("DD_SERVICE"); name != "" {
		cfg.ServiceName = name
	}

	// Service version
	if version := os.Getenv("OTEL_SERVICE_VERSION"); version != "" {
		cfg.ServiceVersion = version
	}

	if version := os.Getenv("DD_VERSION"); version != "" {
		cfg.ServiceVersion = version
	}

	// Environment
	if env := os.Getenv("OTEL_RESOURCE_ATTRIBUTES"); env != "" {
		cfg.Environment = env
	}

	if env := os.Getenv("DD_ENV"); env != "" {
		cfg.Environment = env
	}

	// OTLP endpoint
	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		cfg.Endpoint = endpoint
	}

	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"); endpoint != "" {
		cfg.Endpoint = endpoint
	}

	// Datadog Agent OTLP endpoint (default: localhost:4317)
	if agentHost := os.Getenv("DD_AGENT_HOST"); agentHost != "" {
		cfg.Endpoint = agentHost + ":4317"
	}

	// Protocol
	if protocol := os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL"); protocol != "" {
		cfg.Protocol = protocol
	}

	// Sample rate
	if rate := os.Getenv("OTEL_TRACES_SAMPLER_ARG"); rate != "" {
		// Parse sample rate - default to 1.0 if parsing fails
		cfg.SampleRate = 1.0
	}

	// Insecure
	if insecure := os.Getenv("OTEL_EXPORTER_OTLP_INSECURE"); insecure == "true" {
		cfg.Insecure = true
	}

	return cfg
}

// Provider wraps the trace provider and shutdown function.
type Provider struct {
	provider *sdktrace.TracerProvider
	logger   *zap.Logger
}

// InitTracing initializes OpenTelemetry tracing.
func InitTracing(ctx context.Context, cfg *Config, logger *zap.Logger) (*Provider, error) {
	if !cfg.Enabled {
		logger.Info("Tracing disabled")
		return &Provider{logger: logger}, nil
	}

	logger.Info("Initializing tracing",
		zap.String("service", cfg.ServiceName),
		zap.String("version", cfg.ServiceVersion),
		zap.String("env", cfg.Environment),
		zap.String("endpoint", cfg.Endpoint),
		zap.String("protocol", cfg.Protocol),
		zap.Float64("sample_rate", cfg.SampleRate),
	)

	// Create exporter based on protocol
	var (
		exporter *otlptrace.Exporter
		err      error
	)

	switch cfg.Protocol {
	case "http":
		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(cfg.Endpoint),
		}
		if cfg.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}

		if len(cfg.Headers) > 0 {
			opts = append(opts, otlptracehttp.WithHeaders(cfg.Headers))
		}

		exporter, err = otlptracehttp.New(ctx, opts...)
	default: // grpc
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(cfg.Endpoint),
		}
		if cfg.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}

		if len(cfg.Headers) > 0 {
			opts = append(opts, otlptracegrpc.WithHeaders(cfg.Headers))
		}

		exporter, err = otlptracegrpc.New(ctx, opts...)
	}

	if err != nil {
		return nil, err
	}

	// Create resource with service information
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
	if err != nil {
		return nil, err
	}

	// Create sampler
	var sampler sdktrace.Sampler
	if cfg.SampleRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if cfg.SampleRate <= 0.0 {
		sampler = sdktrace.NeverSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(cfg.SampleRate)
	}

	// Create trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Set global trace provider
	otel.SetTracerProvider(tp)

	// Set global propagator (W3C Trace Context and Baggage)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	logger.Info("Tracing initialized successfully")

	return &Provider{
		provider: tp,
		logger:   logger,
	}, nil
}

// Shutdown gracefully shuts down the trace provider.
func (p *Provider) Shutdown(ctx context.Context) error {
	if p.provider == nil {
		return nil
	}

	p.logger.Info("Shutting down tracing...")

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return p.provider.Shutdown(ctx)
}

// Tracer returns the global tracer.
func Tracer() trace.Tracer {
	return otel.Tracer(TracerName)
}

// StartSpan starts a new span with the given name.
func StartSpan(
	ctx context.Context,
	name string,
	opts ...trace.SpanStartOption,
) (context.Context, trace.Span) {
	return Tracer().Start(ctx, name, opts...)
}

// SpanFromContext returns the current span from context.
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// AddEvent adds an event to the current span.
func AddEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// SetAttributes sets attributes on the current span.
func SetAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attrs...)
}

// RecordError records an error on the current span.
func RecordError(ctx context.Context, err error, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.RecordError(err, trace.WithAttributes(attrs...))
	span.SetStatus(codes.Error, err.Error())
}

// SetOK sets the span status to OK.
func SetOK(ctx context.Context) {
	span := trace.SpanFromContext(ctx)
	span.SetStatus(codes.Ok, "")
}

// Common attribute keys for code search operations.
var (
	AttrRepoID       = attribute.Key("codesearch.repo.id")
	AttrRepoName     = attribute.Key("codesearch.repo.name")
	AttrConnectionID = attribute.Key("codesearch.connection.id")
	AttrHostType     = attribute.Key("codesearch.host.type")
	AttrJobID        = attribute.Key("codesearch.job.id")
	AttrJobType      = attribute.Key("codesearch.job.type")
	AttrSearchQuery  = attribute.Key("codesearch.search.query")
	AttrSearchType   = attribute.Key("codesearch.search.type")
	AttrResultCount  = attribute.Key("codesearch.search.result_count")
	AttrBranchCount  = attribute.Key("codesearch.branch.count")
	AttrBranches     = attribute.Key("codesearch.branches")
)
