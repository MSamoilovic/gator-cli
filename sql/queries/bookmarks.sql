-- name: CreateBookmark :many
INSERT INTO bookmarks (id, created_at, user_id, post_id)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, post_id) DO NOTHING
RETURNING *;

-- name: DeleteBookmark :exec
DELETE FROM bookmarks
WHERE user_id = $1 AND post_id = $2;

-- name: GetBookmarksForUser :many
SELECT posts.* FROM bookmarks
JOIN posts ON bookmarks.post_id = posts.id
WHERE bookmarks.user_id = $1
ORDER BY bookmarks.created_at DESC;

-- name: GetBookmarkedPostIDs :many
SELECT post_id FROM bookmarks
WHERE user_id = $1;
