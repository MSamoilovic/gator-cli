-- name: MarkPostRead :exec
INSERT INTO post_reads (user_id, post_id, read_at)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, post_id) DO NOTHING;

-- name: MarkPostUnread :exec
DELETE FROM post_reads
WHERE user_id = $1 AND post_id = $2;

-- name: MarkPostsRead :exec
INSERT INTO post_reads (user_id, post_id, read_at)
SELECT @user_id, unnest(@post_ids::uuid[]), @read_at
ON CONFLICT (user_id, post_id) DO NOTHING;

-- name: GetReadPostIDs :many
SELECT post_id FROM post_reads
WHERE user_id = $1;

-- name: GetUnreadCountsForUser :many
SELECT posts.feed_id, count(*) AS unread
FROM posts
JOIN feed_follows ON posts.feed_id = feed_follows.feed_id
LEFT JOIN post_reads
    ON post_reads.post_id = posts.id
   AND post_reads.user_id = feed_follows.user_id
WHERE feed_follows.user_id = $1
  AND post_reads.post_id IS NULL
GROUP BY posts.feed_id;
