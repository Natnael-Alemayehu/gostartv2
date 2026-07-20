package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"gostartv2/internal/auth"
	"gostartv2/internal/models"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// authUserRepo is the consumer-side subset of UserRepository needed by
// AuthService. Defined here in the consumer package so the service can be
// unit-tested with a fake repo and so the repository package stays unaware of
// the auth flow.
type authUserRepo interface {
	GetByEmail(ctx context.Context, email string) (*models.User, error)
}

// refreshTokenRepo is the consumer-side contract for the refresh-token
// repository. Methods that return sql.ErrNoRows (GetByHash) are translated by
// the service into auth-layer sentinels.
type refreshTokenRepo interface {
	Create(ctx context.Context, rt models.RefreshTokenCreate) (*models.RefreshToken, error)
	GetByHash(ctx context.Context, hash string) (*models.RefreshToken, error)
	Revoke(ctx context.Context, id uuid.UUID) error
	RevokeChain(ctx context.Context, chainID uuid.UUID) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
}

// AuthService implements the authentication flow: credential verification,
// JWT issuance, and rotating refresh-token management with reuse detection.
// It has no HTTP awareness.
type AuthService struct {
	users         authUserRepo
	refreshTokens refreshTokenRepo
	signer        *auth.Signer
	refreshTTL    time.Duration
}

// NewAuthService returns an AuthService backed by the given user and
// refresh-token repositories, signing access tokens with signer and issuing
// refresh tokens with the given refreshTTL lifetime.
func NewAuthService(
	users authUserRepo,
	refreshTokens refreshTokenRepo,
	signer *auth.Signer,
	refreshTTL time.Duration,
) *AuthService {
	return &AuthService{
		users:         users,
		refreshTokens: refreshTokens,
		signer:        signer,
		refreshTTL:    refreshTTL,
	}
}

// Login verifies the email/password pair and, on success, issues an access
// token and the first refresh token of a new session chain. The returned
// refresh token is the plaintext value the handler must place in an HttpOnly
// cookie; only its hash is persisted. ErrInvalidCredentials is returned for
// both "no such user" and "wrong password" to prevent username enumeration.
//
// security: known timing side channel — the no-user path is faster than the
// wrong-password path because bcrypt.CompareHashAndPassword only runs when
// the user exists. Mitigation belongs in a rate-limiting layer, not here.
func (s *AuthService) Login(ctx context.Context, email, password string) (access, refresh string, err error) {
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", auth.ErrInvalidCredentials
		}

		return "", "", fmt.Errorf("get user by email: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", "", auth.ErrInvalidCredentials
	}

	chainID := uuid.New()

	access, refresh, err = s.issueRefreshAndAccess(ctx, user.ID, chainID)
	if err != nil {
		return "", "", err
	}

	return access, refresh, nil
}

// Refresh validates the presented refresh token, rotates it (revoking the old
// row, issuing a new one in the same chain), and returns a new access token
// plus a new refresh token. Reuse of an already-revoked token triggers
// revocation of the entire chain and returns ErrRefreshTokenReuse.
func (s *AuthService) Refresh(ctx context.Context, presentedRefresh string) (newAccess, newRefresh string, err error) {
	hash := auth.HashRefreshToken(presentedRefresh)

	row, err := s.refreshTokens.GetByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", auth.ErrRefreshTokenRevoked
		}

		return "", "", fmt.Errorf("get refresh token: %w", err)
	}

	if row.RevokedAt != nil {
		// Reuse of a revoked refresh token — strongest signal of token theft.
		// Revoke the entire chain to invalidate any tokens issued after the
		// theft, then surface a distinct sentinel so the handler can force
		// re-auth on the client.
		if rerr := s.refreshTokens.RevokeChain(ctx, row.ChainID); rerr != nil {
			return "", "", fmt.Errorf("revoke chain on reuse: %w", rerr)
		}

		return "", "", auth.ErrRefreshTokenReuse
	}

	if time.Now().After(row.ExpiresAt) {
		return "", "", auth.ErrRefreshTokenExpired
	}

	if err := s.refreshTokens.Revoke(ctx, row.ID); err != nil {
		return "", "", fmt.Errorf("revoke old refresh token: %w", err)
	}

	// Preserve the chain id so the lifetime of the session stays traceable
	// across rotations.
	newAccess, newRefresh, err = s.issueRefreshAndAccess(ctx, row.UserID, row.ChainID)
	if err != nil {
		return "", "", err
	}

	return newAccess, newRefresh, nil
}

// Logout revokes the refresh token identified by the presented plaintext
// value. It is idempotent: an unknown or already-revoked token returns nil.
func (s *AuthService) Logout(ctx context.Context, presentedRefresh string) error {
	hash := auth.HashRefreshToken(presentedRefresh)

	row, err := s.refreshTokens.GetByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}

		return fmt.Errorf("get refresh token: %w", err)
	}

	if err := s.refreshTokens.Revoke(ctx, row.ID); err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}

	return nil
}

// LogoutAll revokes every active refresh token owned by userID. Used as the
// server-side implementation of "revoke all sessions".
func (s *AuthService) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	if err := s.refreshTokens.RevokeAllForUser(ctx, userID); err != nil {
		return fmt.Errorf("revoke all for user: %w", err)
	}

	return nil
}

// issueRefreshAndAccess generates a new refresh token, persists its hash,
// and signs an access token for the same user. Returns the plaintext refresh
// token (for the cookie) and the signed access JWT. On error the access token
// is empty.
func (s *AuthService) issueRefreshAndAccess(ctx context.Context, userID, chainID uuid.UUID) (string, string, error) {
	plaintext, err := auth.GenerateRefreshToken()
	if err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}

	if _, err := s.refreshTokens.Create(ctx, models.RefreshTokenCreate{
		UserID:    userID,
		TokenHash: auth.HashRefreshToken(plaintext),
		ChainID:   chainID,
		ExpiresAt: time.Now().Add(s.refreshTTL),
	}); err != nil {
		return "", "", fmt.Errorf("create refresh token: %w", err)
	}

	access, err := s.signer.Sign(userID, chainID)
	if err != nil {
		return "", "", fmt.Errorf("sign access token: %w", err)
	}

	return access, plaintext, nil
}
