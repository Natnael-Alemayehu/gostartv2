package middleware

import (
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"gostartv2/internal/config"
)

func Recoverer() func(http.Handler) http.Handler {
	return middleware.Recoverer
}

func RequestID() func(http.Handler) http.Handler {
	return middleware.RequestID
}

func CORS(cfg config.CORSConfig) func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   cfg.AllowedMethods,
		AllowedHeaders:   cfg.AllowedHeaders,
		AllowCredentials: cfg.AllowCredentials,
		MaxAge:           cfg.MaxAge,
	})
}

func Logger() func(http.Handler) http.Handler {
	return middleware.Logger
}
