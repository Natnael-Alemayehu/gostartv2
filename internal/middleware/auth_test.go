package middleware

import (
	"context"
	"gostartv2/internal/auth"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

// fakeVerifier is a hand-written Verifier for middleware tests; it returns
// a fixed (claims, error) pair set by the test.
type fakeVerifier struct {
	claims *auth.Claims
	err    error
}

func (f *fakeVerifier) Verify(tokenStr string) (*auth.Claims, error) {
	return f.claims, f.err
}

func TestAuth_NoHeader_401(t *testing.T) {
	v := &fakeVerifier{err: auth.ErrTokenInvalid}
	h := Auth(v)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not run on missing header")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Result().StatusCode)
	}
}

func TestAuth_MalformedHeader_401(t *testing.T) {
	v := &fakeVerifier{err: auth.ErrTokenInvalid}
	h := Auth(v)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not run on malformed header")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "NotBearer token")

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Result().StatusCode)
	}
}

func TestAuth_ExpiredToken_401(t *testing.T) {
	v := &fakeVerifier{err: auth.ErrTokenExpired}
	h := Auth(v)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not run on expired token")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer expired-jwt")

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Result().StatusCode)
	}
}

func TestAuth_InvalidToken_401(t *testing.T) {
	v := &fakeVerifier{err: auth.ErrTokenInvalid}
	h := Auth(v)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not run on invalid token")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer tampered-jwt")

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Result().StatusCode)
	}
}

func TestAuth_OK_InjectsUserIntoContext(t *testing.T) {
	uid := uuid.New()
	cid := uuid.New()
	v := &fakeVerifier{claims: &auth.Claims{UserID: uid, ChainID: cid}}

	var (
		gotUID, gotCID uuid.UUID
		ok             bool
	)

	h := Auth(v)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUID, gotCID, ok = UserFromContext(r.Context())

		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer valid-jwt")

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Result().StatusCode)
	}

	if !ok {
		t.Fatal("UserFromContext returned ok=false after middleware ran")
	}

	if gotUID != uid {
		t.Errorf("userID: got %s want %s", gotUID, uid)
	}

	if gotCID != cid {
		t.Errorf("chainID: got %s want %s", gotCID, cid)
	}
}

func TestUserFromContext_EmptyContext(t *testing.T) {
	if _, _, ok := UserFromContext(context.Background()); ok {
		t.Fatal("UserFromContext should return ok=false on empty context")
	}
}

// Confirm *auth.Verifier satisfies the Verifier interface at compile time
// from within the test as well.
func TestVerifierInterfaceSatisfied(t *testing.T) {
	var (
		_ Verifier = (*auth.Verifier)(nil)
		_ Verifier = &fakeVerifier{}
	)
}
