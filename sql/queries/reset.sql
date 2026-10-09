-- name: CountAllRows :one
SELECT
    (SELECT count(*) FROM users)        AS users,
    (SELECT count(*) FROM feeds)        AS feeds,
    (SELECT count(*) FROM posts)        AS posts,
    (SELECT count(*) FROM feed_follows) AS feed_follows,
    (SELECT count(*) FROM bookmarks)    AS bookmarks,
    (SELECT count(*) FROM post_reads)   AS post_reads,
    (SELECT count(*) FROM api_tokens)   AS api_tokens;

-- name: TruncateAll :exec
TRUNCATE users, feeds, posts, feed_follows, bookmarks, post_reads, api_tokens;
