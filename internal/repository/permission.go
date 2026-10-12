package repository

import (
	"context"
	"database/sql"
	"fmt"

	"retrotide/internal/cache"
	"retrotide/internal/dao"
	"retrotide/internal/db"
	"retrotide/internal/model/spec"
	"retrotide/internal/role"
)

type (
	PermissionRepository interface {
		dao.PermissionDAO
	}

	permissionRepository struct {
		dao.PermissionDAO
		cache *cache.Manager
	}
)

func NewPermissionRepo(permissionDAO dao.PermissionDAO, c *cache.Manager) PermissionRepository {
	return &permissionRepository{PermissionDAO: permissionDAO, cache: c}
}

func (r *permissionRepository) GetRolePermissions(ctx context.Context, tx ...*sql.Tx) (map[role.Role][]string, error) {
	load := func(ctx context.Context) (map[role.Role][]string, error) {
		return r.PermissionDAO.GetRolePermissions(ctx, tx...)
	}

	return r.cache.Load(ctx, cache.RolePermissions, load)
}

func (r *permissionRepository) SetRolePermissions(ctx context.Context, s spec.RolePermissionsUpdate, tx ...*sql.Tx) error {
	if db.Joined(tx) != nil {
		return fmt.Errorf("set the permissions of role %s: %w", s.Role, db.ErrInboundTx)
	}

	if err := r.PermissionDAO.SetRolePermissions(ctx, s); err != nil {
		return err
	}

	if err := r.cache.Del(context.WithoutCancel(ctx), cache.RolePermissions.Key()); err != nil {
		return fmt.Errorf("permissions of role %s %w: %w", s.Role, ErrStaleStaffCache, err)
	}

	return nil
}
