package tracing

import (
	"context"
	"errors"
	"os"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Enabled {
		t.Error("expected Enabled to be false by default")
	}

	if cfg.ServiceName != "code-search" {
		t.Errorf("expected ServiceName 'code-search', got %q", cfg.ServiceName)
	}

	if cfg.ServiceVersion != "1.0.0" {
		t.Errorf("expected ServiceVersion '1.0.0', got %q", cfg.ServiceVersion)
	}

	if cfg.Environment != "development" {
		t.Errorf("expected Environment 'development', got %q", cfg.Environment)
	}

	if cfg.Endpoint != "localhost:4317" {
		t.Errorf("expected Endpoint 'localhost:4317', got %q", cfg.Endpoint)
	}

	if cfg.Protocol != "grpc" {
		t.Errorf("expected Protocol 'grpc', got %q", cfg.Protocol)
	}

	if cfg.SampleRate != 1.0 {
		t.Errorf("expected SampleRate 1.0, got %f", cfg.SampleRate)
	}

	if !cfg.Insecure {
		t.Error("expected Insecure to be true by default")
	}
}

func TestConfigFromEnv(t *testing.T) {
	// Save and restore original env vars
	originalEnv := map[string]string{
		"OTEL_TRACING_ENABLED":        os.Getenv("OTEL_TRACING_ENABLED"),
		"DD_TRACE_ENABLED":            os.Getenv("DD_TRACE_ENABLED"),
		"OTEL_SERVICE_NAME":           os.Getenv("OTEL_SERVICE_NAME"),
		"DD_SERVICE":                  os.Getenv("DD_SERVICE"),
		"OTEL_SERVICE_VERSION":        os.Getenv("OTEL_SERVICE_VERSION"),
		"DD_VERSION":                  os.Getenv("DD_VERSION"),
		"DD_ENV":                      os.Getenv("DD_ENV"),
		"OTEL_EXPORTER_OTLP_ENDPOINT": os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		"DD_AGENT_HOST":               os.Getenv("DD_AGENT_HOST"),
		"OTEL_EXPORTER_OTLP_PROTOCOL": os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL"),
	}

	defer func() {
		for k, v := range originalEnv {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	// Clear all relevant env vars first
	for k := range originalEnv {
		os.Unsetenv(k)
	}

	t.Run("default values", func(t *testing.T) {
		cfg := ConfigFromEnv()
		if cfg.Enabled {
			t.Error("expected Enabled to be false when no env vars set")
		}
	})

	t.Run("OTEL_TRACING_ENABLED=true", func(t *testing.T) {
		os.Setenv("OTEL_TRACING_ENABLED", "true")

		defer os.Unsetenv("OTEL_TRACING_ENABLED")

		cfg := ConfigFromEnv()
		if !cfg.Enabled {
			t.Error("expected Enabled to be true")
		}
	})

	t.Run("DD_TRACE_ENABLED=1", func(t *testing.T) {
		os.Setenv("DD_TRACE_ENABLED", "1")

		defer os.Unsetenv("DD_TRACE_ENABLED")

		cfg := ConfigFromEnv()
		if !cfg.Enabled {
			t.Error("expected Enabled to be true with DD_TRACE_ENABLED=1")
		}
	})

	t.Run("service name from OTEL_SERVICE_NAME", func(t *testing.T) {
		os.Setenv("OTEL_SERVICE_NAME", "my-service")

		defer os.Unsetenv("OTEL_SERVICE_NAME")

		cfg := ConfigFromEnv()
		if cfg.ServiceName != "my-service" {
			t.Errorf("expected ServiceName 'my-service', got %q", cfg.ServiceName)
		}
	})

	t.Run("service name from DD_SERVICE", func(t *testing.T) {
		os.Setenv("DD_SERVICE", "datadog-service")

		defer os.Unsetenv("DD_SERVICE")

		cfg := ConfigFromEnv()
		if cfg.ServiceName != "datadog-service" {
			t.Errorf("expected ServiceName 'datadog-service', got %q", cfg.ServiceName)
		}
	})

	t.Run("endpoint from OTEL_EXPORTER_OTLP_ENDPOINT", func(t *testing.T) {
		os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "otlp.example.com:4317")

		defer os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")

		cfg := ConfigFromEnv()
		if cfg.Endpoint != "otlp.example.com:4317" {
			t.Errorf("expected Endpoint 'otlp.example.com:4317', got %q", cfg.Endpoint)
		}
	})

	t.Run("endpoint from DD_AGENT_HOST", func(t *testing.T) {
		os.Setenv("DD_AGENT_HOST", "datadog-agent")

		defer os.Unsetenv("DD_AGENT_HOST")

		cfg := ConfigFromEnv()
		if cfg.Endpoint != "datadog-agent:4317" {
			t.Errorf("expected Endpoint 'datadog-agent:4317', got %q", cfg.Endpoint)
		}
	})

	t.Run("protocol from env", func(t *testing.T) {
		os.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http")

		defer os.Unsetenv("OTEL_EXPORTER_OTLP_PROTOCOL")

		cfg := ConfigFromEnv()
		if cfg.Protocol != "http" {
			t.Errorf("expected Protocol 'http', got %q", cfg.Protocol)
		}
	})
}

func TestInitTracing_Disabled(t *testing.T) {
	logger := zap.NewNop()
	cfg := &Config{Enabled: false}

	provider, err := InitTracing(context.Background(), cfg, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if provider == nil {
		t.Fatal("expected non-nil provider")
	}

	// Shutdown should not error
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Errorf("unexpected shutdown error: %v", err)
	}
}

func TestTracer(t *testing.T) {
	tracer := Tracer()
	if tracer == nil {
		t.Error("expected non-nil tracer")
	}
}

func TestStartSpan(t *testing.T) {
	ctx := context.Background()

	ctx, span := StartSpan(ctx, "test-span")
	defer span.End()

	if span == nil {
		t.Error("expected non-nil span")
	}

	// Context should contain the span
	spanFromCtx := trace.SpanFromContext(ctx)
	if spanFromCtx == nil {
		t.Error("expected span in context")
	}
}

func TestSpanFromContext(t *testing.T) {
	t.Run("with span", func(t *testing.T) {
		ctx := context.Background()

		ctx, span := StartSpan(ctx, "test-span")
		defer span.End()

		retrieved := SpanFromContext(ctx)
		if retrieved == nil {
			t.Error("expected span from context")
		}
	})

	t.Run("without span", func(t *testing.T) {
		ctx := context.Background()
		span := SpanFromContext(ctx)
		// Should return a no-op span, not nil
		if span == nil {
			t.Error("expected non-nil span even without context")
		}
	})
}

func TestAddEvent(t *testing.T) {
	ctx := context.Background()

	ctx, span := StartSpan(ctx, "test-span")
	defer span.End()

	// Should not panic
	AddEvent(ctx, "test-event", attribute.String("key", "value"))
}

func TestSetAttributes(t *testing.T) {
	ctx := context.Background()

	ctx, span := StartSpan(ctx, "test-span")
	defer span.End()

	// Should not panic
	SetAttributes(ctx,
		attribute.String("key1", "value1"),
		attribute.Int("key2", 42),
		attribute.Bool("key3", true),
	)
}

func TestRecordError(t *testing.T) {
	ctx := context.Background()

	ctx, span := StartSpan(ctx, "test-span")
	defer span.End()

	testErr := errors.New("test error")

	// Should not panic
	RecordError(ctx, testErr)
	RecordError(ctx, testErr, attribute.String("context", "additional info"))
}

func TestSetOK(t *testing.T) {
	ctx := context.Background()

	ctx, span := StartSpan(ctx, "test-span")
	defer span.End()

	// Should not panic
	SetOK(ctx)
}

func TestAttributeKeys(t *testing.T) {
	// Verify attribute keys are defined correctly
	attrs := []struct {
		key      attribute.Key
		expected string
	}{
		{AttrRepoID, "codesearch.repo.id"},
		{AttrRepoName, "codesearch.repo.name"},
		{AttrConnectionID, "codesearch.connection.id"},
		{AttrHostType, "codesearch.host.type"},
		{AttrJobID, "codesearch.job.id"},
		{AttrJobType, "codesearch.job.type"},
		{AttrSearchQuery, "codesearch.search.query"},
		{AttrSearchType, "codesearch.search.type"},
		{AttrResultCount, "codesearch.search.result_count"},
		{AttrBranchCount, "codesearch.branch.count"},
		{AttrBranches, "codesearch.branches"},
	}

	for _, tt := range attrs {
		if string(tt.key) != tt.expected {
			t.Errorf("expected key %q, got %q", tt.expected, string(tt.key))
		}
	}
}

func TestAttributeUsage(t *testing.T) {
	ctx := context.Background()

	ctx, span := StartSpan(ctx, "test-span")
	defer span.End()

	// Test using the predefined attribute keys
	SetAttributes(ctx,
		AttrRepoID.Int64(123),
		AttrRepoName.String("test-repo"),
		AttrConnectionID.Int64(456),
		AttrHostType.String("github"),
		AttrJobID.String("job-789"),
		AttrJobType.String("index"),
		AttrSearchQuery.String("func main"),
		AttrSearchType.String("text"),
		AttrResultCount.Int(42),
		AttrBranchCount.Int(3),
	)
}

func TestProvider_Shutdown_Nil(t *testing.T) {
	provider := &Provider{logger: zap.NewNop()}

	// Should not error or panic with nil provider
	err := provider.Shutdown(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
