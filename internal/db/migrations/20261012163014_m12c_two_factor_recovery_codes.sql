-- +goose Up
CREATE TABLE two_factor_recovery_codes (
    user_id UUID NOT NULL REFERENCES user_totp (user_id) ON DELETE CASCADE,
    code_hash TEXT NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, code_hash),
    CONSTRAINT two_factor_recovery_codes_code_hash_length CHECK (char_length(code_hash) = 64)
);

-- +goose Down
DROP TABLE two_factor_recovery_codes;
