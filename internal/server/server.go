// Package server assembles the application's HTTP server: it wires
// repositories, services, and handlers together, registers routes, and
// configures the underlying http.Server with sane timeouts.
package server

import (
	"fmt"
	"gostartv2/internal/auth"
	"gostartv2/internal/config"
	"gostartv2/internal/database"
	"gostartv2/internal/handlers"
	"gostartv2/internal/repositories"
	"gostartv2/internal/services"
	"log/slog"
	"net/http"
	"time"
)

// Server holds the long-lived dependencies shared across request handling,
// including configuration, logging, the database service, and the mounted
// HTTP handlers. It is constructed once at startup and read concurrently
// while serving requests.
type Server struct {
	cfg         *config.Config
	logger      *slog.Logger
	db          database.Service
	userHandler *handlers.UserHandler
	authHandler *handlers.AuthHandler
	verifier    *auth.Verifier
}

// NewServer builds the dependency graph from the database up to the handlers,
// registers routes, and returns a configured *http.Server ready to be served.
// It is intended to be called once during application bootstrap.
func NewServer(cfg *config.Config, logger *slog.Logger, db database.Service) *http.Server {
	repos := repositories.NewRepositories(db.DB())
	userSvc := services.NewUserService(repos.Users)

	signer := auth.NewSigner(cfg.JWT.Secret, cfg.JWT.Issuer, cfg.JWT.AccessTTL)
	verifier := auth.NewVerifier(cfg.JWT.Secret, cfg.JWT.Issuer)
	authSvc := services.NewAuthService(repos.Users, repos.RefreshTokens, signer, cfg.JWT.RefreshTTL)

	s := &Server{
		cfg:         cfg,
		logger:      logger,
		db:          db,
		userHandler: handlers.NewUserHandler(userSvc),
		authHandler: handlers.NewAuthHandler(
			authSvc,
			verifier,
			cfg.JWT.AccessTTL,
			cfg.JWT.RefreshTTL,
			cfg.IsProd,
		),
		verifier: verifier,
	}

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      s.RegisterRoutes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return server
}
