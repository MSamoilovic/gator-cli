-- +goose Up
-- Trag o tome da li je poslednje povlacenje uspelo. Bez ovoga se iz baze ne
-- moze razlikovati ziv feed od mrtvog: MarkFeedFetched se namerno izvrsava pre
-- preuzimanja, pa i feed koji mesecima vraca 403 ima svez last_fetched_at.
--
-- last_error cuva poruku poslednjeg neuspeha, a prazna niska znaci "poslednji
-- pokusaj je uspeo". failure_count broji uzastopne neuspehe i resetuje se na
-- prvi uspeh, pa razlikuje trenutni prekid mreze od izvora koji je stvarno pao.
ALTER TABLE feeds ADD COLUMN last_error TEXT NOT NULL DEFAULT '';
ALTER TABLE feeds ADD COLUMN failure_count INT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE feeds DROP COLUMN last_error;
ALTER TABLE feeds DROP COLUMN failure_count;
