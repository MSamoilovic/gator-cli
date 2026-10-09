-- +goose Up
ALTER TABLE users ALTER COLUMN name TYPE TEXT;
ALTER TABLE users ADD COLUMN email TEXT;
ALTER TABLE users ADD COLUMN password_hash TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX users_email_idx ON users (lower(email));

-- +goose Down
DROP INDEX users_email_idx;
ALTER TABLE users DROP COLUMN password_hash;
ALTER TABLE users DROP COLUMN email;
ALTER TABLE users ALTER COLUMN name TYPE VARCHAR(20);
