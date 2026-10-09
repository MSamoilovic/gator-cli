-- name: CreateUserWithCredentials :one
INSERT INTO users (id, created_at, updated_at, name, email, password_hash)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE lower(email) = lower($1);

-- name: CreateAPIToken :one
INSERT INTO api_tokens (id, user_id, token_hash, created_at)
VALUES (
    $1,
    $2,
    $3,
    $4
)
RETURNING *;

-- name: GetUserByTokenHash :one
SELECT users.* FROM users
JOIN api_tokens ON api_tokens.user_id = users.id
WHERE api_tokens.token_hash = $1;
