package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"gostartv2/internal/httpx"
	"gostartv2/internal/middleware"
)

func (s *Server) RegisterRoutes() http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(middleware.Logger())
	r.Use(middleware.Recoverer())
	r.Use(middleware.CORS(s.cfg.CORS))

	r.Get("/", s.helloHandler)
	r.Get("/health", s.healthHandler)
	r.Get("/ready", s.readyHandler)

	return r
}

func (s *Server) helloHandler(w http.ResponseWriter, r *http.Request) {
	httpx.RespondJSON(w, http.StatusOK, map[string]string{
		"message": "Hello World",
	})
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	httpx.RespondJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func (s *Server) readyHandler(w http.ResponseWriter, r *http.Request) {
	stats := s.db.Health()

	if stats["status"] != "up" {
		httpx.RespondError(w, http.StatusServiceUnavailable, "db_unavailable", "database is not reachable")
		return
	}

	httpx.RespondJSON(w, http.StatusOK, map[string]string{
		"status": "ready",
	})
}
