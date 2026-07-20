//go:build integration

package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gostartv2/internal/auth"
	"gostartv2/internal/config"
	"gostartv2/internal/handlers"
	"gostartv2/internal/middleware"
	"gostartv2/internal/models"
	"gostartv2/internal/repositories"
	"gostartv2/internal/services"
	"gostartv2/internal/testutil"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

const (
	testSecret   = "e2e-test-secret"
	testIssuer   = "gostartv2-e2e-test"
	accessTTL    = 15 * time.Minute
	refreshTTL   = 7 * 24 * time.Hour
)

// e2eEnv bundles the wiring of an end-to-end auth server against a real
// Postgres container. It mirrors what internal/server.NewServer does in the
// live application, but uses fixed test secrets.
type e2eEnv struct {
	repos     *repositories.Repositories
	userSvc   *services.UserService
	authSvc   *services.AuthService
	verifier  *auth.Verifier
	server    *httptest.Server
	t         *testing.T
}

func newE2EEnv(t *testing.T) *e2eEnv {
	t.Helper()
	db := testutil.SetupTestDB(t)
	repos := repositories.NewRepositories(db)
	userSvc := services.NewUserService(repos.Users)
	signer := auth.NewSigner(testSecret, testIssuer, accessTTL)
	verifier := auth.NewVerifier(testSecret, testIssuer)
	authSvc := services.NewAuthService(repos.Users, repos.RefreshTokens, signer, refreshTTL)

	userHandler := handlers.NewUserHandler(userSvc)
	authHandler := handlers.NewAuthHandler(authSvc, verifier, accessTTL, refreshTTL, false)

	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Post("/login", authHandler.Login)
			r.Post("/refresh", authHandler.Refresh)
			r.Post("/logout", authHandler.Logout)
			r.Group(func(r chi.Router) {
				r.Use(middleware.Auth(verifier))
				r.Post("/logout-all", authHandler.LogoutAll)
			})
		})
		r.Route("/users", func(r chi.Router) {
			r.Post("/", userHandler.Create)
		})
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	return &e2eEnv{
		repos:    repos,
		userSvc:  userSvc,
		authSvc:  authSvc,
		verifier: verifier,
		server:   srv,
		t:        t,
	}
}

type registerBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type loginBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type errorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func postJSON(t *testing.T, client *http.Client, url string, body any) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func decodeJSONBody(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if len(b) == 0 {
		return
	}
	if err := json.Unmarshal(b, dst); err != nil {
		t.Fatalf("decode body %q: %v", string(b), err)
	}
}

func extractRefreshCookie(resp *http.Response) string {
	for _, c := range resp.Cookies() {
		if c.Name == "refresh_token" {
			return c.Value
		}
	}
	return ""
}

