-- name: CreateEmailVerificationToken :exec
INSERT INTO email_verification_tokens (token_hash, user_id, pending_email, expires_at) VALUES ($1, $2, $3, $4);

-- name: ConsumeEmailVerificationToken :one
UPDATE email_verification_tokens SET used_at = NOW()
WHERE token_hash = $1 AND used_at IS NULL AND expires_at > NOW()
RETURNING *;

-- name: DeleteUnusedEmailVerificationTokensForUser :exec
DELETE FROM email_verification_tokens WHERE user_id = $1 AND used_at IS NULL;

-- name: DeletePendingEmailChangesForUser :exec
DELETE FROM email_verification_tokens WHERE user_id = $1 AND used_at IS NULL AND pending_email IS NOT NULL;
