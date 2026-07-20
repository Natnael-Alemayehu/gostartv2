package models

import (
	"time"

	"github.com/google/uuid"
)

// RefreshToken is the domain representation of a stored refresh-token row. The
// TokenHash field carries a SHA-256 hex digest of the plaintext token, never
// the plaintext itself. RevokedAt is nil while the token is active and set to
// the revocation timestamp once rotated or revoked.
type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ChainID   uuid.UUID
	RevokedAt *time.Time
	ExpiresAt time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// RefreshTokenCreate carries the fields required to insert a refresh token.
// Plaintext token is never stored; callers hash it before populating
// TokenHash.
type RefreshTokenCreate struct {
	UserID    uuid.UUID
	TokenHash string
	ChainID   uuid.UUID
	ExpiresAt time.Time
}
