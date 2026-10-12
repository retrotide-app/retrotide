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
	EmailVerificationRepository interface {
		dao.EmailVerificationDAO

		Issue(ctx context.Context, s spec.NewEmailVerification, tx ...*sql.Tx) error
	}

	emailVerificationRepository struct {
		db *sql.DB
		dao.EmailVerificationDAO
	}
)

func NewEmailVerificationRepo(database *sql.DB, verificationDAO dao.EmailVerificationDAO) EmailVerificationRepository {
	return &emailVerificationRepository{db: database, EmailVerificationDAO: verificationDAO}
}

func (r *emailVerificationRepository) Issue(ctx context.Context, s spec.NewEmailVerification, tx ...*sql.Tx) error {
	err := db.WithTx(ctx, r.db, tx, func(tx *sql.Tx) error {
		if err := r.EmailVerificationDAO.DeleteUnusedForUser(ctx, s.UserID, tx); err != nil {
			return err
		}

		return r.EmailVerificationDAO.Create(ctx, s, tx)
	})
	if err != nil {
		return fmt.Errorf("issue email verification token: %w", err)
	}

	return nil
}
