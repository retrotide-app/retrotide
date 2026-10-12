-- name: CreateAuditLogEntry :exec
INSERT INTO audit_log (org_id, actor_id, actor_kind, action, target_type, target_id, subject_id, details, ip)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: ListSiteAuditLog :many
SELECT a.id, a.org_id, a.actor_id, a.actor_kind, a.action, a.target_type, a.target_id, a.subject_id, a.details, a.ip, a.created_at,
       COALESCE(actor.display_name, '')::text AS actor_name,
       COALESCE(subject.display_name, '')::text AS subject_name
FROM audit_log a
LEFT JOIN users actor ON actor.id = a.actor_id
LEFT JOIN users subject ON subject.id = a.subject_id
WHERE a.org_id IS NULL AND (sqlc.arg(action)::text = '' OR a.action = sqlc.arg(action)::text)
ORDER BY a.created_at DESC, a.id DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountSiteAuditLog :one
SELECT COUNT(*) FROM audit_log
WHERE org_id IS NULL AND (sqlc.arg(action)::text = '' OR action = sqlc.arg(action)::text);

-- name: ListSiteAuditLogForSubject :many
SELECT a.id, a.org_id, a.actor_id, a.actor_kind, a.action, a.target_type, a.target_id, a.subject_id, a.details, a.ip, a.created_at,
       COALESCE(actor.display_name, '')::text AS actor_name,
       COALESCE(subject.display_name, '')::text AS subject_name
FROM audit_log a
LEFT JOIN users actor ON actor.id = a.actor_id
LEFT JOIN users subject ON subject.id = a.subject_id
WHERE a.org_id IS NULL AND a.subject_id = sqlc.arg(subject_id)
ORDER BY a.created_at DESC, a.id DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountSiteAuditLogForSubject :one
SELECT COUNT(*) FROM audit_log WHERE org_id IS NULL AND subject_id = $1;

-- name: ClearAuditLogIPsBefore :execrows
UPDATE audit_log SET ip = NULL WHERE ip IS NOT NULL AND created_at < sqlc.arg(cutoff)::timestamptz;

-- name: DeleteAuditLogCreatedBefore :execrows
DELETE FROM audit_log WHERE created_at < sqlc.arg(cutoff)::timestamptz;
