package server

import (
	"fmt"
	"net/http"
	"time"

	"log/slog"

	"gostartv2/internal/config"
	"gostartv2/internal/database"
)

type Server struct {
	cfg    *config.Config
	logger *slog.Logger
	db     database.Service
}

func NewServer(cfg *config.Config, logger *slog.Logger, db database.Service) *http.Server {
	s := &Server{
		cfg:    cfg,
		logger: logger,
		db:     db,
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
