-- +goose Up

ALTER TABLE room_participants
    ADD COLUMN joined_via TEXT NOT NULL DEFAULT 'membership',
    ADD CONSTRAINT room_participants_joined_via_check CHECK (joined_via IN ('membership', 'link'));

-- +goose Down

ALTER TABLE room_participants DROP COLUMN joined_via;
