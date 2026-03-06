// Package log provides a shared zap logger for all components.
package log

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// L is the global logger instance.
	L *zap.Logger
	// S is the global sugared logger for convenience (printf-style logging).
	S *zap.SugaredLogger
)

// Config holds logger configuration.
type Config struct {
	// Level is the minimum log level (debug, info, warn, error)
	Level string
	// Format is the output format (json, console)
	Format string
	// Development enables development mode (more verbose, stack traces)
	Development bool
}

// DefaultConfig returns sensible defaults for production.
func DefaultConfig() Config {
	return Config{
		Level:       "info",
		Format:      "json",
		Development: false,
	}
}

// Init initializes the global logger with the given configuration.
func Init(cfg Config) error {
	level := parseLevel(cfg.Level)

	var encoder zapcore.Encoder

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder

	if cfg.Format == "console" {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stdout),
		level,
	)

	opts := []zap.Option{
		zap.AddCaller(),
		zap.AddCallerSkip(0),
	}

	if cfg.Development {
		opts = append(opts, zap.Development())
		opts = append(opts, zap.AddStacktrace(zapcore.WarnLevel))
	} else {
		opts = append(opts, zap.AddStacktrace(zapcore.ErrorLevel))
	}

	L = zap.New(core, opts...)
	S = L.Sugar()

	return nil
}

// InitFromEnv initializes the logger from environment variables
// LOG_LEVEL: debug, info, warn, error (default: info)
// LOG_FORMAT: json, console (default: json)
// LOG_DEV: true/false - enables development mode (default: false).
func InitFromEnv() error {
	cfg := DefaultConfig()

	if level := os.Getenv("LOG_LEVEL"); level != "" {
		cfg.Level = level
	}

	if format := os.Getenv("LOG_FORMAT"); format != "" {
		cfg.Format = format
	}

	if dev := os.Getenv("LOG_DEV"); dev == "true" || dev == "1" {
		cfg.Development = true
	}

	return Init(cfg)
}

// parseLevel converts a string log level to zapcore.Level.
func parseLevel(level string) zapcore.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// With creates a child logger with additional fields.
func With(fields ...zap.Field) *zap.Logger {
	return L.With(fields...)
}

// WithComponent creates a child logger with a component field.
func WithComponent(component string) *zap.Logger {
	return L.With(zap.String("component", component))
}

// Sync flushes any buffered log entries. Should be called before program exit.
func Sync() {
	if L != nil {
		_ = L.Sync()
	}
}

// Debug logs a debug message.
func Debug(msg string, fields ...zap.Field) {
	L.Debug(msg, fields...)
}

// Info logs an info message.
func Info(msg string, fields ...zap.Field) {
	L.Info(msg, fields...)
}

// Warn logs a warning message.
func Warn(msg string, fields ...zap.Field) {
	L.Warn(msg, fields...)
}

// Error logs an error message.
func Error(msg string, fields ...zap.Field) {
	L.Error(msg, fields...)
}

// Fatal logs a fatal message and exits.
func Fatal(msg string, fields ...zap.Field) {
	L.Fatal(msg, fields...)
}

// String is a shortcut for zap.String.
func String(key, val string) zap.Field {
	return zap.String(key, val)
}

// Int is a shortcut for zap.Int.
func Int(key string, val int) zap.Field {
	return zap.Int(key, val)
}

// Int64 is a shortcut for zap.Int64.
func Int64(key string, val int64) zap.Field {
	return zap.Int64(key, val)
}

// Bool is a shortcut for zap.Bool.
func Bool(key string, val bool) zap.Field {
	return zap.Bool(key, val)
}

// Err is a shortcut for zap.Error.
func Err(err error) zap.Field {
	return zap.Error(err)
}

// Any is a shortcut for zap.Any.
func Any(key string, val any) zap.Field {
	return zap.Any(key, val)
}
