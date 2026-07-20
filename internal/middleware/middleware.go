// Package middleware provides thin wrappers around common HTTP middleware
// used by the server, including panic recovery, request IDs, CORS, structured
// request logging, and authentication. Wrapping the chi and cors defaults here
// keeps route wiring in the server package free of third-party imports and
// gives a single place to adjust middleware behavior.
package middleware

import (
	"gostartv2/internal/config"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// Recoverer returns an HTTP middleware that catches panics in downstream
// handlers, logs the stack trace, and prevents the process from crashing.
// Use it as the outermost middleware so every request is protected.
func Recoverer() func(http.Handler) http.Handler {
	return middleware.Recoverer
}

// RequestID returns an HTTP middleware that injects a unique request ID into
// the request context so it can be propagated into logs and responses for
// tracing a single request across layers.
func RequestID() func(http.Handler) http.Handler {
	return middleware.RequestID
}

// CORS returns an HTTP middleware that applies the cross-origin resource
// sharing policy described by cfg, controlling which origins, methods, and
// headers may access the API from browsers.
func CORS(cfg config.CORSConfig) func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   cfg.AllowedMethods,
		AllowedHeaders:   cfg.AllowedHeaders,
		AllowCredentials: cfg.AllowCredentials,
		MaxAge:           cfg.MaxAge,
	})
}

// Logger returns an HTTP middleware that logs each request using slog with
// structured key-value pairs: method, path, status, duration, and request_id.
// In production (JSON handler) this produces JSON log lines compatible with
// log aggregators; in development (text handler) it produces readable text.
func Logger() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			duration := time.Since(start)

			slog.Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", duration.Milliseconds(),
				"request_id", middleware.GetReqID(r.Context()),
			)
		})
	}
}
