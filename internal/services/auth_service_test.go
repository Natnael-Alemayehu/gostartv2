package services

import (
	"context"
	"database/sql"
	"errors"
	"gostartv2/internal/auth"
	"gostartv2/internal/models"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// mockAuthUserRepo is a function-field mock implementing the authUserRepo
// interface. GetByEmailFn is nil-by-default; the field is only here for
// tests that need to assert calls.
type mockAuthUserRepo struct {
	getByEmailFn func(ctx context.Context, email string) (*models.User, error)
}

func (m *mockAuthUserRepo) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	if m.getByEmailFn != nil {
		return m.getByEmailFn(ctx, email)
	}

	return nil, sql.ErrNoRows
}

// mockRefreshTokenRepo is a function-field mock implementing the
// refreshTokenRepo interface. The in-memory map is keyed by token hash and
// supports Create, GetByHash, Revoke, RevokeChain, and RevokeAllForUser via
// the nil-default function fields.
type mockRefreshTokenRepo struct {
	tokens map[string]*models.RefreshToken

	createFn         func(ctx context.Context, rt models.RefreshTokenCreate) (*models.RefreshToken, error)
	getByHashFn      func(ctx context.Context, hash string) (*models.RefreshToken, error)
	revokeFn         func(ctx context.Context, id uuid.UUID) error
	revokeChainFn    func(ctx context.Context, chainID uuid.UUID) error
	revokeAllForUser func(ctx context.Context, userID uuid.UUID) error
}

func newMockRefreshTokenRepo() *mockRefreshTokenRepo {
	return &mockRefreshTokenRepo{tokens: make(map[string]*models.RefreshToken)}
}

func (m *mockRefreshTokenRepo) Create(ctx context.Context, rt models.RefreshTokenCreate) (*models.RefreshToken, error) {
	if m.createFn != nil {
		return m.createFn(ctx, rt)
	}

	row := &models.RefreshToken{
		ID:        uuid.New(),
		UserID:    rt.UserID,
		TokenHash: rt.TokenHash,
		ChainID:   rt.ChainID,
		ExpiresAt: rt.ExpiresAt,
	}
	m.tokens[rt.TokenHash] = row

	return row, nil
}

func (m *mockRefreshTokenRepo) GetByHash(ctx context.Context, hash string) (*models.RefreshToken, error) {
	if m.getByHashFn != nil {
		return m.getByHashFn(ctx, hash)
	}

	row, ok := m.tokens[hash]
	if !ok {
		return nil, sql.ErrNoRows
	}

	return row, nil
}

func (m *mockRefreshTokenRepo) Revoke(ctx context.Context, id uuid.UUID) error {
	if m.revokeFn != nil {
		return m.revokeFn(ctx, id)
	}

	now := time.Now()

	for _, t := range m.tokens {
		if t.ID == id {
			t.RevokedAt = &now
			return nil
		}
	}

	return nil
}

func (m *mockRefreshTokenRepo) RevokeChain(ctx context.Context, chainID uuid.UUID) error {
	if m.revokeChainFn != nil {
		return m.revokeChainFn(ctx, chainID)
	}

	now := time.Now()

	for _, t := range m.tokens {
		if t.ChainID == chainID {
			t.RevokedAt = &now
		}
	}

	return nil
}

func (m *mockRefreshTokenRepo) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	if m.revokeAllForUser != nil {
		return m.revokeAllForUser(ctx, userID)
	}

	now := time.Now()

	for _, t := range m.tokens {
		if t.UserID == userID {
			t.RevokedAt = &now
		}
	}

	return nil
}

func newTestAuthService(t *testing.T) (*AuthService, *mockAuthUserRepo, *mockRefreshTokenRepo) {
	t.Helper()

	signer := auth.NewSigner("test-secret", "gostartv2-test", 15*time.Minute)
	users := &mockAuthUserRepo{}
	refresh := newMockRefreshTokenRepo()
	svc := NewAuthService(users, refresh, signer, 7*24*time.Hour)

	return svc, users, refresh
}

// hashPassword is a tiny helper for tests that need a real bcrypt hash so the
// CompareHashAndPassword path can succeed.
func hashPassword(t *testing.T, password string) string {
	t.Helper()

	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	return string(h)
}

