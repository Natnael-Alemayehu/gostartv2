package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"gostartv2/internal/auth"
	"gostartv2/internal/middleware"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// mockAuthService is a function-field mock implementing the authService
// interface. Default returns are sensible so the mock is usable out of the
// box with &mockAuthService{}.
type mockAuthService struct {
	loginFn     func(ctx context.Context, email, password string) (string, string, error)
	refreshFn   func(ctx context.Context, refresh string) (string, string, error)
	logoutFn    func(ctx context.Context, refresh string) error
	logoutAllFn func(ctx context.Context, userID uuid.UUID) error
}

func (m *mockAuthService) Login(ctx context.Context, email, password string) (string, string, error) {
	if m.loginFn != nil {
		return m.loginFn(ctx, email, password)
	}

	return "access-token", "refresh-token", nil
}

func (m *mockAuthService) Refresh(ctx context.Context, refresh string) (string, string, error) {
	if m.refreshFn != nil {
		return m.refreshFn(ctx, refresh)
	}

	return "new-access", "new-refresh", nil
}

func (m *mockAuthService) Logout(ctx context.Context, refresh string) error {
	if m.logoutFn != nil {
		return m.logoutFn(ctx, refresh)
	}

	return nil
}

func (m *mockAuthService) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	if m.logoutAllFn != nil {
		return m.logoutAllFn(ctx, userID)
	}

	return nil
}

func newTestAuthHandler(t *testing.T, svc authService, isProd bool) *AuthHandler {
	t.Helper()

	verifier := auth.NewVerifier("test-secret", "gostartv2-test")

	return NewAuthHandler(svc, verifier, 15*time.Minute, 7*24*time.Hour, isProd)
}

// newAuthTestRouter mounts the auth routes the way the live server will.
// logoutAll is wrapped in the Auth middleware so tests can assert the
// context-injection path.
func newAuthTestRouter(h *AuthHandler, verifier middleware.Verifier) http.Handler {
	r := chi.NewRouter()
	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Post("/login", h.Login)
		r.Post("/refresh", h.Refresh)
		r.Post("/logout", h.Logout)
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(verifier))
			r.Post("/logout-all", h.LogoutAll)
		})
	})

	return r
}

func doAuthRequest(
	t *testing.T,
	router http.Handler,
	method, target string,
	body any,
	cookie *http.Cookie,
	bearer string,
) *http.Response {
	t.Helper()

	var reqBody io.Reader

	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}

		reqBody = bytes.NewReader(b)
	}

	req := httptest.NewRequest(method, target, reqBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if cookie != nil {
		req.AddCookie(cookie)
	}

	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	return w.Result()
}

// freshAuthCookie builds a refresh_token cookie for use in tests. It is a
// bare-minimum fixture; production cookie attributes live on the handler.
//
//nolint:gosec // G124: test fixture, no browser round-trip
func freshAuthCookie(value string) *http.Cookie {
	return &http.Cookie{Name: refreshTokenCookie, Value: value}
}

// requireRefreshCookie extracts the refresh_token cookie from resp and asserts
// the prod/HttpOnly/SameSite/Path/Value attributes match the handler's
// cookie policy. expectSecure is true when the test constructs the handler
// with isProd=true.
func requireRefreshCookie(t *testing.T, resp *http.Response, wantValue string, expectSecure bool) {
	t.Helper()

	var rc *http.Cookie

	for _, c := range resp.Cookies() {
		if c.Name == refreshTokenCookie {
			rc = c
		}
	}

	if rc == nil {
		t.Fatal("expected refresh_token cookie to be set")
	}

	if !rc.HttpOnly {
		t.Error("refresh cookie must be HttpOnly")
	}

	if expectSecure && !rc.Secure {
		t.Error("refresh cookie must be Secure in prod")
	}

	if rc.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite: got %v want %v", rc.SameSite, http.SameSiteLaxMode)
	}

	if rc.Path != "/api/v1/auth" {
		t.Errorf("Path: got %q want /api/v1/auth", rc.Path)
	}

	if wantValue != "" && rc.Value != wantValue {
		t.Errorf("cookie value: got %q want %q", rc.Value, wantValue)
	}
}

