package log

import (
	"os"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected zapcore.Level
	}{
		{"debug", zapcore.DebugLevel},
		{"DEBUG", zapcore.DebugLevel},
		{"info", zapcore.InfoLevel},
		{"INFO", zapcore.InfoLevel},
		{"warn", zapcore.WarnLevel},
		{"WARN", zapcore.WarnLevel},
		{"warning", zapcore.WarnLevel},
		{"error", zapcore.ErrorLevel},
		{"ERROR", zapcore.ErrorLevel},
		{"invalid", zapcore.InfoLevel},
		{"", zapcore.InfoLevel},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseLevel(tt.input)
			if result != tt.expected {
				t.Errorf("parseLevel(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Level != "info" {
		t.Errorf("Level = %v, want info", cfg.Level)
	}

	if cfg.Format != "json" {
		t.Errorf("Format = %v, want json", cfg.Format)
	}

	if cfg.Development {
		t.Error("Development should be false by default")
	}
}

func TestInit(t *testing.T) {
	cfg := Config{
		Level:       "info",
		Format:      "json",
		Development: false,
	}

	err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if L == nil {
		t.Error("L should not be nil after Init")
	}

	if S == nil {
		t.Error("S should not be nil after Init")
	}
}

func TestInit_InvalidFormat(t *testing.T) {
	cfg := Config{
		Level:       "info",
		Format:      "invalid",
		Development: false,
	}

	err := Init(cfg)
	if err != nil {
		t.Fatalf("Init should handle invalid format gracefully: %v", err)
	}
}

func TestInit_DevMode(t *testing.T) {
	cfg := Config{
		Level:       "debug",
		Format:      "console",
		Development: true,
	}

	err := Init(cfg)
	if err != nil {
		t.Fatalf("Init with dev mode failed: %v", err)
	}

	if L == nil {
		t.Error("L should not be nil after Init with dev mode")
	}
}

func TestInitFromEnv(t *testing.T) {
	originalLevel := os.Getenv("LOG_LEVEL")
	originalFormat := os.Getenv("LOG_FORMAT")
	originalDev := os.Getenv("LOG_DEV")

	defer func() {
		os.Setenv("LOG_LEVEL", originalLevel)
		os.Setenv("LOG_FORMAT", originalFormat)
		os.Setenv("LOG_DEV", originalDev)
	}()

	os.Setenv("LOG_LEVEL", "warn")
	os.Setenv("LOG_FORMAT", "json")
	os.Setenv("LOG_DEV", "false")

	err := InitFromEnv()
	if err != nil {
		t.Fatalf("InitFromEnv failed: %v", err)
	}

	if L == nil {
		t.Error("L should not be nil after InitFromEnv")
	}
}

func TestInitFromEnv_Defaults(t *testing.T) {
	originalLevel := os.Getenv("LOG_LEVEL")
	originalFormat := os.Getenv("LOG_FORMAT")
	originalDev := os.Getenv("LOG_DEV")

	defer func() {
		os.Setenv("LOG_LEVEL", originalLevel)
		os.Setenv("LOG_FORMAT", originalFormat)
		os.Setenv("LOG_DEV", originalDev)
	}()

	os.Unsetenv("LOG_LEVEL")
	os.Unsetenv("LOG_FORMAT")
	os.Unsetenv("LOG_DEV")

	err := InitFromEnv()
	if err != nil {
		t.Fatalf("InitFromEnv with defaults failed: %v", err)
	}
}

func TestInitFromEnv_DevMode(t *testing.T) {
	originalDev := os.Getenv("LOG_DEV")

	defer func() {
		os.Setenv("LOG_DEV", originalDev)
	}()

	t.Run("true", func(t *testing.T) {
		os.Setenv("LOG_DEV", "true")

		err := InitFromEnv()
		if err != nil {
			t.Fatalf("InitFromEnv failed: %v", err)
		}
	})

	t.Run("1", func(t *testing.T) {
		os.Setenv("LOG_DEV", "1")

		err := InitFromEnv()
		if err != nil {
			t.Fatalf("InitFromEnv failed: %v", err)
		}
	})
}

func TestWith(t *testing.T) {
	cfg := DefaultConfig()

	err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	newLogger := With(zap.String("key", "value"))
	if newLogger == nil {
		t.Error("With should not return nil")
	}
}

func TestWithComponent(t *testing.T) {
	cfg := DefaultConfig()

	err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	newLogger := WithComponent("test-component")
	if newLogger == nil {
		t.Error("WithComponent should not return nil")
	}
}

func TestLogFunctions(t *testing.T) {
	cfg := Config{
		Level:       "debug",
		Format:      "json",
		Development: false,
	}

	err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	Debug("debug message")
	Info("info message")
	Warn("warn message")
	Error("error message")

	Debug("debug with field", String("key", "value"))
	Info("info with field", Int("count", 42))
	Warn("warn with field", Bool("enabled", true))
	Error("error with field", Err(nil))
}

func TestSync(t *testing.T) {
	cfg := DefaultConfig()

	err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	Sync()
}

func TestAllLogLevels(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error"}

	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			cfg := Config{
				Level:       level,
				Format:      "json",
				Development: false,
			}

			err := Init(cfg)
			if err != nil {
				t.Fatalf("Init with level %s failed: %v", level, err)
			}
		})
	}
}

func TestAllFormats(t *testing.T) {
	formats := []string{"json", "console"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			cfg := Config{
				Level:       "info",
				Format:      format,
				Development: false,
			}

			err := Init(cfg)
			if err != nil {
				t.Fatalf("Init with format %s failed: %v", format, err)
			}
		})
	}
}

func TestFieldShortcuts(t *testing.T) {
	stringField := String("key", "value")
	if stringField.Key != "key" {
		t.Errorf("String key = %v, want key", stringField.Key)
	}

	intField := Int("count", 42)
	if intField.Key != "count" {
		t.Errorf("Int key = %v, want count", intField.Key)
	}

	int64Field := Int64("bignum", 9999999999)
	if int64Field.Key != "bignum" {
		t.Errorf("Int64 key = %v, want bignum", int64Field.Key)
	}

	boolField := Bool("enabled", true)
	if boolField.Key != "enabled" {
		t.Errorf("Bool key = %v, want enabled", boolField.Key)
	}

	// Err with nil returns a skip field, test with actual error
	testErr := os.ErrNotExist

	errField := Err(testErr)
	if errField.Key != "error" {
		t.Errorf("Err key = %v, want error", errField.Key)
	}

	anyField := Any("data", map[string]string{"a": "b"})
	if anyField.Key != "data" {
		t.Errorf("Any key = %v, want data", anyField.Key)
	}
}
