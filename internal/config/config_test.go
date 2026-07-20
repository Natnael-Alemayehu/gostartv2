package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	for _, key := range []string{
		"APP_ENV", "PORT", "DB_HOST", "DB_PORT", "DB_NAME", "DB_USER",
		"DB_PASSWORD", "DB_SCHEMA", "DB_SSLMODE", "DB_MAX_CONNS",
		"DB_MAX_IDLE", "JWT_SECRET", "JWT_ACCESS_TTL", "JWT_REFRESH_TTL",
		"JWT_ISSUER", "CORS_ALLOWED_ORIGINS", "CORS_ALLOW_CREDENTIALS",
	} {
		t.Setenv(key, "")
		os.Unsetenv(key) //nolint:errcheck // test setup: best-effort unset
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.AppEnv != EnvLocal {
		t.Errorf("AppEnv: got %q want %q", cfg.AppEnv, EnvLocal)
	}

	if cfg.Port != 8080 {
		t.Errorf("Port: got %d want 8080", cfg.Port)
	}

	if cfg.IsProd {
		t.Error("IsProd should be false for local")
	}

	if !cfg.IsDev {
		t.Error("IsDev should be true for local")
	}

	if cfg.DB.Host != "localhost" {
		t.Errorf("DB.Host: got %q want localhost", cfg.DB.Host)
	}

	if cfg.DB.Port != 5432 {
		t.Errorf("DB.Port: got %d want 5432", cfg.DB.Port)
	}

	if cfg.DB.MaxConns != 25 {
		t.Errorf("DB.MaxConns: got %d want 25", cfg.DB.MaxConns)
	}

	if cfg.JWT.AccessTTL != 15*time.Minute {
		t.Errorf("JWT.AccessTTL: got %v want 15m", cfg.JWT.AccessTTL)
	}

	if cfg.JWT.RefreshTTL != 168*time.Hour {
		t.Errorf("JWT.RefreshTTL: got %v want 168h", cfg.JWT.RefreshTTL)
	}

	if cfg.JWT.Issuer != "gostartv2" {
		t.Errorf("JWT.Issuer: got %q want gostartv2", cfg.JWT.Issuer)
	}
}

func TestLoad_ProductionRequiresJWTSecret(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://example.com")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing JWT_SECRET in production")
	}
}

func TestLoad_ProductionRejectsShortJWTSecret(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "short")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://example.com")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for short JWT_SECRET in production")
	}
}

func TestLoad_ProductionRejectsWildcardCORS(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "at-least-16-chars-long-secret")
	t.Setenv("CORS_ALLOWED_ORIGINS", "*")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for wildcard CORS in production")
	}
}

func TestLoad_ProductionValid(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "at-least-16-chars-long-secret")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if !cfg.IsProd {
		t.Error("IsProd should be true for production")
	}

	if cfg.IsDev {
		t.Error("IsDev should be false for production")
	}
}

func TestDBConfig_DSN(t *testing.T) {
	cfg := DBConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
		Schema:   "public",
		SSLMode:  "disable",
	}

	dsn := cfg.DSN()

	//nolint:gosec // G101: test fixture with placeholder credentials, not real secrets
	want := "postgres://testuser:testpass@localhost:5432/testdb?sslmode=disable&search_path=public"
	if dsn != want {
		t.Errorf("DSN: got %q want %q", dsn, want)
	}
}
