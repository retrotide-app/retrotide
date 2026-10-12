package dao

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"retrotide/internal/dao/sqlcgen"
	"retrotide/internal/db"
	"retrotide/internal/model/spec"
	"retrotide/internal/role"

	"github.com/google/uuid"
)

type (
	RoleDAO interface {
		GetRole(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) (role.Role, error)
		SetRole(ctx context.Context, s spec.RoleAssignment, tx ...*sql.Tx) error
		RemoveRole(ctx context.Context, s spec.RoleAssignment, tx ...*sql.Tx) error
	}

	roleDAO struct {
		db *sql.DB
	}
)

func (r *roleDAO) GetRole(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) (role.Role, error) {
	current, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetUserRole(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get the role of user %s: %w", userID, err)
	}

	return role.Role(current), nil
}

func (r *roleDAO) SetRole(ctx context.Context, s spec.RoleAssignment, tx ...*sql.Tx) error {
	err := sqlcgen.New(db.TxOrDB(r.db, tx)).SetUserRole(ctx, sqlcgen.SetUserRoleParams{
		UserID: s.UserID,
		Role:   string(s.Role),
	})
	if isForeignKeyViolation(err) {
		return fmt.Errorf("set role %s for user %s: %w: %w", s.Role, s.UserID, ErrNotFound, err)
	}
	if err != nil {
		return fmt.Errorf("set role %s for user %s: %w", s.Role, s.UserID, err)
	}

	return nil
}

func (r *roleDAO) RemoveRole(ctx context.Context, s spec.RoleAssignment, tx ...*sql.Tx) error {
	removed, err := sqlcgen.New(db.TxOrDB(r.db, tx)).RemoveUserRole(ctx, sqlcgen.RemoveUserRoleParams{
		UserID: s.UserID,
		Role:   string(s.Role),
	})
	if err != nil {
		return fmt.Errorf("remove role %s from user %s: %w", s.Role, s.UserID, err)
	}

	if removed == 0 {
		return fmt.Errorf("remove role %s from user %s: %w", s.Role, s.UserID, ErrNotFound)
	}

	return nil
}
