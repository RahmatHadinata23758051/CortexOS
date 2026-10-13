package logging

import (
	"context"
	"log/slog"
	"os"
)

// Logger is the minimal structured logging interface for CortexOS.
// Implementations should produce JSON output with stable field names.
type Logger interface {
	DebugContext(ctx context.Context, msg string, args ...any)
	InfoContext(ctx context.Context, msg string, args ...any)
	WarnContext(ctx context.Context, msg string, args ...any)
	ErrorContext(ctx context.Context, msg string, args ...any)
}

// NopLogger discards all log output.
type NopLogger struct{}

func (NopLogger) DebugContext(_ context.Context, _ string, _ ...any) {}
func (NopLogger) InfoContext(_ context.Context, _ string, _ ...any)  {}
func (NopLogger) WarnContext(_ context.Context, _ string, _ ...any)  {}
func (NopLogger) ErrorContext(_ context.Context, _ string, _ ...any) {}

// StdLogger wraps slog.DefaultLogger to satisfy the Logger interface.
type StdLogger struct {
	*slog.Logger
}

func (l StdLogger) DebugContext(ctx context.Context, msg string, args ...any) {
	l.Log(ctx, slog.LevelDebug, msg, args...)
}
func (l StdLogger) InfoContext(ctx context.Context, msg string, args ...any) {
	l.Log(ctx, slog.LevelInfo, msg, args...)
}
func (l StdLogger) WarnContext(ctx context.Context, msg string, args ...any) {
	l.Log(ctx, slog.LevelWarn, msg, args...)
}
func (l StdLogger) ErrorContext(ctx context.Context, msg string, args ...any) {
	l.Log(ctx, slog.LevelError, msg, args...)
}

// DefaultLogger returns a process-global Logger using slog with JSON output.
// It is safe to call multiple times; the first call configures the global slog default.
func DefaultLogger() Logger {
	if l := slog.Default(); l != nil {
		return StdLogger{Logger: l}
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler).With(
		slog.String("service", "cortexos"),
		slog.String("version", "0.1.0"),
	)
	slog.SetDefault(logger)
	return StdLogger{Logger: logger}
}

// With adds key-value pairs to a logger, returning a new Logger.
func With(l Logger, args ...any) Logger {
	if sl, ok := l.(StdLogger); ok {
		return StdLogger{Logger: sl.With(args...)}
	}
	return l
}