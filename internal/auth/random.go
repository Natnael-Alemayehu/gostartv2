package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// RefreshTokenBytes is the length of entropy used for a refresh token. 32
// bytes (256 bits) matches the security level of the HMAC-SHA256 signing key
// and is well above the brute-force threshold for online attackers.
const RefreshTokenBytes = 32

// GenerateRefreshToken returns a URL-safe refresh token string suitable for
// use in an HttpOnly cookie. The token is base64url-encoded without padding
// so it survives cookie serialization unchanged.
func GenerateRefreshToken() (string, error) {
	b := make([]byte, RefreshTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: read random bytes: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashRefreshToken returns the lowercase hex SHA-256 digest of the plaintext
// token. The hash, not the plaintext, is stored in the database so a DB breach
// does not leak live sessions. Comparison of stored hashes by SQL equality is
// safe because the input space is large enough to defeat brute force on the
// hash itself.
func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
