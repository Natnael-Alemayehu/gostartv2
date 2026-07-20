//go:build integration

package repositories

import (
	"context"
	"database/sql"
	"errors"
	"gostartv2/internal/auth"
	"gostartv2/internal/models"
	"testing"
	"time"

	"github.com/google/uuid"
)

func newRefreshTokenRepo(t *testing.T) *RefreshTokenRepository {
	t.Helper()
	return newRepo(t).RefreshTokens
}

// createTestUser inserts a user via the Users repo so the FK on
// refresh_tokens.user_id is satisfied. Returns the new user id.
func createTestUser(t *testing.T, ctx context.Context, repos *Repositories, email string) uuid.UUID {
	t.Helper()
	u, err := repos.Users.Create(ctx, models.UserCreate{
		Email:        email,
		PasswordHash: "$2a$10$dummyhash",
		Name:         "Test User",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u.ID
}

func TestRefreshTokenRepository_CreateAndGetByHash(t *testing.T) {
	repos := newRepo(t)
	r := repos.RefreshTokens

	ctx := t.Context()
	userID := createTestUser(t, ctx, repos, "rt-create@example.com")
	chainID := uuid.New()
	hash := sha256Hex("plaintext-token-1")

	rt, err := r.Create(ctx, models.RefreshTokenCreate{
		UserID:    userID,
		TokenHash: hash,
		ChainID:   chainID,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rt.ID == uuid.Nil {
		t.Fatal("expected non-nil ID")
	}
	if rt.UserID != userID {
		t.Errorf("UserID: got %s want %s", rt.UserID, userID)
	}
	if rt.TokenHash != hash {
		t.Errorf("TokenHash: got %s want %s", rt.TokenHash, hash)
	}
	if rt.ChainID != chainID {
		t.Errorf("ChainID: got %s want %s", rt.ChainID, chainID)
	}
	if rt.RevokedAt != nil {
		t.Errorf("RevokedAt should be nil on insert, got %v", *rt.RevokedAt)
	}

	got, err := r.GetByHash(ctx, hash)
	if err != nil {
		t.Fatalf("GetByHash: %v", err)
	}
	if got.ID != rt.ID {
		t.Errorf("GetByHash returned wrong row: got %s want %s", got.ID, rt.ID)
	}
}

func TestRefreshTokenRepository_GetByHash_NotFound(t *testing.T) {
	r := newRefreshTokenRepo(t)
	ctx := t.Context()

	_, err := r.GetByHash(ctx, sha256Hex("nonexistent"))
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestRefreshTokenRepository_Revoke_Idempotent(t *testing.T) {
	repos := newRepo(t)
	r := repos.RefreshTokens

	ctx := t.Context()
	userID := createTestUser(t, ctx, repos, "rt-revoke@example.com")

	rt, err := r.Create(ctx, models.RefreshTokenCreate{
		UserID:    userID,
		TokenHash: sha256Hex("plaintext-token-revoke"),
		ChainID:   uuid.New(),
		ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := r.Revoke(ctx, rt.ID); err != nil {
		t.Fatalf("first Revoke: %v", err)
	}

	row, err := r.GetByHash(ctx, rt.TokenHash)
	if err != nil {
		t.Fatalf("GetByHash after Revoke: %v", err)
	}
	if row.RevokedAt == nil {
		t.Fatal("RevokedAt should be set after Revoke")
	}

	// Second revoke is a no-op.
	if err := r.Revoke(ctx, rt.ID); err != nil {
		t.Fatalf("second Revoke should be no-op, got %v", err)
	}

	// Revoke on a missing id also a no-op.
	missingID := uuid.New()
	if err := r.Revoke(ctx, missingID); err != nil {
		t.Fatalf("Revoke on missing id should be no-op, got %v", err)
	}
}

func TestRefreshTokenRepository_RevokeChain(t *testing.T) {
	repos := newRepo(t)
	r := repos.RefreshTokens

	ctx := t.Context()
	userID := createTestUser(t, ctx, repos, "rt-chain@example.com")
	chainID := uuid.New()

	// Three tokens, same chain.
	hashes := []string{
		sha256Hex("chain-1"),
		sha256Hex("chain-2"),
		sha256Hex("chain-3"),
	}
	for _, h := range hashes {
		if _, err := r.Create(ctx, models.RefreshTokenCreate{
			UserID:    userID,
			TokenHash: h,
			ChainID:   chainID,
			ExpiresAt: time.Now().Add(time.Hour),
		}); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	// Different chain on same user must NOT be revoked.
	otherChainHash := sha256Hex("other-chain")
	if _, err := r.Create(ctx, models.RefreshTokenCreate{
		UserID:    userID,
		TokenHash: otherChainHash,
		ChainID:   uuid.New(),
		ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("Create other chain: %v", err)
	}

	if err := r.RevokeChain(ctx, chainID); err != nil {
		t.Fatalf("RevokeChain: %v", err)
	}

	for _, h := range hashes {
		row, err := r.GetByHash(ctx, h)
		if err != nil {
			t.Fatalf("GetByHash(%s): %v", h, err)
		}
		if row.RevokedAt == nil {
			t.Errorf("token %s should be revoked", h)
		}
	}

	other, err := r.GetByHash(ctx, otherChainHash)
	if err != nil {
		t.Fatalf("GetByHash(other chain): %v", err)
	}
	if other.RevokedAt != nil {
		t.Fatal("other-chain token was wrongly revoked by RevokeChain")
	}
}

func TestRefreshTokenRepository_RevokeAllForUser(t *testing.T) {
	repos := newRepo(t)
	r := repos.RefreshTokens

	ctx := t.Context()
	userA := createTestUser(t, ctx, repos, "rt-userA@example.com")
	userB := createTestUser(t, ctx, repos, "rt-userB@example.com")

	for i := range 2 {
		if _, err := r.Create(ctx, models.RefreshTokenCreate{
			UserID:    userA,
			TokenHash: sha256Hex("userA-" + string(rune('a'+i))),
			ChainID:   uuid.New(),
			ExpiresAt: time.Now().Add(time.Hour),
		}); err != nil {
			t.Fatalf("Create userA token: %v", err)
		}
	}
	if _, err := r.Create(ctx, models.RefreshTokenCreate{
		UserID:    userB,
		TokenHash: sha256Hex("userB-token"),
		ChainID:   uuid.New(),
		ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("Create userB token: %v", err)
	}

	if err := r.RevokeAllForUser(ctx, userA); err != nil {
		t.Fatalf("RevokeAllForUser: %v", err)
	}

	row, err := r.GetByHash(ctx, sha256Hex("userB-token"))
	if err != nil {
		t.Fatalf("GetByHash userB: %v", err)
	}
	if row.RevokedAt != nil {
		t.Fatal("userB token was wrongly revoked by RevokeAllForUser")
	}
	for _, h := range []string{sha256Hex("userA-a"), sha256Hex("userA-b")} {
		row, err := r.GetByHash(ctx, h)
		if err != nil {
			t.Fatalf("GetByHash(%s): %v", h, err)
		}
		if row.RevokedAt == nil {
			t.Errorf("token %s should be revoked", h)
		}
	}
}

func TestRefreshTokenRepository_Create_DuplicateHash(t *testing.T) {
	repos := newRepo(t)
	r := repos.RefreshTokens

	ctx := t.Context()
	userID := createTestUser(t, ctx, repos, "rt-dup@example.com")
	hash := sha256Hex("dup-token")

	if _, err := r.Create(ctx, models.RefreshTokenCreate{
		UserID:    userID,
		TokenHash: hash,
		ChainID:   uuid.New(),
		ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("first Create: %v", err)
	}

	_, err := r.Create(ctx, models.RefreshTokenCreate{
		UserID:    userID,
		TokenHash: hash, // same hash
		ChainID:   uuid.New(),
		ExpiresAt: time.Now().Add(time.Hour),
	})
	if err == nil {
		t.Fatal("expected duplicate-hash error, got nil")
	}
}

// sha256Hex is a small test helper that delegates to auth.HashRefreshToken so
// the hash format stays consistent between production code and tests.
func sha256Hex(s string) string {
	return auth.HashRefreshToken(s)
}
