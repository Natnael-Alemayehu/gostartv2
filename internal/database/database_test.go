package database

import (
	"gostartv2/internal/config"
	"gostartv2/internal/testutil"
	"log/slog"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestNew(t *testing.T) {
	dbCfg := testutil.StartPostgres(t)

	srv, err := New(dbCfg, slog.Default())
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	if srv == nil {
		t.Fatal("New() returned nil")
	}
}

func TestHealth(t *testing.T) {
	dbCfg := testutil.StartPostgres(t)

	srv, err := New(dbCfg, slog.Default())
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	stats := srv.Health()

	if stats["status"] != "up" {
		t.Fatalf("expected status to be up, got %s", stats["status"])
	}

	if _, ok := stats["error"]; ok {
		t.Fatalf("expected error not to be present, got %s", stats["error"])
	}
}

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

	stats := srv.Health()

	if stats["status"] != "down" {
		t.Fatalf("expected status to be down, got %s", stats["status"])
	}

	if _, ok := stats["error"]; !ok {
		t.Fatal("expected error to be present")
	}
}

func TestClose(t *testing.T) {
	dbCfg := testutil.StartPostgres(t)

	srv, err := New(dbCfg, slog.Default())
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	if err := srv.Close(); err != nil {
		t.Fatalf("expected Close() to return nil, got %v", err)
	}
}

func TestDB(t *testing.T) {
	dbCfg := testutil.StartPostgres(t)

	srv, err := New(dbCfg, slog.Default())
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	if srv.DB() == nil {
		t.Fatal("expected DB() to return non-nil *sql.DB")
	}

	if err := srv.DB().PingContext(t.Context()); err != nil {
		t.Fatalf("expected DB() to be pingable, got %v", err)
	}
}
