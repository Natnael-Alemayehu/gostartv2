// Package main is the entry point for the gostartv2 API server binary.
// It loads configuration, initializes the logger and database, wires up the
// HTTP server, and runs graceful shutdown on SIGINT/SIGTERM.
package main

import (
	"context"
	"fmt"
	"gostartv2/internal/config"
	"gostartv2/internal/database"
	"gostartv2/internal/logging"
	"gostartv2/internal/server"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Version is the build version of the API server binary. It is overridden at
// link time (e.g. -ldflags "-X main.Version=...") by the release build and
// defaults to "dev" for local builds. It is logged at startup.
var Version = "dev"

func gracefulShutdown(apiServer *http.Server, db database.Service, logger *slog.Logger, done chan bool) {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()

	logger.Info("shutting down gracefully, press Ctrl+C again to force")
	stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := apiServer.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", "error", err)
	}

	if db != nil {
		if err := db.Close(); err != nil {
			logger.Error("database close error", "error", err)
		}
	}

	logger.Info("server exiting")

	done <- true
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(fmt.Sprintf("load config: %s", err))
	}

	logger := logging.New(cfg)

	db, err := database.New(cfg.DB, logger)
	if err != nil {
		panic(fmt.Sprintf("connect to database: %s", err))
	}

	apiServer := server.NewServer(cfg, logger, db)

	logger.Info("starting server", "port", cfg.Port, "env", cfg.AppEnv, "version", Version)

	done := make(chan bool, 1)
	go gracefulShutdown(apiServer, db, logger, done)

	if err := apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		panic(fmt.Sprintf("http server error: %s", err))
	}

	<-done
	logger.Info("graceful shutdown complete")
}
