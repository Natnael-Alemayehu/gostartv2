package logging

import (
	"context"
	"io"
	"log/slog"
	"os"

	"gostartv2/internal/config"
)

type ctxKey struct{}

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

func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, logger)
}

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
