-- +goose Up
ALTER TABLE users ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE admin_log (
    id UUID PRIMARY KEY,
    at TIMESTAMP NOT NULL,
    user_id UUID REFERENCES users (id) ON DELETE SET NULL,
    user_name TEXT NOT NULL,
    command TEXT NOT NULL,
    args TEXT NOT NULL
);

CREATE INDEX admin_log_at_idx ON admin_log (at DESC);

-- +goose Down
DROP TABLE admin_log;
ALTER TABLE users DROP COLUMN is_admin;
