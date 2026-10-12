package dao

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"retrotide/internal/dao/sqlcgen"
	"retrotide/internal/db"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

type (
	EmailVerificationDAO interface {
		Create(ctx context.Context, s spec.NewEmailVerification, tx ...*sql.Tx) error
		Consume(ctx context.Context, tokenHash string, tx ...*sql.Tx) (*model.EmailVerificationToken, error)
		DeleteUnusedForUser(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) error
		DeletePendingEmailChangesForUser(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) error
	}

	emailVerificationDAO struct {
		db *sql.DB
	}
)

func (r *emailVerificationDAO) Create(ctx context.Context, s spec.NewEmailVerification, tx ...*sql.Tx) error {
	err := sqlcgen.New(db.TxOrDB(r.db, tx)).CreateEmailVerificationToken(ctx, sqlcgen.CreateEmailVerificationTokenParams{
		TokenHash:    s.TokenHash,
		UserID:       s.UserID,
		PendingEmail: s.PendingEmail,
		ExpiresAt:    s.ExpiresAt,
	})
	if err != nil {
		return fmt.Errorf("create email verification token for user %s: %w", s.UserID, err)
	}

	return nil
}

func (r *emailVerificationDAO) Consume(ctx context.Context, tokenHash string, tx ...*sql.Tx) (*model.EmailVerificationToken, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ConsumeEmailVerificationToken(ctx, tokenHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("consume email verification token: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("consume email verification token: %w", err)
	}

	return &model.EmailVerificationToken{
		TokenHash:    row.TokenHash,
		UserID:       row.UserID,
		PendingEmail: row.PendingEmail,
		ExpiresAt:    row.ExpiresAt,
		UsedAt:       nullableTime(row.UsedAt),
		CreatedAt:    row.CreatedAt,
	}, nil
}

func (r *emailVerificationDAO) DeleteUnusedForUser(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) error {
	if err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteUnusedEmailVerificationTokensForUser(ctx, userID); err != nil {
		return fmt.Errorf("delete unused email verification tokens for user %s: %w", userID, err)
	}

	return nil
}

func (r *emailVerificationDAO) DeletePendingEmailChangesForUser(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) error {
	if err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeletePendingEmailChangesForUser(ctx, userID); err != nil {
		return fmt.Errorf("delete pending email changes for user %s: %w", userID, err)
	}

	return nil
}
