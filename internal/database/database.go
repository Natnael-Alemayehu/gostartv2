package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"gostartv2/internal/config"
)

type Service interface {
	Health() map[string]string
	Ping(ctx context.Context) error
	Close() error
	DB() *sql.DB
}

type service struct {
	db     *sql.DB
	logger *slog.Logger
}

func New(cfg config.DBConfig, logger *slog.Logger) (Service, error) {
	db, err := sql.Open("pgx", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("database open: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxConns)
	db.SetMaxIdleConns(cfg.MaxIdle)
	db.SetConnMaxLifetime(time.Hour)

	s := &service{
		db:     db,
		logger: logger,
	}

	return s, nil
}

func (s *service) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *service) Health() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
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

func (s *service) Close() error {
	s.logger.Info("disconnecting from database")
	return s.db.Close()
}

func (s *service) DB() *sql.DB {
	return s.db
}
