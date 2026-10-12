-- name: CreateGuestDevice :one
INSERT INTO guest_devices (token_hash, expires_at) VALUES ($1, $2) RETURNING *;

-- name: GetLiveGuestDevice :one
SELECT * FROM guest_devices WHERE token_hash = $1 AND expires_at > NOW();

-- name: TouchGuestDevice :exec
UPDATE guest_devices SET last_seen_at = NOW() WHERE id = $1;

-- name: DeleteGuestDevice :exec
DELETE FROM guest_devices WHERE token_hash = $1;

-- name: DeleteGuestDeviceByID :exec
DELETE FROM guest_devices WHERE id = $1;

-- name: DeleteStaleGuestDevices :execrows
DELETE FROM guest_devices WHERE expires_at <= NOW() OR last_seen_at <= sqlc.arg(unseen_before)::timestamptz;
