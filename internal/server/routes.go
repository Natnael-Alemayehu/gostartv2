package server

import (
	"gostartv2/internal/httpx"
	"gostartv2/internal/middleware"
	"net/http"

	"github.com/go-chi/chi/v5"
)

const statusKey = "status"

// RegisterRoutes builds the chi router used by the API server: it installs
// the global RequestID, Logger, Recoverer, and CORS middleware, exposes the
// liveness and readiness endpoints, mounts the public /api/v1/auth routes,
// and protects the user CRUD routes (except POST /users, which stays public
// as the registration endpoint) with the Auth middleware. It is called once
// from NewServer to obtain the http.Handler passed to http.Server.
func (s *Server) RegisterRoutes() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID())
	r.Use(middleware.Logger())
	r.Use(middleware.Recoverer())
	r.Use(middleware.CORS(s.cfg.CORS))

	r.Get("/", s.helloHandler)
	r.Get("/health", s.healthHandler)
	r.Get("/ready", s.readyHandler)

	r.Route("/api/v1", func(r chi.Router) {
		// Public auth endpoints. login and refresh rely on credentials /
		// cookies; logout-all mounts behind the Auth middleware so the
		// caller must supply a valid access token.
		r.Route("/auth", func(r chi.Router) {
			r.Post("/login", s.authHandler.Login)
			r.Post("/refresh", s.authHandler.Refresh)
			r.Post("/logout", s.authHandler.Logout)
			r.Group(func(r chi.Router) {
				r.Use(middleware.Auth(s.verifier))
				r.Post("/logout-all", s.authHandler.LogoutAll)
			})
		})

		// User resource: POST stays public as registration; GET/PUT/DELETE
		// require a valid access token whose user id may be extracted via
		// middleware.UserFromContext.
		r.Route("/users", func(r chi.Router) {
			r.Post("/", s.userHandler.Create)

			r.Group(func(r chi.Router) {
				r.Use(middleware.Auth(s.verifier))
				r.Get("/", s.userHandler.List)
				r.Get("/{id}", s.userHandler.Get)
				r.Put("/{id}", s.userHandler.Update)
				r.Delete("/{id}", s.userHandler.Delete)
			})
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
	stats := s.db.Health(r.Context())

	if stats[statusKey] != "up" {
		httpx.RespondError(w, http.StatusServiceUnavailable, "db_unavailable", "database is not reachable")
		return
	}

	httpx.RespondJSON(w, http.StatusOK, map[string]string{
		statusKey: "ready",
	})
}
