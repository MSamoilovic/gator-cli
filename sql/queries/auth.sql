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
INSERT INTO api_tokens (id, user_id, token_hash, created_at, expires_at)
VALUES (
    @id,
    @user_id,
    @token_hash,
    @created_at,
    now() + make_interval(days => @lifetime_days::int)
)
RETURNING *;

-- name: GetUserByTokenHash :one
SELECT users.* FROM users
JOIN api_tokens ON api_tokens.user_id = users.id
WHERE api_tokens.token_hash = $1
  AND api_tokens.expires_at > now();

-- name: TouchAPIToken :execrows
UPDATE api_tokens
SET last_used_at = now(),
    expires_at = now() + make_interval(days => @lifetime_days::int)
WHERE token_hash = @token_hash
  AND (last_used_at IS NULL OR last_used_at < now() - interval '1 day');

-- name: DeleteAPITokensForUser :one
WITH gone AS (
    DELETE FROM api_tokens WHERE user_id = $1
    RETURNING expires_at
)
SELECT count(*) FILTER (WHERE expires_at > now()) FROM gone;

-- name: DeleteExpiredAPITokens :execrows
DELETE FROM api_tokens WHERE expires_at <= now();

-- name: DeleteAPIToken :execrows
DELETE FROM api_tokens WHERE token_hash = $1;
