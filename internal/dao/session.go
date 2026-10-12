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
	"retrotide/internal/token"

	"github.com/google/uuid"
)

type (
	SessionDAO interface {
		Create(ctx context.Context, s spec.NewSession, tx ...*sql.Tx) error
		Get(ctx context.Context, token string, tx ...*sql.Tx) (*model.Session, error)
		Delete(ctx context.Context, token string, tx ...*sql.Tx) error
		DeleteAllForUser(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) error
		DeleteAllForUserExcept(ctx context.Context, s spec.SessionDeletionExcept, tx ...*sql.Tx) error
		CleanExpired(ctx context.Context, tx ...*sql.Tx) (int64, error)
	}

	sessionDAO struct {
		db *sql.DB
	}
)

func (r *sessionDAO) Create(ctx context.Context, s spec.NewSession, tx ...*sql.Tx) error {
	err := sqlcgen.New(db.TxOrDB(r.db, tx)).CreateSession(ctx, sqlcgen.CreateSessionParams{
		Token:     token.Hash(s.Token),
		UserID:    s.UserID,
		ExpiresAt: s.ExpiresAt,
	})
	if err != nil {
		return fmt.Errorf("create session for user %s: %w", s.UserID, err)
	}

	return nil
}

func (r *sessionDAO) Get(ctx context.Context, presented string, tx ...*sql.Tx) (*model.Session, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetLiveSession(ctx, token.Hash(presented))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get session: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	return &model.Session{
		UserID:    row.UserID,
		CreatedAt: row.CreatedAt,
		ExpiresAt: row.ExpiresAt,
	}, nil
}

func (r *sessionDAO) Delete(ctx context.Context, presented string, tx ...*sql.Tx) error {
	if err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteSession(ctx, token.Hash(presented)); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}

func (r *sessionDAO) DeleteAllForUser(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) error {
	if err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteSessionsForUser(ctx, userID); err != nil {
		return fmt.Errorf("delete sessions for user %s: %w", userID, err)
	}

	return nil
}

func (r *sessionDAO) DeleteAllForUserExcept(ctx context.Context, s spec.SessionDeletionExcept, tx ...*sql.Tx) error {
	err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteSessionsForUserExcept(ctx, sqlcgen.DeleteSessionsForUserExceptParams{
		UserID: s.UserID,
		Token:  token.Hash(s.KeepToken),
	})
	if err != nil {
		return fmt.Errorf("delete other sessions for user %s: %w", s.UserID, err)
	}

	return nil
}

func (r *sessionDAO) CleanExpired(ctx context.Context, tx ...*sql.Tx) (int64, error) {
	removed, err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteExpiredSessions(ctx)
	if err != nil {
		return 0, fmt.Errorf("clean expired sessions: %w", err)
	}

	return removed, nil
}
