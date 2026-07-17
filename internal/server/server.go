package server

import (
	"fmt"
	"net/http"
	"time"

	"log/slog"

	"gostartv2/internal/config"
	"gostartv2/internal/database"
	"gostartv2/internal/handlers"
	"gostartv2/internal/repositories"
	"gostartv2/internal/services"
)

type Server struct {
	cfg         *config.Config
	logger      *slog.Logger
	db          database.Service
	userHandler *handlers.UserHandler
}

func NewServer(cfg *config.Config, logger *slog.Logger, db database.Service) *http.Server {
	repos := repositories.NewRepositories(db.DB())
	userSvc := services.NewUserService(repos.Users)

	s := &Server{
		cfg:         cfg,
		logger:      logger,
		db:          db,
		userHandler: handlers.NewUserHandler(userSvc),
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
