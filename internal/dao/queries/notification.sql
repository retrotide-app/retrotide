-- name: CreateNotification :one
INSERT INTO notifications (user_id, org_id, type, reference_type, reference_id, actor_id, message)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListNotificationsForUser :many
SELECT * FROM notifications
WHERE user_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: CountNotificationsForUser :one
SELECT COUNT(*) FROM notifications WHERE user_id = $1;

-- name: CountUnreadNotificationsForUser :one
SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND NOT read;

-- name: MarkNotificationRead :execrows
UPDATE notifications SET read = TRUE WHERE id = $1 AND user_id = $2;

-- name: MarkAllNotificationsRead :execrows
UPDATE notifications SET read = TRUE WHERE user_id = $1 AND NOT read;

-- name: DeleteNotificationsCreatedBefore :execrows
DELETE FROM notifications WHERE created_at < $1;
