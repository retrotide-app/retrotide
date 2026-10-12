package dao

import (
	"context"
	"database/sql"
	"fmt"

	"retrotide/internal/dao/sqlcgen"
	"retrotide/internal/db"
	"retrotide/internal/model/spec"
	"retrotide/internal/role"
)

type (
	PermissionDAO interface {
		GetRolePermissions(ctx context.Context, tx ...*sql.Tx) (map[role.Role][]string, error)
		SetRolePermissions(ctx context.Context, s spec.RolePermissionsUpdate, tx ...*sql.Tx) error
	}

	permissionDAO struct {
		db *sql.DB
	}
)

func (r *permissionDAO) GetRolePermissions(ctx context.Context, tx ...*sql.Tx) (map[role.Role][]string, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListRolePermissions(ctx)
	if err != nil {
		return nil, fmt.Errorf("get role permissions: %w", err)
	}

	byRole := make(map[role.Role][]string)
	for _, row := range rows {
		byRole[role.Role(row.Role)] = append(byRole[role.Role(row.Role)], row.Permission)
	}

	return byRole, nil
}

func (r *permissionDAO) SetRolePermissions(ctx context.Context, s spec.RolePermissionsUpdate, tx ...*sql.Tx) error {
	err := db.WithTx(ctx, r.db, tx, func(tx *sql.Tx) error {
		queries := sqlcgen.New(tx)

		if err := queries.DeleteRolePermissions(ctx, string(s.Role)); err != nil {
			return fmt.Errorf("clear the permissions of role %s: %w", s.Role, err)
		}

		for _, permission := range s.Permissions {
			err := queries.InsertRolePermission(ctx, sqlcgen.InsertRolePermissionParams{
				Role:       string(s.Role),
				Permission: permission,
			})
			if err != nil {
				return fmt.Errorf("grant %s to role %s: %w", permission, s.Role, err)
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("set the permissions of role %s: %w", s.Role, err)
	}

	return nil
}
