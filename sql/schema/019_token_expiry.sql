-- +goose Up
ALTER TABLE api_tokens ADD COLUMN expires_at TIMESTAMP NOT NULL DEFAULT (now() + interval '90 days');

CREATE INDEX api_tokens_expires_at_idx ON api_tokens (expires_at);

-- +goose Down
ALTER TABLE api_tokens DROP COLUMN expires_at;