func TestAuthHandler_Login_OK(t *testing.T) {
	svc := &mockAuthService{
		loginFn: func(ctx context.Context, email, password string) (string, string, error) {
			if email != "alice@example.com" || password != "supersecret" {
				t.Errorf("unexpected creds: %s / %s", email, password)
			}

			return "access-token", "raw-refresh", nil
		},
	}
	h := newTestAuthHandler(t, svc, true)
	router := newAuthTestRouter(h, nil)

	resp := doAuthRequest(t, router, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"email": "alice@example.com", "password": "supersecret"},
		nil, "")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Refresh cookie must be set, HttpOnly, Secure in prod, SameSite=Lax, and
	// scoped to the /api/v1/auth path.
	requireRefreshCookie(t, resp, "raw-refresh", true)

	body := decodeAuthBody(t, resp)
	if body["access_token"] != "access-token" {
		t.Errorf("access_token: got %v want %q", body["access_token"], "access-token")
	}

	if body["token_type"] != "Bearer" {
		t.Errorf("token_type: got %v want Bearer", body["token_type"])
	}

	if body["expires_in"] == nil {
		t.Error("expires_in missing")
	}
}

func TestAuthHandler_Login_InvalidCredentials(t *testing.T) {
	svc := &mockAuthService{
		loginFn: func(ctx context.Context, email, password string) (string, string, error) {
			return "", "", auth.ErrInvalidCredentials
		},
	}
	h := newTestAuthHandler(t, svc, false)
	router := newAuthTestRouter(h, nil)

	// Password must pass validation (min 8) so it reaches the service.
	resp := doAuthRequest(t, router, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"email": "x@example.com", "password": "validlenpw"}, nil, "")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}

	body := decodeAuthBody(t, resp)

	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatal("missing error envelope")
	}

	if errObj["code"] != "invalid_credentials" {
		t.Errorf("code: got %v want invalid_credentials", errObj["code"])
	}

	// No cookie should be set on a failed login.
	for _, c := range resp.Cookies() {
		if c.Name == refreshTokenCookie {
			t.Error("refresh cookie should not be set on failed login")
		}
	}
}

func TestAuthHandler_Login_ValidationError(t *testing.T) {
	h := newTestAuthHandler(t, &mockAuthService{}, false)
	router := newAuthTestRouter(h, nil)

	// Empty body — both email and password fail validation.
	resp := doAuthRequest(t, router, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"email": "", "password": ""}, nil, "")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAuthHandler_Login_InvalidJSON(t *testing.T) {
	h := newTestAuthHandler(t, &mockAuthService{}, false)
	router := newAuthTestRouter(h, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		strings.NewReader("{not json"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Result().StatusCode)
	}
}

func TestAuthHandler_Refresh_OK(t *testing.T) {
	svc := &mockAuthService{
		refreshFn: func(ctx context.Context, refresh string) (string, string, error) {
			if refresh != "raw-refresh-from-cookie" {
				t.Errorf("refresh arg: got %q want raw-refresh-from-cookie", refresh)
			}

			return "new-access", "new-refresh", nil
		},
	}
	h := newTestAuthHandler(t, svc, true)
	router := newAuthTestRouter(h, nil)

	resp := doAuthRequest(t, router, http.MethodPost, "/api/v1/auth/refresh",
		nil, freshAuthCookie("raw-refresh-from-cookie"), "stale-access-token")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var newCookie *http.Cookie

	for _, c := range resp.Cookies() {
		if c.Name == refreshTokenCookie {
			newCookie = c
		}
	}

	if newCookie == nil || newCookie.Value != "new-refresh" {
		t.Errorf("expected new refresh cookie 'new-refresh', got %v", newCookie)
	}

	body := decodeAuthBody(t, resp)
	if body["access_token"] != "new-access" {
		t.Errorf("access_token: got %v want new-access", body["access_token"])
	}
}

func TestAuthHandler_Refresh_NoCookie(t *testing.T) {
	h := newTestAuthHandler(t, &mockAuthService{}, false)
	router := newAuthTestRouter(h, nil)

	resp := doAuthRequest(t, router, http.MethodPost, "/api/v1/auth/refresh",
		nil, nil, "stale-access")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuthHandler_Refresh_NoBearer(t *testing.T) {
	h := newTestAuthHandler(t, &mockAuthService{}, false)
	router := newAuthTestRouter(h, nil)

	resp := doAuthRequest(t, router, http.MethodPost, "/api/v1/auth/refresh",
		nil, freshAuthCookie("raw-refresh"), "")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 (CSRF defense), got %d", resp.StatusCode)
	}

	body := decodeAuthBody(t, resp)

	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "missing_token" {
		t.Errorf("code: got %v want missing_token", errObj["code"])
	}
	// svc.Logout must NOT have been invoked — the CSRF check fails before it.
}

