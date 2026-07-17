// Package logging configures the application's slog logger and propagates it
// through context, so handlers and services can retrieve a request-scoped
// logger without taking a direct dependency on a specific logger instance.
package logging

import (
	"context"
	"gostartv2/internal/config"
	"io"
	"log/slog"
	"os"
)

type ctxKey struct{}

// New builds the default slog.Logger from cfg, selecting a JSON handler in
// production and a text handler with source locations otherwise, then registers
// it via slog.SetDefault. Call exactly once during bootstrap in main.
func New(cfg *config.Config) *slog.Logger {
	var handler slog.Handler

	opts := &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: cfg.IsDev,
	}

	if cfg.IsProd {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger
}

// FromContext returns the logger stored in ctx by WithLogger, falling back to
// slog.Default() when none is present. It never returns nil.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok {
		return l
	}

	return slog.Default()
}

// WithLogger returns a derived context carrying logger so downstream code can
// retrieve it via FromContext. Use it in middleware to attach a request-scoped
// logger.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, logger)
}

// NewHandler builds a slog.Logger writing to w, mirroring New's handler
// selection but without registering it as the default. Use it when a component
// such as an access-log middleware needs its own writer.
func NewHandler(w io.Writer, cfg *config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: cfg.IsDev,
	}

	var handler slog.Handler
	if cfg.IsProd {
		handler = slog.NewJSONHandler(w, opts)
	} else {
		handler = slog.NewTextHandler(w, opts)
	}

	return slog.New(handler)
}
