-- name: CreateSavedTemplateWithinQuota :one
INSERT INTO saved_templates (org_id, team_id, owner_id, created_by, name, description, columns)
SELECT sqlc.arg(org_id)::uuid, sqlc.narg(team_id)::uuid, sqlc.narg(owner_id)::uuid, sqlc.arg(created_by)::uuid, sqlc.arg(name)::text, sqlc.arg(description)::text, sqlc.arg(columns)::jsonb
WHERE (
    SELECT COUNT(*) FROM saved_templates WHERE org_id = sqlc.arg(org_id)::uuid
) < sqlc.arg(max_templates)::int
RETURNING *;

-- name: GetSavedTemplate :one
SELECT st.*, COALESCE(t.name, '')::text AS team_name
FROM saved_templates st
LEFT JOIN teams t ON t.org_id = st.org_id AND t.id = st.team_id
WHERE st.org_id = $1 AND st.id = $2;

-- name: ListSavedTemplatesForViewer :many
SELECT st.*, COALESCE(t.name, '')::text AS team_name
FROM saved_templates st
LEFT JOIN teams t ON t.org_id = st.org_id AND t.id = st.team_id
WHERE st.org_id = $1 AND (st.owner_id = $2 OR st.team_id IS NOT NULL)
ORDER BY st.created_at, st.id;

-- name: CountSavedTemplates :one
SELECT COUNT(*)::int FROM saved_templates WHERE org_id = $1;

-- name: DeleteSavedTemplate :one
DELETE FROM saved_templates WHERE org_id = $1 AND id = $2 RETURNING *;