func TestAuthHandler_Refresh_Reuse(t *testing.T) {
	svc := &mockAuthService{
		refreshFn: func(ctx context.Context, refresh string) (string, string, error) {
			return "", "", auth.ErrRefreshTokenReuse
		},
	}
	h := newTestAuthHandler(t, svc, true)
	router := newAuthTestRouter(h, nil)

	resp := doAuthRequest(t, router, http.MethodPost, "/api/v1/auth/refresh",
		nil, freshAuthCookie("revoked-token"), "access")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}

	body := decodeAuthBody(t, resp)

	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "session_revoked" {
		t.Errorf("code: got %v want session_revoked", errObj["code"])
	}
}

func TestAuthHandler_Logout_OK(t *testing.T) {
	loggedOut := false
	svc := &mockAuthService{
		logoutFn: func(ctx context.Context, refresh string) error {
			if refresh != "to-be-cleared" {
				t.Errorf("refresh arg: got %q want to-be-cleared", refresh)
			}

			loggedOut = true

			return nil
		},
	}
	h := newTestAuthHandler(t, svc, true)
	router := newAuthTestRouter(h, nil)

	resp := doAuthRequest(t, router, http.MethodPost, "/api/v1/auth/logout",
		nil, freshAuthCookie("to-be-cleared"), "")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	if !loggedOut {
		t.Error("svc.Logout was not invoked")
	}

	// Cookie must be cleared.
	var cleared *http.Cookie

	for _, c := range resp.Cookies() {
		if c.Name == refreshTokenCookie {
			cleared = c
		}
	}

	if cleared == nil {
		t.Fatal("expected cleared refresh cookie")
	}

	if cleared.MaxAge != -1 {
		t.Errorf("cleared cookie MaxAge: got %d want -1", cleared.MaxAge)
	}
}

func TestAuthHandler_Logout_NoCookie_NoOp(t *testing.T) {
	svc := &mockAuthService{
		logoutFn: func(ctx context.Context, refresh string) error {
			t.Error("Logout should not be called when there is no cookie")
			return nil
		},
	}
	h := newTestAuthHandler(t, svc, false)
	router := newAuthTestRouter(h, nil)

	resp := doAuthRequest(t, router, http.MethodPost, "/api/v1/auth/logout",
		nil, nil, "")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 even without cookie, got %d", resp.StatusCode)
	}
}

func TestAuthHandler_LogoutAll_OK(t *testing.T) {
	uid := uuid.New()
	svc := &mockAuthService{
		logoutAllFn: func(ctx context.Context, userID uuid.UUID) error {
			if userID != uid {
				t.Errorf("userID: got %s want %s", userID, uid)
			}

			return nil
		},
	}
	h := newTestAuthHandler(t, svc, true)

	// Sign a real access token for the test user so the middleware lets the
	// request through.
	signer := auth.NewSigner("test-secret", "gostartv2-test", 15*time.Minute)
	verifier := auth.NewVerifier("test-secret", "gostartv2-test")

	access, err := signer.Sign(uid, uuid.New())
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	router := newAuthTestRouter(h, verifier)

	resp := doAuthRequest(t, router, http.MethodPost, "/api/v1/auth/logout-all",
		nil, nil, access)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestAuthHandler_LogoutAll_NoToken_Unauthorized(t *testing.T) {
	h := newTestAuthHandler(t, &mockAuthService{}, true)
	verifier := auth.NewVerifier("test-secret", "gostartv2-test")
	router := newAuthTestRouter(h, verifier)

	resp := doAuthRequest(t, router, http.MethodPost, "/api/v1/auth/logout-all",
		nil, nil, "")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// decodeAuthBody is the auth-handlers analogue of decodeBody in
// user_handler_test.go.
func decodeAuthBody(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	var out map[string]any
	if len(b) == 0 {
		return out
	}

	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("decode body %q: %v", string(b), err)
	}

	return out
}
