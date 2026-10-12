-- +goose Up

CREATE TABLE guest_devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT guest_devices_token_hash_key UNIQUE (token_hash),
    CONSTRAINT guest_devices_token_hash_length CHECK (char_length(token_hash) = 64)
);
CREATE INDEX idx_guest_devices_expires_at ON guest_devices (expires_at);
CREATE INDEX idx_guest_devices_last_seen_at ON guest_devices (last_seen_at);

ALTER TABLE board_participants
    ADD COLUMN guest_device_id UUID,
    ADD CONSTRAINT board_participants_guest_device_id_fkey FOREIGN KEY (guest_device_id) REFERENCES guest_devices (id) ON DELETE SET NULL,
    ADD CONSTRAINT board_participants_board_guest_key UNIQUE (board_id, guest_device_id),
    ADD CONSTRAINT board_participants_one_principal CHECK (num_nonnulls(user_id, guest_device_id) <= 1);
CREATE INDEX idx_board_participants_guest ON board_participants (guest_device_id) WHERE guest_device_id IS NOT NULL;

-- +goose Down

DROP INDEX idx_board_participants_guest;
ALTER TABLE board_participants
    DROP CONSTRAINT board_participants_one_principal,
    DROP CONSTRAINT board_participants_board_guest_key,
    DROP CONSTRAINT board_participants_guest_device_id_fkey,
    DROP COLUMN guest_device_id;

DROP TABLE guest_devices;
