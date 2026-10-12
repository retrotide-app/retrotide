-- +goose Up
ALTER TABLE two_factor_challenges ADD COLUMN remember BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE two_factor_trusted_devices (
    token_hash TEXT PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES user_totp (user_id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT two_factor_trusted_devices_token_hash_length CHECK (char_length(token_hash) = 64)
);
CREATE INDEX idx_two_factor_trusted_devices_user ON two_factor_trusted_devices (user_id);
CREATE INDEX idx_two_factor_trusted_devices_expires_at ON two_factor_trusted_devices (expires_at);

-- +goose Down
DROP TABLE two_factor_trusted_devices;
ALTER TABLE two_factor_challenges DROP COLUMN remember;
