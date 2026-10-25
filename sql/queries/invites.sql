-- name: CreateInvite :one
INSERT INTO invites (id, token_hash, note, created_at, expires_at)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
)
RETURNING *;

-- name: GetInvite :one
SELECT invites.*, (invites.expires_at <= @now::timestamp) AS expired
FROM invites
WHERE token_hash = @token_hash::text;

-- name: ClaimInvite :one
UPDATE invites
SET claimed_at = @now::timestamp
WHERE token_hash = @token_hash::text
  AND claimed_at IS NULL
  AND expires_at > @now::timestamp
RETURNING *;

-- name: ReleaseInvite :execrows
UPDATE invites
SET claimed_at = NULL
WHERE id = $1 AND user_id IS NULL;

-- name: SetInviteUser :execrows
UPDATE invites
SET user_id = $2
WHERE id = $1;

-- name: ListInvites :many
SELECT invites.*, COALESCE(users.name, '') AS user_name,
    (invites.expires_at <= @now::timestamp) AS expired
FROM invites
LEFT JOIN users ON users.id = invites.user_id
ORDER BY invites.created_at DESC;

-- name: DeleteInvite :execrows
DELETE FROM invites
WHERE id = $1 AND claimed_at IS NULL;
