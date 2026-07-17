package testutil

import (
	"context"
	"database/sql"
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

	"gostartv2/internal/config"
	"gostartv2/migrations"
)

const postgresImage = "postgres:16-alpine"

func SkipIfNoDocker(t *testing.T) {
	t.Helper()

	if _, err := os.Stat("/var/run/docker.sock"); err != nil {
		t.Skipf("docker socket not found, skipping integration test: %v", err)
	}
	conn, err := net.Dial("unix", "/var/run/docker.sock")
	if err != nil {
		t.Skipf("docker socket not accessible, skipping integration test: %v", err)
	}
	_ = conn.Close()
}

func StartPostgres(t *testing.T) (config.DBConfig, func()) {
	t.Helper()
	SkipIfNoDocker(t)

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
		context.Background(),
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

	host, err := container.Host(context.Background())
	if err != nil {
		t.Fatalf("could not get container host: %v", err)
	}

	port, err := container.MappedPort(context.Background(), "5432/tcp")
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

	teardown := func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Fatalf("could not teardown postgres container: %v", err)
		}
	}

	return dbCfg, teardown
}

func SetupTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()

	dbCfg, containerTeardown := StartPostgres(t)

	db, err := sql.Open("pgx", dbCfg.DSN())
	if err != nil {
		containerTeardown()
		t.Fatalf("could not open db: %v", err)
	}

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		_ = db.Close()
		containerTeardown()
		t.Fatalf("set goose dialect: %v", err)
	}
	if err := goose.Up(db, "."); err != nil {
		_ = db.Close()
		containerTeardown()
		t.Fatalf("apply migrations: %v", err)
	}

	teardown := func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close test db: %v", err)
		}
		containerTeardown()
	}

	return db, teardown
}
