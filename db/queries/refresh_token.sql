-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (user_id, token_hash, chain_id, expires_at)
VALUES (sqlc.arg(user_id), sqlc.arg(token_hash), sqlc.arg(chain_id), sqlc.arg(expires_at))
RETURNING id, user_id, token_hash, chain_id, revoked_at, expires_at, created_at, updated_at;

-- name: GetRefreshTokenByHash :one
SELECT id, user_id, token_hash, chain_id, revoked_at, expires_at, created_at, updated_at
FROM refresh_tokens
WHERE token_hash = sqlc.arg(token_hash);

-- name: RevokeRefreshToken :one
UPDATE refresh_tokens
SET revoked_at = now(), updated_at = now()
WHERE id = sqlc.arg(id) AND revoked_at IS NULL
RETURNING id;

-- name: RevokeChain :exec
UPDATE refresh_tokens
SET revoked_at = now(), updated_at = now()
WHERE chain_id = sqlc.arg(chain_id) AND revoked_at IS NULL;

-- name: RevokeAllForUser :exec
UPDATE refresh_tokens
SET revoked_at = now(), updated_at = now()
WHERE user_id = sqlc.arg(user_id) AND revoked_at IS NULL;
