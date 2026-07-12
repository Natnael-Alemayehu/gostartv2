package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type AppEnv string

const (
	EnvLocal   AppEnv = "local"
	EnvDev     AppEnv = "development"
	EnvProd    AppEnv = "production"
	EnvStaging AppEnv = "staging"
	EnvTest    AppEnv = "test"
)

type Config struct {
	AppEnv AppEnv
	Port   int
	IsProd bool
	IsDev  bool

	DB   DBConfig
	JWT  JWTConfig
	CORS CORSConfig
}

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

type JWTConfig struct {
	Secret     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
	Issuer     string
}

type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

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

func (c *Config) Validate() error {
	var missing []string

	if c.DB.Host == "" {
		missing = append(missing, "DB_HOST")
	}
	if c.DB.Name == "" {
		missing = append(missing, "DB_NAME")
	}
	if c.DB.User == "" {
		missing = append(missing, "DB_USER")
	}

	if c.IsProd {
		if c.JWT.Secret == "" {
			missing = append(missing, "JWT_SECRET (required in production)")
		}
		if len(c.CORS.AllowedOrigins) == 1 && c.CORS.AllowedOrigins[0] == "*" {
			return fmt.Errorf("CORS_ALLOWED_ORIGINS must not be wildcard in production")
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return nil
}

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
