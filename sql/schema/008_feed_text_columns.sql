-- +goose Up
-- Ime feeda se izvlaci iz <title>, a URL-ovi feedova umeju da budu dugacki
-- (YouTube kanali, Google News teme). Postgres nema korist od duzinskog
-- ogranicenja, pa TEXT uklanja celu klasu gresaka.
ALTER TABLE feeds ALTER COLUMN name TYPE TEXT;
ALTER TABLE feeds ALTER COLUMN url TYPE TEXT;

-- +goose Down
-- Namerno bez USING left(...): ako podaci ne staju nazad, bolje da migracija
-- padne nego da tiho skrati URL i time pokvari feed.
ALTER TABLE feeds ALTER COLUMN name TYPE VARCHAR(40);
ALTER TABLE feeds ALTER COLUMN url TYPE VARCHAR(80);
