-- name: CreateUser :one
INSERT INTO users (email, password_hash, name)
VALUES (sqlc.arg(email), sqlc.arg(password_hash), sqlc.arg(name))
RETURNING id, email, password_hash, name, created_at, updated_at;

-- name: GetUserByID :one
SELECT id, email, password_hash, name, created_at, updated_at
FROM users
WHERE id = sqlc.arg(id);

-- name: GetUserByEmail :one
SELECT id, email, password_hash, name, created_at, updated_at
FROM users
WHERE email = sqlc.arg(email);

-- name: ListUsers :many
SELECT id, email, password_hash, name, created_at, updated_at
FROM users
WHERE (created_at, id) < (
    COALESCE(sqlc.narg(cursor_created_at)::timestamptz, 'infinity'::timestamptz),
    COALESCE(sqlc.narg(cursor_id)::uuid, '00000000-0000-0000-0000-000000000000'::uuid)
)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(max_rows);

-- name: UpdateUser :one
UPDATE users
SET
    email         = COALESCE(sqlc.narg(email), email),
    password_hash = COALESCE(sqlc.narg(password_hash), password_hash),
    name          = COALESCE(sqlc.narg(name), name),
    updated_at    = now()
WHERE id = sqlc.arg(id)
RETURNING id, email, password_hash, name, created_at, updated_at;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = sqlc.arg(id);

-- name: CountUsers :one
SELECT COUNT(*) FROM users;
