package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"gostartv2/internal/config"
	"gostartv2/internal/database"
	"gostartv2/internal/logging"
	"gostartv2/internal/server"
)

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
		panic(fmt.Sprintf("failed to load config: %s", err))
	}

	logger := logging.New(cfg)

	db, err := database.New(cfg.DB, logger)
	if err != nil {
		panic(fmt.Sprintf("failed to connect to database: %s", err))
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
