-- name: CreatePasswordResetToken :exec
INSERT INTO password_reset_tokens (token_hash, user_id, expires_at) VALUES ($1, $2, $3);

-- name: ConsumePasswordResetToken :one
UPDATE password_reset_tokens SET used_at = NOW()
WHERE token_hash = $1 AND used_at IS NULL AND expires_at > NOW()
RETURNING *;

-- name: DeleteUnusedPasswordResetTokensForUser :exec
DELETE FROM password_reset_tokens WHERE user_id = $1 AND used_at IS NULL;
