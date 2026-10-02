-- +goose Up
ALTER TABLE feeds ALTER COLUMN user_id DROP NOT NULL;

ALTER TABLE feeds DROP CONSTRAINT feeds_user_id_fkey;
ALTER TABLE feeds ADD CONSTRAINT feeds_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE feeds DROP CONSTRAINT feeds_user_id_fkey;
ALTER TABLE feeds ADD CONSTRAINT feeds_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE feeds ALTER COLUMN user_id SET NOT NULL;
