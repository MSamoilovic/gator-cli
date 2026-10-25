-- +goose Up
CREATE TABLE invites (
    id UUID PRIMARY KEY,
    token_hash TEXT UNIQUE NOT NULL,
    note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    claimed_at TIMESTAMP,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL
);

-- +goose Down
DROP TABLE invites;
