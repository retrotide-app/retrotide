-- name: ListRolePermissions :many
SELECT role, permission FROM role_permissions ORDER BY role, permission;

-- name: DeleteRolePermissions :exec
DELETE FROM role_permissions WHERE role = $1;

-- name: InsertRolePermission :exec
INSERT INTO role_permissions (role, permission) VALUES ($1, $2) ON CONFLICT DO NOTHING;
