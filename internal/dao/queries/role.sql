-- name: GetUserRole :one
SELECT role FROM user_roles WHERE user_id = $1;

-- name: SetUserRole :exec
INSERT INTO user_roles (user_id, role) VALUES ($1, $2)
ON CONFLICT (user_id) DO UPDATE SET role = EXCLUDED.role;

-- name: RemoveUserRole :execrows
DELETE FROM user_roles WHERE user_id = $1 AND role = $2;
