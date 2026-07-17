package server

import (
	"gostartv2/internal/httpx"
	"gostartv2/internal/middleware"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

const statusKey = "status"

// RegisterRoutes builds the chi router used by the API server: it installs
// the global RequestID, Logger, Recoverer, and CORS middleware, exposes the
// liveness and readiness endpoints, and mounts the /api/v1 user routes on
// the injected UserHandler. It is called once from NewServer to obtain the
// http.Handler passed to http.Server.
func (s *Server) RegisterRoutes() http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(middleware.Logger())
	r.Use(middleware.Recoverer())
	r.Use(middleware.CORS(s.cfg.CORS))

	r.Get("/", s.helloHandler)
	r.Get("/health", s.healthHandler)
	r.Get("/ready", s.readyHandler)

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/users", func(r chi.Router) {
			r.Get("/", s.userHandler.List)
			r.Post("/", s.userHandler.Create)
			r.Get("/{id}", s.userHandler.Get)
			r.Put("/{id}", s.userHandler.Update)
			r.Delete("/{id}", s.userHandler.Delete)
		})
	})

	return r
}

func (s *Server) helloHandler(w http.ResponseWriter, r *http.Request) {
	httpx.RespondJSON(w, http.StatusOK, map[string]string{
		"message": "Hello World",
	})
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	httpx.RespondJSON(w, http.StatusOK, map[string]string{
		statusKey: "ok",
	})
}

func (s *Server) readyHandler(w http.ResponseWriter, r *http.Request) {
	stats := s.db.Health()

	if stats[statusKey] != "up" {
		httpx.RespondError(w, http.StatusServiceUnavailable, "db_unavailable", "database is not reachable")
		return
	}

	httpx.RespondJSON(w, http.StatusOK, map[string]string{
		statusKey: "ready",
	})
}
