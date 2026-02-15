package log

import (
	"context"
	"log/slog"
)

type contextKey struct{}

var loggerKey = contextKey{}

// Ctx returns the logger from the context. If no logger is found, it returns the default logger.
func Ctx(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

// With returns a new context with the given logger.
func With(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}
