// Package database manages the PostgreSQL connection pool and exposes health
// and ping checks used by the readiness endpoint. It wraps database/sql with a
// pgx driver and sizes the pool according to the supplied config.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"gostartv2/internal/config"
	"log/slog"
	"time"
)

// Service is the contract a database wrapper must satisfy: liveness ping,
// health reporting, access to the underlying *sql.DB, and graceful close. It is
// declared here so callers depend only on the capabilities they use.
type Service interface {
	Health(ctx context.Context) map[string]string
	Ping(ctx context.Context) error
	Close() error
	DB() *sql.DB
}

type service struct {
	db     *sql.DB
	logger *slog.Logger
}

// New opens a pgx-backed *sql.DB using cfg.DSN, applies pool sizing and
// lifetime settings from cfg, and returns a Service ready for use. Returns an
// error if the driver cannot open the database.
func New(cfg config.DBConfig, logger *slog.Logger) (Service, error) {
	db, err := sql.Open("pgx", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("database open: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxConns)
	db.SetMaxIdleConns(cfg.MaxIdle)
	db.SetConnMaxLifetime(time.Hour)
	db.SetConnMaxIdleTime(5 * time.Minute)

	s := &service{
		db:     db,
		logger: logger,
	}

	return s, nil
}

// Ping verifies the database is reachable within the caller's deadline.
// Returns the underlying driver error on failure.
func (s *service) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// Health probes the database with a one-second timeout derived from ctx and
// returns a status map for the readiness endpoint: "status" is "up" or "down",
// with an "error" detail on failure. Pool statistics are intentionally omitted
// so internal metrics are not exposed publicly.
func (s *service) Health(ctx context.Context) map[string]string {
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	stats := make(map[string]string)

	err := s.db.PingContext(ctx)
	if err != nil {
		stats["status"] = "down"
		stats["error"] = fmt.Sprintf("db down: %v", err)

		return stats
	}

	stats["status"] = "up"
	stats["message"] = "It's healthy"

	return stats
}

// Close logs the disconnection and closes the underlying connection pool.
// Call it during graceful shutdown.
func (s *service) Close() error {
	s.logger.Info("disconnecting from database")
	return s.db.Close()
}

// DB returns the underlying *sql.DB so callers such as repositories can run
// queries and transactions directly.
func (s *service) DB() *sql.DB {
	return s.db
}
