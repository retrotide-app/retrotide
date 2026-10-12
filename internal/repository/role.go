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

	"github.com/google/uuid"
)

type (
	RoleRepository interface {
		dao.RoleDAO

		AssignToNewAccount(ctx context.Context, s spec.RoleAssignment, tx ...*sql.Tx) error
	}

	roleRepository struct {
		dao.RoleDAO
		cache *cache.Manager
	}
)

func NewRoleRepo(roleDAO dao.RoleDAO, c *cache.Manager) RoleRepository {
	return &roleRepository{RoleDAO: roleDAO, cache: c}
}

func (r *roleRepository) GetRole(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) (role.Role, error) {
	load := func(ctx context.Context) (role.Role, error) {
		return r.RoleDAO.GetRole(ctx, userID, tx...)
	}

	return r.cache.Load(ctx, cache.UserRole, load, userID.String())
}

func (r *roleRepository) SetRole(ctx context.Context, s spec.RoleAssignment, tx ...*sql.Tx) error {
	if db.Joined(tx) != nil {
		return fmt.Errorf("set role %s for user %s: %w", s.Role, s.UserID, db.ErrInboundTx)
	}

	if err := r.RoleDAO.SetRole(ctx, s); err != nil {
		return err
	}

	return r.invalidate(ctx, s.UserID)
}

func (r *roleRepository) RemoveRole(ctx context.Context, s spec.RoleAssignment, tx ...*sql.Tx) error {
	if db.Joined(tx) != nil {
		return fmt.Errorf("remove role %s from user %s: %w", s.Role, s.UserID, db.ErrInboundTx)
	}

	if err := r.RoleDAO.RemoveRole(ctx, s); err != nil {
		return err
	}

	return r.invalidate(ctx, s.UserID)
}

func (r *roleRepository) AssignToNewAccount(ctx context.Context, s spec.RoleAssignment, tx ...*sql.Tx) error {
	if db.Joined(tx) == nil {
		return fmt.Errorf("assign role %s to new account %s: %w", s.Role, s.UserID, dao.ErrTxRequired)
	}

	return r.RoleDAO.SetRole(ctx, s, tx...)
}

func (r *roleRepository) invalidate(ctx context.Context, userID uuid.UUID) error {
	if err := r.cache.Del(context.WithoutCancel(ctx), cache.UserRole.Key(userID.String())); err != nil {
		return fmt.Errorf("role of user %s %w: %w", userID, ErrStaleStaffCache, err)
	}

	return nil
}
