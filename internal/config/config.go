// Package config loads typed application configuration from environment variables
// and validates required values once at startup. It is the only package that
// reads os.Getenv or loads .env, keeping all environment access in one place.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// AppEnv identifies a deployment environment. It drives behavior switches such
// as JSON vs text logging and gates production-only validation rules.
type AppEnv string

const (
	// EnvLocal is the developer's local machine environment.
	EnvLocal AppEnv = "local"
	// EnvDev is a shared development environment for integration and QA.
	EnvDev AppEnv = "development"
	// EnvProd is the production environment serving real traffic.
	EnvProd AppEnv = "production"
	// EnvStaging is the pre-production environment mirroring production.
	EnvStaging AppEnv = "staging"
	// EnvTest is the automated test environment used by the test suite.
	EnvTest AppEnv = "test"
)

// Config holds all runtime configuration, grouped by subsystem. It is
// constructed once in main and passed by pointer to the components that need it.
type Config struct {
	AppEnv AppEnv
	Port   int
	IsProd bool
	IsDev  bool

	DB   DBConfig
	JWT  JWTConfig
	CORS CORSConfig
}

// DBConfig holds PostgreSQL connection parameters and pool sizing for the
// primary database. DSN renders the connection URL from these fields.
type DBConfig struct {
	Host     string
	Port     int
	Name     string
	User     string
	Password string
	Schema   string
	SSLMode  string
	MaxConns int
	MaxIdle  int
}

// JWTConfig holds the signing secret and token lifetimes used by the auth
// package to mint and verify JWTs. Secret is required only in production.
type JWTConfig struct {
	Secret     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
	Issuer     string
}

// CORSConfig holds the cross-origin policy applied by the CORS middleware.
// In production AllowedOrigins must not be the "*" wildcard.
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

// Load reads configuration from the environment, applying sensible defaults
// for unset values. It loads .env via godotenv first, then builds and validates
// a Config. Call once during bootstrap; returns an error if required values are
// missing or production-only constraints are violated.
func Load() (*Config, error) {
	_ = godotenv.Load()

	appEnv := AppEnv(getenv("APP_ENV", "local"))

	cfg := &Config{
		AppEnv: appEnv,
		IsProd: appEnv == EnvProd,
		IsDev:  appEnv == EnvLocal || appEnv == EnvDev || appEnv == EnvTest,
		Port:   getenvInt("PORT", 8080),

		DB: DBConfig{
			Host:     getenv("DB_HOST", "localhost"),
			Port:     getenvInt("DB_PORT", 5432),
			Name:     getenv("DB_NAME", "gostartv2"),
			User:     getenv("DB_USER", "postgres"),
			Password: getenv("DB_PASSWORD", ""),
			Schema:   getenv("DB_SCHEMA", "public"),
			SSLMode:  getenv("DB_SSLMODE", "disable"),
			MaxConns: getenvInt("DB_MAX_CONNS", 25),
			MaxIdle:  getenvInt("DB_MAX_IDLE", 5),
		},

		JWT: JWTConfig{
			Secret:     getenv("JWT_SECRET", ""),
			AccessTTL:  getenvDuration("JWT_ACCESS_TTL", 15*time.Minute),
			RefreshTTL: getenvDuration("JWT_REFRESH_TTL", 168*time.Hour),
			Issuer:     getenv("JWT_ISSUER", "gostartv2"),
		},

		CORS: CORSConfig{
			AllowedOrigins:   getenvSlice("CORS_ALLOWED_ORIGINS", []string{"*"}),
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
			AllowCredentials: getenvBool("CORS_ALLOW_CREDENTIALS", false),
			MaxAge:           300,
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate reports every missing required value in a single error so
// misconfiguration is diagnosed in one pass. In production it additionally
// enforces a non-wildcard CORS origin and a JWT secret of at least 16 bytes.
func (c *Config) Validate() error {
	var missing []string

	if os.Getenv("DB_HOST") == "" && c.DB.Host == "" {
		missing = append(missing, "DB_HOST")
	}

	if os.Getenv("DB_NAME") == "" && c.DB.Name == "" {
		missing = append(missing, "DB_NAME")
	}

	if os.Getenv("DB_USER") == "" && c.DB.User == "" {
		missing = append(missing, "DB_USER")
	}

	if c.IsProd {
		if c.JWT.Secret == "" {
			missing = append(missing, "JWT_SECRET (required in production)")
		} else if len(c.JWT.Secret) < 16 {
			missing = append(missing, "JWT_SECRET (must be at least 16 characters in production)")
		}

		if len(c.CORS.AllowedOrigins) == 1 && c.CORS.AllowedOrigins[0] == "*" {
			return errors.New("CORS_ALLOWED_ORIGINS must not be wildcard in production")
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return nil
}

// DSN renders the PostgreSQL connection URL for this DBConfig, including SSL
// mode and search path, suitable for sql.Open with the pgx driver.
func (c *DBConfig) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s&search_path=%s",
		c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode, c.Schema)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}

func getenvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}

	return fallback
}

func getenvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}

	return fallback
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}

	return fallback
}

func getenvSlice(key string, fallback []string) []string {
	if v := os.Getenv(key); v != "" {
		parts := strings.Split(v, ",")
		for i, p := range parts {
			parts[i] = strings.TrimSpace(p)
		}

		return parts
	}

	return fallback
}
