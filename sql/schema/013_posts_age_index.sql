-- +goose Up
CREATE INDEX posts_age_idx ON posts (COALESCE(published_at, created_at));

-- +goose Down
DROP INDEX posts_age_idx;
