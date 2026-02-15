package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jameshartig/raterudder/pkg/ess"
	"github.com/jameshartig/raterudder/pkg/log"
	"github.com/jameshartig/raterudder/pkg/server"
	"github.com/jameshartig/raterudder/pkg/storage"
	"github.com/jameshartig/raterudder/pkg/utility"

	"github.com/levenlabs/go-lflag"
	"github.com/levenlabs/go-llog"
)

func main() {
	// init packages
	u := utility.Configured()
	e := ess.Configured()
	s := storage.Configured()

	// init server
	srv := server.Configured(u, e, s)

	// parse flags
	lflag.Configure()

	var level slog.Level
	// lflag automatically sets llog's level, but we need to set the slog level
	switch llog.GetLevel() {
	case llog.DebugLevel:
		level = slog.LevelDebug
	case llog.InfoLevel:
		level = slog.LevelInfo
	case llog.WarnLevel:
		level = slog.LevelWarn
	case llog.ErrorLevel:
		level = slog.LevelError
	default:
		panic(fmt.Errorf("unknown log level: %s", llog.GetLevel().String()))
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
	slog.SetDefault(logger)
	slog.Debug("logger configured", slog.String("level", level.String()))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// 4. Defer Close on storage
	// If initialization inside lflag.Do failed, we wouldn't be here (panic).
	defer func() {
		if err := s.Close(); err != nil {
			log.Ctx(ctx).ErrorContext(ctx, "failed to close storage", "error", err)
		}
	}()

	// 5. Start Server
	// Run will block until context is canceled or error happens
	if err := srv.Run(ctx); err != nil {
		log.Ctx(ctx).ErrorContext(ctx, "server failed", "error", err)
		os.Exit(1)
	}
	log.Ctx(ctx).InfoContext(ctx, "server exited cleanly")
}
