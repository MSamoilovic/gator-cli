-- +goose Up
DELETE FROM posts p
WHERE p.url <> btrim(p.url, E' \t\n\r')
  AND EXISTS (SELECT 1 FROM posts q WHERE q.url = btrim(p.url, E' \t\n\r'));

UPDATE posts
SET url = btrim(url, E' \t\n\r')
WHERE url <> btrim(url, E' \t\n\r');

-- +goose Down
SELECT 1;
