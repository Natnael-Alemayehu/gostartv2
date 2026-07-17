//go:build integration

package database

import (
	"context"
	"gostartv2/internal/testutil"
	"log/slog"
	"testing"
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

	stats := srv.Health(context.Background())

	if stats["status"] != "up" {
		t.Fatalf("expected status to be up, got %s", stats["status"])
	}

	if _, ok := stats["error"]; ok {
		t.Fatalf("expected error not to be present, got %s", stats["error"])
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