func TestAuthService_Login_OK(t *testing.T) {
	svc, users, _ := newTestAuthService(t)
	uid := uuid.New()
	hash := hashPassword(t, "correct-horse-battery")

	users.getByEmailFn = func(ctx context.Context, email string) (*models.User, error) {
		if email != "alice@example.com" {
			t.Errorf("unexpected email %q", email)
		}

		return &models.User{ID: uid, Email: email, PasswordHash: hash}, nil
	}

	access, refresh, err := svc.Login(t.Context(), "alice@example.com", "correct-horse-battery")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	if access == "" {
		t.Error("expected non-empty access token")
	}

	if refresh == "" {
		t.Error("expected non-empty refresh token")
	}

	if len(refresh) != 43 {
		t.Errorf("refresh token length: got %d want 43 (base64url unpadded of 32 bytes)", len(refresh))
	}

	verifier := auth.NewVerifier("test-secret", "gostartv2-test")

	claims, err := verifier.Verify(access)
	if err != nil {
		t.Fatalf("verify access: %v", err)
	}

	if claims.UserID != uid {
		t.Errorf("UserID claim: got %s want %s", claims.UserID, uid)
	}

	if claims.ChainID == uuid.Nil {
		t.Error("ChainID should be set on first login")
	}
}

func TestAuthService_Login_UserNotFound(t *testing.T) {
	svc, _, _ := newTestAuthService(t)

	_, _, err := svc.Login(t.Context(), "nobody@example.com", "any-password")
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	svc, users, _ := newTestAuthService(t)
	hash := hashPassword(t, "correct-password")

	users.getByEmailFn = func(ctx context.Context, email string) (*models.User, error) {
		return &models.User{ID: uuid.New(), Email: email, PasswordHash: hash}, nil
	}

	_, _, err := svc.Login(t.Context(), "alice@example.com", "wrong-password")
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthService_Refresh_OK_RotatesAndRevokes(t *testing.T) {
	svc, users, refresh := newTestAuthService(t)
	uid := uuid.New()
	chainID := uuid.New()

	users.getByEmailFn = func(ctx context.Context, email string) (*models.User, error) {
		return &models.User{ID: uid, Email: email, PasswordHash: hashPassword(t, "pw")}, nil
	}

	_, firstRefresh, err := svc.Login(t.Context(), "alice@example.com", "pw")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	// Login issues its own new chain; we want to test Refresh specifically, so
	// rewrite the issued row to use our chosen chainID for determinism.
	storedRow := refresh.tokens[auth.HashRefreshToken(firstRefresh)]
	storedRow.ChainID = chainID

	newAccess, newRefresh, err := svc.Refresh(t.Context(), firstRefresh)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if newAccess == "" {
		t.Error("expected non-empty new access token")
	}

	if newRefresh == firstRefresh {
		t.Error("rotation should produce a different refresh token")
	}

	// The old token must now be revoked.
	oldRow, err := refresh.GetByHash(t.Context(), auth.HashRefreshToken(firstRefresh))
	if err != nil {
		t.Fatalf("get old refresh: %v", err)
	}

	if oldRow.RevokedAt == nil {
		t.Error("old refresh token should be revoked after rotation")
	}

	// New row must not be revoked and must preserve the chain id.
	newRow, err := refresh.GetByHash(t.Context(), auth.HashRefreshToken(newRefresh))
	if err != nil {
		t.Fatalf("get new refresh: %v", err)
	}

	if newRow.RevokedAt != nil {
		t.Error("new refresh token should be active")
	}

	if newRow.ChainID != chainID {
		t.Errorf("ChainID should be preserved, got %s want %s", newRow.ChainID, chainID)
	}

	// Claims from the new access token must retain the chain id.
	v := auth.NewVerifier("test-secret", "gostartv2-test")

	claims, err := v.Verify(newAccess)
	if err != nil {
		t.Fatalf("verify new access: %v", err)
	}

	if claims.ChainID != chainID {
		t.Errorf("claim ChainID: got %s want %s", claims.ChainID, chainID)
	}

	if claims.UserID != uid {
		t.Errorf("claim UserID: got %s want %s", claims.UserID, uid)
	}
}

func TestAuthService_Refresh_UnknownToken(t *testing.T) {
	svc, _, _ := newTestAuthService(t)

	_, _, err := svc.Refresh(t.Context(), "fake-token")
	if !errors.Is(err, auth.ErrRefreshTokenRevoked) {
		t.Errorf("expected ErrRefreshTokenRevoked, got %v", err)
	}
}

func TestAuthService_Refresh_Reuse_TriggersChainRevoke(t *testing.T) {
	svc, users, refresh := newTestAuthService(t)
	chainID := uuid.New()

	users.getByEmailFn = func(ctx context.Context, email string) (*models.User, error) {
		return &models.User{ID: uuid.New(), Email: email, PasswordHash: hashPassword(t, "pw")}, nil
	}

	// Plant an already-revoked token with the chosen chain, plus a second
	// active token in the same chain (the "stolen" rotated token).
	now := time.Now()
	hashOld := auth.HashRefreshToken("old-plaintext")
	hashNew := auth.HashRefreshToken("new-plaintext")
	refresh.tokens[hashOld] = &models.RefreshToken{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		TokenHash: hashOld,
		ChainID:   chainID,
		RevokedAt: &now,
		ExpiresAt: time.Now().Add(time.Hour),
	}
	refresh.tokens[hashNew] = &models.RefreshToken{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		TokenHash: hashNew,
		ChainID:   chainID,
		ExpiresAt: time.Now().Add(time.Hour),
	}

	// Present the revoked token — reuse detection should revoke the new one too.
	_, _, err := svc.Refresh(t.Context(), "old-plaintext")
	if !errors.Is(err, auth.ErrRefreshTokenReuse) {
		t.Fatalf("expected ErrRefreshTokenReuse, got %v", err)
	}

	newRow, err := refresh.GetByHash(t.Context(), hashNew)
	if err != nil {
		t.Fatalf("get new token after reuse: %v", err)
	}

	if newRow.RevokedAt == nil {
		t.Error("active token in same chain should be revoked on reuse")
	}
}

func TestAuthService_Refresh_Expired(t *testing.T) {
	svc, users, refresh := newTestAuthService(t)
	plaintext := "expired-plaintext"
	refresh.tokens[auth.HashRefreshToken(plaintext)] = &models.RefreshToken{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		TokenHash: auth.HashRefreshToken(plaintext),
		ChainID:   uuid.New(),
		ExpiresAt: time.Now().Add(-time.Minute), // already expired
	}
	// Login helper unused here; suppress the unused-variable lint by wiring a
	// no-op GetByEmail.
	users.getByEmailFn = func(ctx context.Context, email string) (*models.User, error) {
		return nil, sql.ErrNoRows
	}

	_, _, err := svc.Refresh(t.Context(), plaintext)
	if !errors.Is(err, auth.ErrRefreshTokenExpired) {
		t.Errorf("expected ErrRefreshTokenExpired, got %v", err)
	}
}

func TestAuthService_Logout_OK(t *testing.T) {
	svc, users, refresh := newTestAuthService(t)
	users.getByEmailFn = func(ctx context.Context, email string) (*models.User, error) {
		return &models.User{ID: uuid.New(), Email: email, PasswordHash: hashPassword(t, "pw")}, nil
	}

	_, refreshToken, err := svc.Login(t.Context(), "alice@example.com", "pw")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	if err := svc.Logout(t.Context(), refreshToken); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	row, err := refresh.GetByHash(t.Context(), auth.HashRefreshToken(refreshToken))
	if err != nil {
		t.Fatalf("get after logout: %v", err)
	}

	if row.RevokedAt == nil {
		t.Error("token should be revoked after logout")
	}
}

func TestAuthService_Logout_UnknownToken_NoOp(t *testing.T) {
	svc, _, _ := newTestAuthService(t)
	if err := svc.Logout(t.Context(), "never-issued"); err != nil {
		t.Errorf("Logout on unknown token should be nil, got %v", err)
	}
}

func TestAuthService_LogoutAll_OK(t *testing.T) {
	svc, users, refresh := newTestAuthService(t)
	uid := uuid.New()
	users.getByEmailFn = func(ctx context.Context, email string) (*models.User, error) {
		return &models.User{ID: uid, Email: email, PasswordHash: hashPassword(t, "pw")}, nil
	}

	_, r1, _ := svc.Login(t.Context(), "alice@example.com", "pw")
	// Plant a second active token under the same user with a different chain
	// to simulate a separate session.
	hash2 := auth.HashRefreshToken("second-plaintext")
	refresh.tokens[hash2] = &models.RefreshToken{
		ID:        uuid.New(),
		UserID:    uid,
		TokenHash: hash2,
		ChainID:   uuid.New(),
		ExpiresAt: time.Now().Add(time.Hour),
	}

	if err := svc.LogoutAll(t.Context(), uid); err != nil {
		t.Fatalf("LogoutAll: %v", err)
	}

	row1, err := refresh.GetByHash(t.Context(), auth.HashRefreshToken(r1))
	if err != nil {
		t.Fatalf("get r1: %v", err)
	}

	if row1.RevokedAt == nil {
		t.Error("r1 should be revoked after LogoutAll")
	}

	row2, err := refresh.GetByHash(t.Context(), hash2)
	if err != nil {
		t.Fatalf("get row2: %v", err)
	}

	if row2.RevokedAt == nil {
		t.Error("row2 should be revoked after LogoutAll")
	}
}

// Compile-time interface satisfaction checks.
var (
	_ authUserRepo     = (*mockAuthUserRepo)(nil)
	_ refreshTokenRepo = (*mockRefreshTokenRepo)(nil)
)
