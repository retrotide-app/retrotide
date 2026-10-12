-- +goose Up
CREATE TABLE user_totp (
    user_id UUID PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    secret TEXT NOT NULL,
    confirmed_at TIMESTAMPTZ,
    last_used_step BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT user_totp_secret_length CHECK (char_length(secret) = 32),
    CONSTRAINT user_totp_last_used_step_check CHECK (last_used_step >= 0)
);

CREATE TABLE two_factor_challenges (
    token_hash TEXT PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    return_to TEXT,
    attempts INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT two_factor_challenges_token_hash_length CHECK (char_length(token_hash) = 64),
    CONSTRAINT two_factor_challenges_attempts_check CHECK (attempts >= 0)
);
CREATE INDEX idx_two_factor_challenges_expires_at ON two_factor_challenges (expires_at);
CREATE INDEX idx_two_factor_challenges_user ON two_factor_challenges (user_id);

-- +goose Down
DROP TABLE two_factor_challenges;
DROP TABLE user_totp;
