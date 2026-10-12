-- +goose Up

INSERT INTO role_permissions (role, permission) VALUES ('support', 'view_organisations')
ON CONFLICT (role, permission) DO NOTHING;

-- +goose Down

DELETE FROM role_permissions WHERE role = 'support' AND permission = 'view_organisations';
