package handlers

import (
	"context"
	"errors"
	"gostartv2/internal/auth"
	"gostartv2/internal/httpx"
	"gostartv2/internal/middleware"
	"gostartv2/internal/services"
	"net/http"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

// authService is the consumer-side contract for the auth service. Defined
// here in the consumer package per the same pattern as userService above.
type authService interface {
	Login(ctx context.Context, email, password string) (access, refresh string, err error)
	Refresh(ctx context.Context, refresh string) (newAccess, newRefresh string, err error)
	Logout(ctx context.Context, refresh string) error
	LogoutAll(ctx context.Context, userID uuid.UUID) error
}

// AuthHandler exposes the auth flow as HTTP endpoints: login, refresh,
// logout, and logout-all.
type AuthHandler struct {
	svc        authService
	verifier   *auth.Verifier
	validator  *validator.Validate
	accessTTL  time.Duration
	refreshTTL time.Duration
	isProd     bool
}

// NewAuthHandler returns an AuthHandler backed by the given service. The
// verifier is reserved for future per-request access-token validation (the
// logout-all path currently relies on the auth middleware to inject the user
// id). accessTTL is surfaced in the response body's expires_in field;
// refreshTTL drives the cookie MaxAge attribute. isProd controls whether the
// refresh-token cookie is marked Secure.
func NewAuthHandler(
	svc authService,
	verifier *auth.Verifier,
	accessTTL time.Duration,
	refreshTTL time.Duration,
	isProd bool,
) *AuthHandler {
	return &AuthHandler{
		svc:        svc,
		verifier:   verifier,
		validator:  validator.New(),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		isProd:     isProd,
	}
}

type loginRequest struct {
	Email    string `json:"email" validate:"required,email,max=254"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

const refreshTokenCookie = "refresh_token"

// Login handles POST /api/v1/auth/login. It validates credentials, issues an
// access token and a refresh token, sets the refresh token in an HttpOnly
// cookie, and returns the access token in the JSON body.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeAndValidate(w, r, h.validator, &req) {
		return
	}

	access, refresh, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		respondAuthError(w, err)
		return
	}

	h.setRefreshCookie(w, refresh)
	httpx.RespondJSON(w, http.StatusOK, tokenResponse{
		AccessToken: access,
		TokenType:   "Bearer",
		ExpiresIn:   int(h.accessTTL.Seconds()),
	})
}

// Refresh handles POST /api/v1/auth/refresh. The presented refresh token is
// read from the cookie; the request must also carry a (possibly expired)
// Bearer access token header as a CSRF defense. On success the old refresh
// token is revoked, a new one is issued in the same chain, and a new access
// token is returned.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	refresh, ok := h.readRefreshCookie(r)
	if !ok {
		httpx.RespondError(w, http.StatusUnauthorized, "missing_refresh", "session expired")
		return
	}

	// CSRF defense: the request must carry an Authorization header. We do not
	// require the access token to be valid (it may have just expired) — only
	// that the client can produce one, which a CSRFattacker cannot.
	if _, ok := bearerToken(r); !ok {
		httpx.RespondError(w, http.StatusUnauthorized, "missing_token", "authorization required")
		return
	}

	newAccess, newRefresh, err := h.svc.Refresh(r.Context(), refresh)
	if err != nil {
		respondAuthError(w, err)
		return
	}

	h.setRefreshCookie(w, newRefresh)
	httpx.RespondJSON(w, http.StatusOK, tokenResponse{
		AccessToken: newAccess,
		TokenType:   "Bearer",
		ExpiresIn:   int(h.accessTTL.Seconds()),
	})
}

// Logout handles POST /api/v1/auth/logout. It revokes the refresh token in
// the cookie (if present) and clears the cookie. Idempotent.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	refresh, ok := h.readRefreshCookie(r)
	if ok {
		_ = h.svc.Logout(r.Context(), refresh)
	}

	h.clearRefreshCookie(w)
	httpx.RespondNoContent(w)
}

// LogoutAll handles POST /api/v1/auth/logout-all. It is mounted behind the
// auth middleware, so the user id is extracted from the request context.
// Every active refresh token owned by that user is revoked.
func (h *AuthHandler) LogoutAll(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.RespondError(w, http.StatusUnauthorized, "missing_token", "authorization required")
		return
	}

	if err := h.svc.LogoutAll(r.Context(), userID); err != nil {
		respondAuthError(w, err)
		return
	}

	h.clearRefreshCookie(w)
	httpx.RespondNoContent(w)
}

// setRefreshCookie writes the plaintext refresh token into an HttpOnly,
// SameSite=Lax cookie scoped to the auth route prefix. The Secure flag is
// governed by isProd so local development over plain http continues to work;
// in production the cookie is always marked Secure.
//
//nolint:gosec // G124: Secure flag is intentionally dev/prod-controlled
func (h *AuthHandler) setRefreshCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshTokenCookie,
		Value:    token,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   h.isProd,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(h.refreshTTL.Seconds()),
	})
}

// clearRefreshCookie overwrites the refresh cookie with a zero-maxage value
// so the client deletes it. The Secure flag follows the prod/dev setting to
// match the cookie that was set; a mismatched Secure flag would prevent the
// browser from honouring the deletion.
//
//nolint:gosec // G124: Secure flag is intentionally dev/prod-controlled
func (h *AuthHandler) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshTokenCookie,
		Value:    "",
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   h.isProd,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// readRefreshCookie returns the refresh token from the request cookie. The
// boolean is false when the cookie is missing or malformed.
func (h *AuthHandler) readRefreshCookie(r *http.Request) (string, bool) {
	c, err := r.Cookie(refreshTokenCookie)
	if err != nil {
		return "", false
	}

	if c.Value == "" {
		return "", false
	}

	return c.Value, true
}

// bearerToken extracts the access token from the Authorization header. The
// boolean is false when the header is missing or not of the form
// "Bearer <token>". Used as a CSRF defense on /refresh — we only need to
// confirm the client can produce an Authorization header, not that the token
// is valid.
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

// respondAuthError maps auth-service errors to HTTP responses following the
// same envelope shape used by respondServiceError in user_handler.go.
// Messages are deliberately generic to avoid disclosing token state to
// potential attackers.
func respondAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		httpx.RespondError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
	case errors.Is(err, auth.ErrRefreshTokenRevoked):
		httpx.RespondError(w, http.StatusUnauthorized, "session_expired", "session expired")
	case errors.Is(err, auth.ErrRefreshTokenReuse):
		httpx.RespondError(w, http.StatusUnauthorized, "session_revoked", "session expired")
	case errors.Is(err, auth.ErrRefreshTokenExpired):
		httpx.RespondError(w, http.StatusUnauthorized, "refresh_expired", "session expired")
	case errors.Is(err, services.ErrUserNotFound):
		// Login returns ErrInvalidCredentials before we get here, but keep
		// the mapping for safety.
		httpx.RespondError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
	default:
		httpx.RespondError(w, http.StatusInternalServerError, "internal_error", "something went wrong")
	}
}
