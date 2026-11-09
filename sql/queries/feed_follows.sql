-- name: CreateFeedFollow :many
WITH inserted AS (
    INSERT INTO feed_follows (id, created_at, updated_at, user_id, feed_id)
    VALUES ($1, $2, $3, $4, $5)
    ON CONFLICT (user_id, feed_id) DO NOTHING
    RETURNING *
)
SELECT
    inserted.*,
    users.name AS user_name,
    feeds.name AS feed_name
FROM inserted
JOIN users ON inserted.user_id = users.id
JOIN feeds ON inserted.feed_id = feeds.id;

-- name: UnfollowFeed :one
WITH removed_bookmarks AS (
    DELETE FROM bookmarks
    USING posts
    WHERE bookmarks.post_id = posts.id
      AND posts.feed_id = @feed_id
      AND bookmarks.user_id = @user_id
    RETURNING bookmarks.post_id
), removed_reads AS (
    DELETE FROM post_reads
    USING posts
    WHERE post_reads.post_id = posts.id
      AND posts.feed_id = @feed_id
      AND post_reads.user_id = @user_id
), removed_follow AS (
    DELETE FROM feed_follows
    WHERE feed_follows.user_id = @user_id
      AND feed_follows.feed_id = @feed_id
)
SELECT count(*) FROM removed_bookmarks;

-- name: GetFeedFollowsForUser :many
SELECT
    feed_follows.*,
    users.name AS user_name,
    feeds.name AS feed_name,
    feeds.url AS feed_url,
    feeds.failure_count AS feed_failures
FROM feed_follows
JOIN users ON feed_follows.user_id = users.id
JOIN feeds ON feed_follows.feed_id = feeds.id
WHERE feed_follows.user_id = $1;
-- name: SetFeedFollowCategory :exec
UPDATE feed_follows
SET category = $3, updated_at = NOW()
WHERE user_id = $1 AND feed_id = $2;
