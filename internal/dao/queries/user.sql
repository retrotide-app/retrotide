-- name: CreateUser :one
INSERT INTO users (email, password_hash, display_name, email_verified)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByIDForUpdate :one
SELECT * FROM users WHERE id = $1 FOR UPDATE;

-- name: UserEmailInUse :one
SELECT EXISTS (
    SELECT 1 FROM users WHERE email = sqlc.arg(email) AND id <> sqlc.arg(exclude_user_id)
) AS in_use;

-- name: SetUserPasswordHash :execrows
UPDATE users SET password_hash = sqlc.arg(password_hash)::text, updated_at = NOW() WHERE id = sqlc.arg(id);

-- name: SetUserDisplayName :execrows
UPDATE users SET display_name = $2, updated_at = NOW() WHERE id = $1;

-- name: MarkUserEmailVerified :execrows
UPDATE users SET email_verified = TRUE, updated_at = NOW() WHERE id = $1;

-- name: SetUserVerifiedEmail :execrows
UPDATE users SET email = $2, email_verified = TRUE, updated_at = NOW() WHERE id = $1;

-- name: UserIsBanned :one
SELECT (banned_at IS NOT NULL)::boolean AS banned FROM users WHERE id = $1;

-- name: UserIsLocked :one
SELECT (locked_at IS NOT NULL)::boolean AS locked FROM users WHERE id = $1;

-- name: UpdateUserLastSeenIP :execrows
UPDATE users SET last_seen_ip = sqlc.arg(last_seen_ip)::text, last_seen_ip_at = NOW() WHERE id = sqlc.arg(id);

-- name: ClearUserLastSeenIPsBefore :execrows
UPDATE users SET last_seen_ip = NULL, last_seen_ip_at = NULL WHERE last_seen_ip_at < sqlc.arg(cutoff)::timestamptz;

-- name: DeleteUser :execrows
DELETE FROM users WHERE id = $1;

-- name: AnyUserExists :one
SELECT EXISTS (SELECT 1 FROM users) AS any_exists;

-- name: LockFirstAccountClaim :exec
SELECT pg_advisory_xact_lock(hashtextextended('retrotide:first-account', 0));

-- name: ListUsersWithRoles :many
SELECT sqlc.embed(users), COALESCE(user_roles.role, '')::text AS role
FROM users
LEFT JOIN user_roles ON user_roles.user_id = users.id
WHERE (sqlc.arg(pattern)::text = '' OR users.email ILIKE sqlc.arg(pattern)::text OR users.display_name ILIKE sqlc.arg(pattern)::text)
ORDER BY users.created_at DESC, users.id DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountUsers :one
SELECT COUNT(*) FROM users
WHERE (sqlc.arg(pattern)::text = '' OR email ILIKE sqlc.arg(pattern)::text OR display_name ILIKE sqlc.arg(pattern)::text);

-- name: BanUser :execrows
UPDATE users SET banned_at = NOW(), banned_by = sqlc.narg(banned_by), ban_reason = sqlc.arg(ban_reason), updated_at = NOW() WHERE id = sqlc.arg(id);

-- name: UnbanUser :execrows
UPDATE users SET banned_at = NULL, banned_by = NULL, ban_reason = '', updated_at = NOW() WHERE id = $1;

-- name: LockUser :execrows
UPDATE users SET locked_at = NOW(), locked_by = sqlc.narg(locked_by), lock_reason = sqlc.arg(lock_reason), updated_at = NOW() WHERE id = sqlc.arg(id);

-- name: UnlockUser :execrows
UPDATE users SET locked_at = NULL, locked_by = NULL, lock_reason = '', updated_at = NOW() WHERE id = $1;

-- name: MarkUserEmailUnverified :execrows
UPDATE users SET email_verified = FALSE, updated_at = NOW() WHERE id = $1;
