-- +goose Up
-- Kategorija (kod citaca: folder) stoji na feed_follows, ne na feeds, jer je to
-- korisnikova organizacija: isti feed dvoje ljudi moze da drzi pod razlicitim
-- imenom foldera. Prazna niska znaci "u korenu", sto je i podrazumevano stanje
-- za sve postojece pretplate.
ALTER TABLE feed_follows ADD COLUMN category TEXT NOT NULL DEFAULT '';

CREATE INDEX feed_follows_user_category_idx ON feed_follows (user_id, category);

-- +goose Down
DROP INDEX feed_follows_user_category_idx;
ALTER TABLE feed_follows DROP COLUMN category;
