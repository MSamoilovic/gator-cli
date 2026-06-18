-- name: CreatePost :one
INSERT INTO posts (id, created_at, updated_at, title, url, description, published_at, feed_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetPostsForUser :many
SELECT posts.* FROM posts
JOIN feed_follows ON posts.feed_id = feed_follows.feed_id
WHERE feed_follows.user_id = $1
ORDER BY posts.published_at DESC
LIMIT $2;

-- name: GetPostsForUserFiltered :many
SELECT posts.* FROM posts
JOIN feed_follows ON posts.feed_id = feed_follows.feed_id
JOIN feeds ON posts.feed_id = feeds.id
WHERE feed_follows.user_id = @user_id
  AND (@feed_name::text = '' OR feeds.name ILIKE '%' || @feed_name::text || '%')
ORDER BY
  CASE WHEN @sort_dir::text = 'asc' THEN posts.published_at END ASC NULLS LAST,
  posts.published_at DESC NULLS LAST
LIMIT @post_limit
OFFSET @post_offset;
