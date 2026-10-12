-- name: CreateOIDCFlow :exec
INSERT INTO oidc_flows (token_hash, provider, purpose, state, nonce, code_verifier, user_id, session_token_hash, return_to, expires_at, then_link)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: ConsumeOIDCFlow :one
DELETE FROM oidc_flows
WHERE token_hash = $1 AND provider = $2 AND expires_at > NOW()
RETURNING *;

-- name: DeleteExpiredOIDCFlows :execrows
DELETE FROM oidc_flows WHERE expires_at <= NOW();
