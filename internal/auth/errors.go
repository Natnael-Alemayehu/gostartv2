package auth

import "errors"

// ErrInvalidCredentials is returned when an email/password pair does not match
// a known user. It is intentionally identical for "no such user" and "wrong
// password" to prevent username enumeration.
var ErrInvalidCredentials = errors.New("auth: invalid credentials")

// ErrTokenExpired is returned when an access token's exp claim is in the past.
var ErrTokenExpired = errors.New("auth: token expired")

// ErrTokenInvalid is returned when an access token fails signature, issuer,
// or structural validation. It is the catch-all for any non-expired parse
// failure so callers do not need to import the underlying JWT library.
var ErrTokenInvalid = errors.New("auth: token invalid")

// ErrRefreshTokenRevoked is returned when a refresh token lookup finds no row
// or a row that has already been revoked. The user-facing message is the same
// in both cases ("session expired") to avoid disclosing server state.
var ErrRefreshTokenRevoked = errors.New("auth: refresh token revoked")

// ErrRefreshTokenReuse is returned when a refresh token that is already
// revoked is presented again. Reuse is the strongest available signal of
// token theft; the entire chain is revoked as a defensive measure before
// this error is returned to the caller.
var ErrRefreshTokenReuse = errors.New("auth: refresh token reuse detected")

// ErrRefreshTokenExpired is returned when a refresh token's expires_at is in
// the past. Distinct from ErrRefreshTokenRevoked so callers can return the
// appropriate user-facing code.
var ErrRefreshTokenExpired = errors.New("auth: refresh token expired")
