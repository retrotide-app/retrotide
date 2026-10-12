package repository

import (
	"context"
	"database/sql"
	"fmt"

	"retrotide/internal/dao"
	"retrotide/internal/db"
	"retrotide/internal/model/spec"
)

type (
	PasswordResetRepository interface {
		dao.PasswordResetDAO

		Issue(ctx context.Context, s spec.NewPasswordReset, tx ...*sql.Tx) error
	}

	passwordResetRepository struct {
		db *sql.DB
		dao.PasswordResetDAO
	}
)

func NewPasswordResetRepo(database *sql.DB, resetDAO dao.PasswordResetDAO) PasswordResetRepository {
	return &passwordResetRepository{db: database, PasswordResetDAO: resetDAO}
}

func (r *passwordResetRepository) Issue(ctx context.Context, s spec.NewPasswordReset, tx ...*sql.Tx) error {
	err := db.WithTx(ctx, r.db, tx, func(tx *sql.Tx) error {
		if err := r.PasswordResetDAO.DeleteUnusedForUser(ctx, s.UserID, tx); err != nil {
			return err
		}

		return r.PasswordResetDAO.Create(ctx, s, tx)
	})
	if err != nil {
		return fmt.Errorf("issue password reset token: %w", err)
	}

	return nil
}
