package database

import (
	"context"
	"log/slog"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"gostartv2/internal/config"
)

func mustStartPostgresContainer(t *testing.T) (config.DBConfig, func()) {
	t.Helper()

	if _, err := os.Stat("/var/run/docker.sock"); err != nil {
		t.Skipf("docker socket not found, skipping integration test: %v", err)
	}

	conn, err := net.Dial("unix", "/var/run/docker.sock")
	if err != nil {
		t.Skipf("docker socket not accessible, skipping integration test: %v", err)
	}
	_ = conn.Close()

	dbCfg := config.DBConfig{
		Name:     "database",
		User:     "user",
		Password: "password",
		SSLMode:  "disable",
		Schema:   "public",
		MaxConns: 25,
		MaxIdle:  5,
	}

	dbContainer, err := postgres.Run(
		context.Background(),
		"postgres:latest",
		postgres.WithDatabase(dbCfg.Name),
		postgres.WithUsername(dbCfg.User),
		postgres.WithPassword(dbCfg.Password),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
	)
	if err != nil {
		t.Fatalf("could not start postgres container: %v", err)
	}

	dbHost, err := dbContainer.Host(context.Background())
	if err != nil {
		t.Fatalf("could not get container host: %v", err)
	}

	dbPort, err := dbContainer.MappedPort(context.Background(), "5432/tcp")
	if err != nil {
		t.Fatalf("could not get mapped port: %v", err)
	}

	dbCfg.Host = dbHost
	dbCfg.Port = 5432
	if portStr := dbPort.Port(); portStr != "" {
		if n, err := strconv.Atoi(portStr); err == nil {
			dbCfg.Port = n
		}
	}

	teardown := func() {
		if err := dbContainer.Terminate(context.Background()); err != nil {
			t.Fatalf("could not teardown postgres container: %v", err)
		}
	}

	return dbCfg, teardown
}

func TestNew(t *testing.T) {
	dbCfg, teardown := mustStartPostgresContainer(t)
	defer teardown()

	srv, err := New(dbCfg, slog.Default())
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if srv == nil {
		t.Fatal("New() returned nil")
	}
}

func TestHealth(t *testing.T) {
	dbCfg, teardown := mustStartPostgresContainer(t)
	defer teardown()

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
	dbCfg, teardown := mustStartPostgresContainer(t)
	defer teardown()

	srv, err := New(dbCfg, slog.Default())
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	if err := srv.Close(); err != nil {
		t.Fatalf("expected Close() to return nil, got %v", err)
	}
}
