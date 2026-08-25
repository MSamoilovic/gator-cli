-- name: GetFeedStatsForUser :many
-- Jedan red po feedu koji korisnik prati, sa svime sto treba da se proceni
-- vredi li ga i dalje pratiti: koliko objavljuje, koliko si od toga otvorio i
-- kad je poslednji put dao znak zivota.
--
-- Feed bez ijednog posta nema MAX, pa se prazan rezultat svodi na nulto vreme
-- umesto na NULL — isti dogovor koji GetPostsForUserFiltered koristi za @since.
SELECT
    feeds.id AS feed_id,
    feeds.name AS feed_name,
    feeds.url AS feed_url,
    feed_follows.category,
    feeds.failure_count,
    COUNT(posts.id) AS post_count,
    COUNT(posts.id) FILTER (WHERE posts.published_at >= @since::timestamp) AS recent_count,
    COUNT(post_reads.post_id) AS read_count,
    COUNT(bookmarks.post_id) AS bookmark_count,
    COALESCE(MAX(posts.published_at), '0001-01-01 00:00:00'::timestamp)::timestamp AS last_published,
    COALESCE(MAX(post_reads.read_at), '0001-01-01 00:00:00'::timestamp)::timestamp AS last_read
FROM feed_follows
JOIN feeds ON feeds.id = feed_follows.feed_id
LEFT JOIN posts ON posts.feed_id = feeds.id
LEFT JOIN post_reads
    ON post_reads.post_id = posts.id
   AND post_reads.user_id = feed_follows.user_id
LEFT JOIN bookmarks
    ON bookmarks.post_id = posts.id
   AND bookmarks.user_id = feed_follows.user_id
WHERE feed_follows.user_id = @user_id
GROUP BY feeds.id, feeds.name, feeds.url, feed_follows.category, feeds.failure_count
ORDER BY post_count DESC, feeds.name ASC;
