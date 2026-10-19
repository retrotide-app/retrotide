-- name: CreateUserIdentity :one
INSERT INTO user_identities (provider, subject, user_id, email)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetUserIdentityBySubject :one
SELECT * FROM user_identities WHERE provider = $1 AND subject = $2;

-- name: GetUserIdentityForUser :one
SELECT * FROM user_identities WHERE user_id = $1 AND provider = $2;

-- name: ListUserIdentitiesForUser :many
SELECT * FROM user_identities WHERE user_id = $1 ORDER BY created_at, provider;

-- name: DeleteUserIdentityUnlessLast :execrows
DELETE FROM user_identities
WHERE user_identities.user_id = sqlc.arg(user_id)
  AND user_identities.provider = sqlc.arg(provider)
  AND (
      (sqlc.arg(password_counts)::boolean AND EXISTS (SELECT 1 FROM users WHERE users.id = sqlc.arg(user_id) AND users.password_hash IS NOT NULL))
      OR EXISTS (SELECT 1 FROM user_identities AS other WHERE other.user_id = sqlc.arg(user_id) AND other.provider <> sqlc.arg(provider))
  );

-- name: DeleteUserIdentitiesForUser :execrows
DELETE FROM user_identities WHERE user_id = $1;
