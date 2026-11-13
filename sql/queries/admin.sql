-- name: CountAdmins :one
SELECT count(*) FROM users WHERE is_admin;

-- name: ListAdmins :many
SELECT * FROM users WHERE is_admin ORDER BY name;

-- name: GrantAdmin :execrows
UPDATE users SET is_admin = true, updated_at = @updated_at
WHERE users.id = @id
  AND NOT is_admin
  AND password_hash <> ''
  AND (NOT @only_if_none::bool OR NOT EXISTS (SELECT 1 FROM users AS other WHERE other.is_admin));

-- name: RevokeAdmin :execrows
UPDATE users SET is_admin = false, updated_at = @updated_at
WHERE users.id = @id
  AND is_admin
  AND EXISTS (SELECT 1 FROM users AS other WHERE other.is_admin AND other.id <> @id FOR UPDATE);

-- name: LogAdminAction :exec
INSERT INTO admin_log (id, at, user_id, user_name, command, args)
VALUES (@id, @at, @user_id, @user_name, @command, @args);

-- name: ListAdminLog :many
SELECT * FROM admin_log ORDER BY at DESC LIMIT @max_rows;
