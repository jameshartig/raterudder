package log

import (
	"context"
	"log/slog"
	"os"
)

var (
	defaultLogLevel slog.LevelVar
	defaultLogger   = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     &defaultLogLevel,
	}))
)

func init() {
	defaultLogLevel.Set(slog.LevelInfo)
}

type contextKey struct{}

var loggerKey = contextKey{}

// Ctx returns the logger from the context. If no logger is found, it returns the default logger.
func Ctx(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return l
	}
	return defaultLogger
}

// With returns a new context with the given logger.
func With(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

func SetDefaultLogLevel(level slog.Level) {
	defaultLogLevel.Set(level)
}
