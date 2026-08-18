-- name: CreateFeed :one
INSERT INTO feeds (id, created_at, updated_at, name, url, user_id)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING *;

-- name: GetFeeds :many
SELECT feeds.name, feeds.url, users.name AS user_name,
       feeds.last_error, feeds.failure_count
FROM feeds
JOIN users ON feeds.user_id = users.id;

-- name: GetFeedByUrl :one
SELECT * FROM feeds WHERE url = $1;

-- name: MarkFeedFetched :exec
UPDATE feeds
SET last_fetched_at = NOW(), updated_at = NOW()
WHERE id = $1;

-- name: SaveFeedValidators :exec
-- Odvojeno od MarkFeedFetched, koji se namerno izvrsava pre preuzimanja da feed
-- koji stalno puca ne bi bio pokusavan u svakom krugu. Validatori se, naprotiv,
-- znaju tek posle uspesnog odgovora.
UPDATE feeds
SET etag = $2, last_modified = $3, updated_at = NOW()
WHERE id = $1;

-- name: GetNextFeedToFetch :one
SELECT * FROM feeds
ORDER BY last_fetched_at ASC NULLS FIRST
LIMIT 1;

-- name: GetFeedsToFetch :many
SELECT * FROM feeds
ORDER BY last_fetched_at ASC NULLS FIRST;

-- name: MarkFeedFailed :exec
-- failure_count raste po uzastopnim neuspesima; resetuje ga tek uspeh.
UPDATE feeds
SET last_error = $2, failure_count = failure_count + 1, updated_at = NOW()
WHERE id = $1;

-- name: MarkFeedHealthy :exec
-- Uslov na kraju stedi upis kod feedova koji su i inace zdravi, sto je vecina
-- u svakom krugu.
UPDATE feeds
SET last_error = '', failure_count = 0, updated_at = NOW()
WHERE id = $1 AND (last_error <> '' OR failure_count <> 0);

-- name: GetBrokenFeeds :many
SELECT id, name, url, last_error, failure_count, last_fetched_at
FROM feeds
WHERE failure_count > 0
ORDER BY failure_count DESC, name;
