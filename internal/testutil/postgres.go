// Package testutil provides shared helpers for integration tests that need
// a real PostgreSQL instance. It spins up an ephemeral Postgres container
// via testcontainers-go and applies the project's goose migrations so each
// test starts from a clean, schema-complete database.
package testutil

import (
	"context"
	"database/sql"
	"gostartv2/internal/config"
	"gostartv2/migrations"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const postgresImage = "postgres:16-alpine"

// SkipIfNoDocker skips the calling test when the Docker daemon is unreachable.
// It verifies both that the socket file exists and that it can be dialed, so
// tests fail fast with a clear skip reason instead of hanging on container
// startup. Call it at the top of any test that depends on testcontainers.
func SkipIfNoDocker(t *testing.T) {
	t.Helper()

	if _, err := os.Stat("/var/run/docker.sock"); err != nil {
		t.Skipf("docker socket not found, skipping integration test: %v", err)
	}

	dialer := net.Dialer{Timeout: 5 * time.Second}

	conn, err := dialer.DialContext(t.Context(), "unix", "/var/run/docker.sock")
	if err != nil {
		t.Skipf("docker socket not accessible, skipping integration test: %v", err)
	}

	_ = conn.Close()
}

// StartPostgres launches an ephemeral postgres:16-alpine container via
// testcontainers-go and returns a config.DBConfig pointing at it. The
// container is terminated automatically via t.Cleanup, so callers never
// need to tear it down themselves. Use this when a test needs the database
// but wants to manage migrations and connections on its own.
func StartPostgres(t *testing.T) config.DBConfig {
	t.Helper()
	SkipIfNoDocker(t)

	ctx := t.Context()

	dbCfg := config.DBConfig{
		Name:     "database",
		User:     "user",
		Password: "password",
		SSLMode:  "disable",
		Schema:   "public",
		MaxConns: 25,
		MaxIdle:  5,
	}

	container, err := postgres.Run(
		ctx,
		postgresImage,
		postgres.WithDatabase(dbCfg.Name),
		postgres.WithUsername(dbCfg.User),
		postgres.WithPassword(dbCfg.Password),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(10*time.Second)),
	)
	if err != nil {
		t.Fatalf("could not start postgres container: %v", err)
	}

	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Fatalf("could not teardown postgres container: %v", err)
		}
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("could not get container host: %v", err)
	}

	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("could not get mapped port: %v", err)
	}

	dbCfg.Host = host

	dbCfg.Port = 5432
	if portStr := port.Port(); portStr != "" {
		if n, err := strconv.Atoi(portStr); err == nil {
			dbCfg.Port = n
		}
	}

	return dbCfg
}

// SetupTestDB is the one-call helper for integration tests: it starts a
// Postgres container, opens a *sql.DB against it, and applies the embedded
// goose migrations. Both the returned DB and the underlying container are
// cleaned up via t.Cleanup. Tests should call this once per subtest that
// needs an isolated database; it fatals on any setup failure.
func SetupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dbCfg := StartPostgres(t)

	db, err := sql.Open("pgx", dbCfg.DSN())
	if err != nil {
		t.Fatalf("could not open db: %v", err)
	}

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close test db: %v", err)
		}
	})

	goose.SetBaseFS(migrations.FS)

	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("set goose dialect: %v", err)
	}

	if err := goose.Up(db, "."); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	return db
}
