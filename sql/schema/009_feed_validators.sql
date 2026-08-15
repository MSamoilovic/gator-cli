-- +goose Up
-- Otisci koje server salje uz feed, da bi se sledeci put moglo pitati "je li se
-- promenilo?" umesto da se skida ceo XML. Vrednosti se cuvaju kao TEXT i vracaju
-- serveru doslovno onakve kakve su stigle: ETag je neprozirna niska (moze biti i
-- W/"..."), a Last-Modified bi se preformatiranjem lako razisao sa onim sto
-- server ocekuje.
ALTER TABLE feeds ADD COLUMN etag TEXT NOT NULL DEFAULT '';
ALTER TABLE feeds ADD COLUMN last_modified TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE feeds DROP COLUMN etag;
ALTER TABLE feeds DROP COLUMN last_modified;
