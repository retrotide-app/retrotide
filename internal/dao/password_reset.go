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
	PasswordResetDAO interface {
		Create(ctx context.Context, s spec.NewPasswordReset, tx ...*sql.Tx) error
		Consume(ctx context.Context, tokenHash string, tx ...*sql.Tx) (*model.PasswordResetToken, error)
		DeleteUnusedForUser(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) error
	}

	passwordResetDAO struct {
		db *sql.DB
	}
)

func (r *passwordResetDAO) Create(ctx context.Context, s spec.NewPasswordReset, tx ...*sql.Tx) error {
	err := sqlcgen.New(db.TxOrDB(r.db, tx)).CreatePasswordResetToken(ctx, sqlcgen.CreatePasswordResetTokenParams{
		TokenHash: s.TokenHash,
		UserID:    s.UserID,
		ExpiresAt: s.ExpiresAt,
	})
	if err != nil {
		return fmt.Errorf("create password reset token for user %s: %w", s.UserID, err)
	}

	return nil
}

func (r *passwordResetDAO) Consume(ctx context.Context, tokenHash string, tx ...*sql.Tx) (*model.PasswordResetToken, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ConsumePasswordResetToken(ctx, tokenHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("consume password reset token: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("consume password reset token: %w", err)
	}

	return &model.PasswordResetToken{
		TokenHash: row.TokenHash,
		UserID:    row.UserID,
		ExpiresAt: row.ExpiresAt,
		UsedAt:    nullableTime(row.UsedAt),
		CreatedAt: row.CreatedAt,
	}, nil
}

func (r *passwordResetDAO) DeleteUnusedForUser(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) error {
	if err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteUnusedPasswordResetTokensForUser(ctx, userID); err != nil {
		return fmt.Errorf("delete unused password reset tokens for user %s: %w", userID, err)
	}

	return nil
}
