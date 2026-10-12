-- name: CreateInvitation :one
INSERT INTO invitations (email, token_hash, expires_at) VALUES ($1, $2, $3) RETURNING *;

-- name: RenewInvitation :one
UPDATE invitations SET token_hash = $2, expires_at = $3, updated_at = NOW() WHERE id = $1 RETURNING *;

-- name: GetInvitationByID :one
SELECT * FROM invitations WHERE id = $1;

-- name: DeleteInvitation :one
DELETE FROM invitations WHERE id = $1 RETURNING *;

-- name: GetLiveInvitationByTokenHash :one
SELECT * FROM invitations WHERE token_hash = $1 AND expires_at > NOW();

-- name: ConsumeInvitation :one
DELETE FROM invitations WHERE token_hash = $1 AND expires_at > NOW() RETURNING *;

-- name: ListInvitations :many
SELECT * FROM invitations
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountInvitations :one
SELECT COUNT(*) FROM invitations;
