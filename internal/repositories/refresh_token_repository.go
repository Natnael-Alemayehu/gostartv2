package repositories

import (
	"context"
	"database/sql"
	"errors"
	"gostartv2/internal/db/sqlc"
	"gostartv2/internal/models"

	"github.com/google/uuid"
)

// RefreshTokenRepository persists and retrieves refresh-token rows via
// sqlc-generated queries. It performs no business logic — rotation,
// reuse-detection, and revocation decisions live in the service layer.
type RefreshTokenRepository struct {
	q *sqlc.Queries
}

// NewRefreshTokenRepository returns a RefreshTokenRepository that issues
// queries through the given sqlc.Queries instance, which may wrap either a
// *sql.DB or a transaction.
func NewRefreshTokenRepository(q *sqlc.Queries) *RefreshTokenRepository {
	return &RefreshTokenRepository{q: q}
}

// Create inserts a refresh token. Callers must populate TokenHash with the
// SHA-256 digest of the plaintext token, never the plaintext itself.
func (r *RefreshTokenRepository) Create(ctx context.Context, rt models.RefreshTokenCreate) (*models.RefreshToken, error) {
	row, err := r.q.CreateRefreshToken(ctx, sqlc.CreateRefreshTokenParams{
		UserID:    rt.UserID,
		TokenHash: rt.TokenHash,
		ChainID:   rt.ChainID,
		ExpiresAt: rt.ExpiresAt,
	})
	if err != nil {
		return nil, err
	}

	return refreshTokenToModel(row), nil
}

// GetByHash fetches the row whose token_hash matches the given SHA-256
// digest. A sql.ErrNoRows error is returned to the caller untranslated when
// no row matches; the caller decides whether that means "unknown token" or
// "revoked" based on surrounding state.
func (r *RefreshTokenRepository) GetByHash(ctx context.Context, hash string) (*models.RefreshToken, error) {
	row, err := r.q.GetRefreshTokenByHash(ctx, hash)
	if err != nil {
		return nil, err
	}

	return refreshTokenToModel(row), nil
}

// Revoke marks the token with the given id as revoked by setting its
// revoked_at timestamp. It is idempotent: revoking an already-revoked or
// missing row returns nil.
func (r *RefreshTokenRepository) Revoke(ctx context.Context, id uuid.UUID) error {
	_, err := r.q.RevokeRefreshToken(ctx, id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	return nil
}

// RevokeChain marks every active token sharing the given chain_id as
// revoked. Used during refresh-token reuse detection to ensure the entire
// session chain is invalidated at once.
func (r *RefreshTokenRepository) RevokeChain(ctx context.Context, chainID uuid.UUID) error {
	return r.q.RevokeChain(ctx, chainID)
}

// RevokeAllForUser marks every active token owned by the given user as
// revoked. Used as the server-side implementation of logout-all.
func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	return r.q.RevokeAllForUser(ctx, userID)
}

func refreshTokenToModel(row sqlc.RefreshToken) *models.RefreshToken {
	return &models.RefreshToken{
		ID:        row.ID,
		UserID:    row.UserID,
		TokenHash: row.TokenHash,
		ChainID:   row.ChainID,
		RevokedAt: row.RevokedAt,
		ExpiresAt: row.ExpiresAt,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}
