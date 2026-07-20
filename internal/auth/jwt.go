// Package auth provides JWT access-token signing and verification plus the
// helpers used to mint and hash opaque refresh tokens. The package is pure
// (no database or HTTP awareness) so it can be unit-tested in isolation and
// reused by services, handlers, and middleware.
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims is the payload of an access token. UserID and ChainID are custom
// claims; the rest of the standard JWT fields are carried by the embedded
// RegisteredClaims. ChainID groups all refresh tokens issued from one login
// so the service can revoke an entire session chain on reuse detection.
type Claims struct {
	UserID  uuid.UUID `json:"uid"`
	ChainID uuid.UUID `json:"cid"`
	jwt.RegisteredClaims
}

// Signer produces signed access tokens using HMAC-SHA256. The same secret
// must be paired with a Verifier constructed from the same secret.
type Signer struct {
	secret    []byte
	issuer    string
	accessTTL time.Duration
}

// Verifier validates access tokens produced by the matching Signer.
type Verifier struct {
	secret []byte
	issuer string
}

// NewSigner constructs a Signer from the configured secret, issuer, and
// access-token TTL. The secret is copied into a slice so the caller may
// safely retain its own copy.
func NewSigner(secret, issuer string, accessTTL time.Duration) *Signer {
	return &Signer{
		secret:    []byte(secret),
		issuer:    issuer,
		accessTTL: accessTTL,
	}
}

// NewVerifier constructs a Verifier from the configured secret and issuer.
func NewVerifier(secret, issuer string) *Verifier {
	return &Verifier{
		secret: []byte(secret),
		issuer: issuer,
	}
}

// Sign issues a signed access token for the given user and chain. The token
// expires after the Signer's configured accessTTL.
func (s *Signer) Sign(userID, chainID uuid.UUID) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		UserID:  userID,
		ChainID: chainID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    s.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("auth: sign access token: %w", err)
	}

	return signed, nil
}

// Verify parses and validates the token string. A token whose exp claim is in
// the past returns ErrTokenExpired; any other validation or parse failure
// returns ErrTokenInvalid. Callers need not import the jwt library to
// discriminate between failure modes.
func (v *Verifier) Verify(tokenStr string) (*Claims, error) {
	var claims Claims

	parser := jwt.NewParser(jwt.WithIssuer(v.issuer), jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))

	_, err := parser.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}

		return v.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}

		return nil, ErrTokenInvalid
	}

	return &claims, nil
}