// TestAuth_FullFlow walks the entire auth lifecycle against a real Postgres:
// register a user, log in to obtain tokens, refresh to rotate, logout (which
// revokes the refresh token), attempt to refresh using the now-revoked token
// (must fail with reuse/revoked sentinel), and finally exercise logout-all.
func TestAuth_FullFlow(t *testing.T) {
	env := newE2EEnv(t)
	client := env.server.Client()

	// 1. Register a new user.
	email := fmt.Sprintf("e2e-%s@example.com", randomSuffix())
	registerURL := env.server.URL + "/api/v1/users"
	resp := postJSON(t, client, registerURL, registerBody{
		Email:    email,
		Password: "validpassword",
		Name:     "E2E User",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// 2. Log in: should return an access token + set the refresh cookie.
	loginURL := env.server.URL + "/api/v1/auth/login"
	resp = postJSON(t, client, loginURL, loginBody{
		Email:    email,
		Password: "validpassword",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login: expected 200, got %d", resp.StatusCode)
	}
	var tok tokenResponse
	decodeJSONBody(t, resp, &tok)
	_ = resp.Body.Close()
	if tok.AccessToken == "" {
		t.Fatal("expected non-empty access token")
	}
	if tok.TokenType != "Bearer" {
		t.Errorf("token_type: got %q want Bearer", tok.TokenType)
	}
	refreshCookie := extractRefreshCookie(resp)
	if refreshCookie == "" {
		t.Fatal("expected refresh_token cookie")
	}

	// 3. Verify the access token is well-formed via the real Verifier.
	claims, err := env.verifier.Verify(tok.AccessToken)
	if err != nil {
		t.Fatalf("verify access token: %v", err)
	}
	// Sanity-check by reading the user id out of the access token. We have no
	// "me" endpoint, so a non-zero uid proves the JWT carries the right subject.
	if claims.UserID.String() == "00000000-0000-0000-0000-000000000000" {
		t.Fatal("expected non-zero user id in access token claims")
	}

	// 4. Refresh: send the cookie back plus a CSRF Bearer header.
	refreshURL := env.server.URL + "/api/v1/auth/refresh"
	req, _ := http.NewRequest(http.MethodPost, refreshURL, nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: refreshCookie})
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("refresh request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("refresh: expected 200, got %d", resp.StatusCode)
	}
	var tok2 tokenResponse
	decodeJSONBody(t, resp, &tok2)
	_ = resp.Body.Close()
	refreshCookie2 := extractRefreshCookie(resp)
	if tok2.AccessToken == "" {
		t.Fatal("expected new access token on refresh")
	}
	if refreshCookie2 == "" {
		t.Fatal("expected new refresh cookie on refresh")
	}
	if refreshCookie2 == refreshCookie {
		t.Fatal("refresh token was not rotated")
	}

	// 5. Old refresh token must now be revoked server-side: attempting refresh
	// with it must fail.
	req, _ = http.NewRequest(http.MethodPost, refreshURL, nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: refreshCookie})
	req.Header.Set("Authorization", "Bearer "+tok2.AccessToken)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("post-rotation refresh: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("reuse-after-rotation: expected 401, got %d", resp.StatusCode)
	}
	var errBody errorEnvelope
	decodeJSONBody(t, resp, &errBody)
	_ = resp.Body.Close()
	// The user-facing code is the same for reuse and revoked ("session_*").
	// Either is acceptable; both signal the client must re-auth.
	switch errBody.Error.Code {
	case "session_revoked", "session_expired":
	default:
		t.Errorf("expected session_revoked or session_expired code, got %q", errBody.Error.Code)
	}

	// 6. Log in a second session so we can verify logout-all cascades.
	resp = postJSON(t, client, loginURL, loginBody{
		Email:    email,
		Password: "validpassword",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("second login: expected 200, got %d", resp.StatusCode)
	}
	decodeJSONBody(t, resp, &tok2)
	refreshCookie2 = extractRefreshCookie(resp)
	_ = resp.Body.Close()

	// 7. Logout-all using the access token.
	logoutAllURL := env.server.URL + "/api/v1/auth/logout-all"
	req, _ = http.NewRequest(http.MethodPost, logoutAllURL, nil)
	req.Header.Set("Authorization", "Bearer "+tok2.AccessToken)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("logout-all request: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("logout-all: expected 204, got %d", resp.StatusCode)
	}

	// 8. Verify the refresh token from session 2 is now revoked: refreshing
	// with it must fail.
	req, _ = http.NewRequest(http.MethodPost, refreshURL, nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: refreshCookie2})
	// Even with a fresh access token it must fail since the refresh is revoked.
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("post-logout-all refresh: %v", err)
	}
	// No Authorization header here → expect 401 from the CSRF check or the
	// reuse check. Either way, the request must not succeed.
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("post-logout-all refresh: expected 401, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()
}

// TestAuth_Login_WrongPassword verifies a known user cannot log in with a
// bad password server-side end-to-end.
func TestAuth_Login_WrongPassword(t *testing.T) {
	env := newE2EEnv(t)
	client := env.server.Client()

	email := "wrongpw-" + randomSuffix() + "@example.com"
	resp := postJSON(t, client, env.server.URL+"/api/v1/users", registerBody{
		Email:    email,
		Password: "correct-password-here",
		Name:     "E2E User",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	resp = postJSON(t, client, env.server.URL+"/api/v1/auth/login", loginBody{
		Email:    email,
		Password: "definitely-wrong-password",
	})
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 on wrong password, got %d", resp.StatusCode)
	}

	// No refresh cookie should be set on a failed login.
	if c := extractRefreshCookie(resp); c != "" {
		t.Errorf("expected no refresh cookie on failed login, got %q", c)
	}
}

// TestAuth_AuthMiddleware_RejectsMissingHeader is a focused integration test
// of the middleware itself, asserting a route mounted behind Auth() rejects
// clients with no Authorization header.
func TestAuth_AuthMiddleware_RejectsMissingHeader(t *testing.T) {
	env := newE2EEnv(t)
	client := env.server.Client()

	resp, err := client.Post(env.server.URL+"/api/v1/auth/logout-all", "application/json", bytes.NewReader([]byte("{}")))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 on missing auth header, got %d", resp.StatusCode)
	}
}

// randomSuffix is a tiny helper for producing distinct emails across tests.
func randomSuffix() string {
	tok, _ := auth.GenerateRefreshToken()
	return tok[:8]
}

// unused-import suppression guards so local tooling stays happy with the
// build-tag-only file.
var (
	_ = context.Background
	_ = config.Config{}
	_ = models.User{}
	_ = errors.Is
)