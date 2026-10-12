package repository

import (
	"context"
	"database/sql"
	"fmt"

	"retrotide/internal/dao"
	"retrotide/internal/db"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
)

type (
	TwoFactorRepository interface {
		dao.TwoFactorDAO

		Activate(ctx context.Context, s spec.TwoFactorActivation, tx ...*sql.Tx) (*model.UserTOTP, error)
	}

	twoFactorRepository struct {
		dao.TwoFactorDAO

		db            *sql.DB
		sessions      SessionRepository
		verifications EmailVerificationRepository
	}

	TwoFactorRepoDeps struct {
		Sessions      SessionRepository
		Verifications EmailVerificationRepository
	}
)

func NewTwoFactorRepo(database *sql.DB, twoFactorDAO dao.TwoFactorDAO, deps TwoFactorRepoDeps) TwoFactorRepository {
	return &twoFactorRepository{TwoFactorDAO: twoFactorDAO, db: database, sessions: deps.Sessions, verifications: deps.Verifications}
}

func (r *twoFactorRepository) Activate(ctx context.Context, s spec.TwoFactorActivation, tx ...*sql.Tx) (*model.UserTOTP, error) {
	var confirmed *model.UserTOTP

	err := db.WithTx(ctx, r.db, tx, func(tx *sql.Tx) error {
		totp, err := r.TwoFactorDAO.ConfirmTOTP(ctx, spec.TwoFactorStepUse{UserID: s.UserID, Step: s.Step}, tx)
		if err != nil {
			return err
		}

		for _, codeHash := range s.CodeHashes {
			if err := r.TwoFactorDAO.CreateRecoveryCode(ctx, spec.TwoFactorRecoveryCodeUse{UserID: s.UserID, CodeHash: codeHash}, tx); err != nil {
				return err
			}
		}

		keep := spec.SessionDeletionExcept{UserID: s.UserID, KeepToken: s.KeepToken}
		if err := r.sessions.DeleteAllForUserExcept(ctx, keep, tx); err != nil {
			return err
		}

		if err := r.verifications.DeletePendingEmailChangesForUser(ctx, s.UserID, tx); err != nil {
			return err
		}

		confirmed = totp

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("turn on two-factor authentication: %w", err)
	}

	return confirmed, nil
}
