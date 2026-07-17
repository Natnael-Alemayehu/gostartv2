package database

import (
	"context"
	"gostartv2/internal/config"
	"log/slog"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestHealthDown(t *testing.T) {
	dbCfg := config.DBConfig{
		Host:     "localhost",
		Port:     1,
		Name:     "test",
		User:     "test",
		Password: "test",
		SSLMode:  "disable",
		Schema:   "public",
		MaxConns: 1,
		MaxIdle:  1,
	}

	srv, err := New(dbCfg, slog.Default())
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	stats := srv.Health(context.Background())

	if stats["status"] != "down" {
		t.Fatalf("expected status to be down, got %s", stats["status"])
	}

	if _, ok := stats["error"]; !ok {
		t.Fatal("expected error to be present")
	}
}
