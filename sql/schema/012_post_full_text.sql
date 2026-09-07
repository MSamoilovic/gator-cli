-- +goose Up
ALTER TABLE posts ADD COLUMN full_text TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE posts DROP COLUMN full_text;
