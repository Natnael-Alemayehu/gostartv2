package middleware

import (
	"context"
	"errors"
	"gostartv2/internal/auth"
	"gostartv2/internal/httpx"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// ctxKey is an unexported type used for context value keys. Unexported so
// other packages cannot collide with our keys, per golang-context skill.
type ctxKey int

const (
	userKey ctxKey = iota
)

// userValue is the unexported value shape stored under userKey. Fields are
// unexported too so only this package can construct one; callers use the
// typed accessor UserFromContext.
type userValue struct {
	userID  uuid.UUID
	chainID uuid.UUID
}

// Verifier is the consumer-side contract for an access-token verifier. The
// auth package's *auth.Verifier satisfies this interface, so the middleware
// can be tested with a fake verifier without depending on the auth package's
// concrete type. Compile-time check at the bottom of this file.
type Verifier interface {
	Verify(tokenStr string) (*auth.Claims, error)
}

// Auth returns an HTTP middleware that requires a valid Bearer access token.
// On success the user id and chain id are injected into the request context
// and available via UserFromContext. On failure a 401 is written with the
// appropriate error code.
func Auth(v Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr, ok := bearerToken(r)
			if !ok {
				httpx.RespondError(w, http.StatusUnauthorized, "missing_token", "authorization required")
				return
			}

			claims, err := v.Verify(tokenStr)
			if err != nil {
				respondAuthError(w, err)
				return
			}

			ctx := withUser(r.Context(), claims.UserID, claims.ChainID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserFromContext returns the user id and chain id extracted by the Auth
// middleware. The boolean is false when no user is present, which usually
// indicates the Auth middleware was not applied to the route.
func UserFromContext(ctx context.Context) (uuid.UUID, uuid.UUID, bool) {
	v, ok := ctx.Value(userKey).(userValue)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}

	return v.userID, v.chainID, true
}

// withUser stores the user id and chain id in the context. Unexported so only
// this package can construct the value, preventing key collisions.
func withUser(ctx context.Context, userID, chainID uuid.UUID) context.Context {
	return context.WithValue(ctx, userKey, userValue{
		userID:  userID,
		chainID: chainID,
	})
}

// bearerToken extracts the access token from the Authorization header. The
// boolean is false when the header is missing or not of the form
// "Bearer <token>".
func bearerToken(r *http.Request) (string, bool) {
	authz := r.Header.Get("Authorization")
	if authz == "" {
		return "", false
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(authz, prefix) {
		return "", false
	}

	token := strings.TrimPrefix(authz, prefix)
	if token == "" {
		return "", false
	}

	return token, true
}

// respondAuthError maps verifier errors to HTTP responses using the same
// envelope shape as the rest of the application.
func respondAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrTokenExpired):
		httpx.RespondError(w, http.StatusUnauthorized, "token_expired", "token expired")
	default:
		httpx.RespondError(w, http.StatusUnauthorized, "token_invalid", "invalid token")
	}
}

// Compile-time check: *auth.Verifier satisfies our Verifier interface.
var _ Verifier = (*auth.Verifier)(nil)
