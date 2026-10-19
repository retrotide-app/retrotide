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
	UserIdentityDAO interface {
		Create(ctx context.Context, s spec.NewUserIdentity, tx ...*sql.Tx) (*model.UserIdentity, error)
		GetBySubject(ctx context.Context, s spec.IdentitySubject, tx ...*sql.Tx) (*model.UserIdentity, error)
		GetForUser(ctx context.Context, s spec.UserIdentityKey, tx ...*sql.Tx) (*model.UserIdentity, error)
		ListForUser(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) ([]model.UserIdentity, error)
		DeleteUnlessLast(ctx context.Context, s spec.UserIdentityRemoval, tx ...*sql.Tx) error
		DeleteAllForUser(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) (int64, error)
	}

	userIdentityDAO struct {
		db *sql.DB
	}
)

func toUserIdentity(row sqlcgen.UserIdentity) *model.UserIdentity {
	return &model.UserIdentity{
		Provider:  model.IdentityProvider(row.Provider),
		Subject:   row.Subject,
		UserID:    row.UserID,
		Email:     row.Email,
		CreatedAt: row.CreatedAt,
	}
}

func (r *userIdentityDAO) Create(ctx context.Context, s spec.NewUserIdentity, tx ...*sql.Tx) (*model.UserIdentity, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CreateUserIdentity(ctx, sqlcgen.CreateUserIdentityParams{
		Provider: string(s.Provider),
		Subject:  s.Subject,
		UserID:   s.UserID,
		Email:    s.Email,
	})
	if isUniqueViolation(err) {
		return nil, fmt.Errorf("link %s identity to user %s: %w: %w", s.Provider, s.UserID, ErrConflict, err)
	}
	if err != nil {
		return nil, fmt.Errorf("link %s identity to user %s: %w", s.Provider, s.UserID, err)
	}

	return toUserIdentity(row), nil
}

func (r *userIdentityDAO) GetBySubject(ctx context.Context, s spec.IdentitySubject, tx ...*sql.Tx) (*model.UserIdentity, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetUserIdentityBySubject(ctx, sqlcgen.GetUserIdentityBySubjectParams{
		Provider: string(s.Provider),
		Subject:  s.Subject,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get %s identity by subject: %w", s.Provider, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get %s identity by subject: %w", s.Provider, err)
	}

	return toUserIdentity(row), nil
}

func (r *userIdentityDAO) GetForUser(ctx context.Context, s spec.UserIdentityKey, tx ...*sql.Tx) (*model.UserIdentity, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetUserIdentityForUser(ctx, sqlcgen.GetUserIdentityForUserParams{
		UserID:   s.UserID,
		Provider: string(s.Provider),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get %s identity of user %s: %w", s.Provider, s.UserID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get %s identity of user %s: %w", s.Provider, s.UserID, err)
	}

	return toUserIdentity(row), nil
}

func (r *userIdentityDAO) ListForUser(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) ([]model.UserIdentity, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListUserIdentitiesForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list identities of user %s: %w", userID, err)
	}

	identities := make([]model.UserIdentity, 0, len(rows))
	for _, row := range rows {
		identities = append(identities, *toUserIdentity(row))
	}

	return identities, nil
}

func (r *userIdentityDAO) DeleteUnlessLast(ctx context.Context, s spec.UserIdentityRemoval, tx ...*sql.Tx) error {
	deleted, err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteUserIdentityUnlessLast(ctx, sqlcgen.DeleteUserIdentityUnlessLastParams{
		UserID:         s.UserID,
		Provider:       string(s.Provider),
		PasswordCounts: s.PasswordCounts,
	})
	if err != nil {
		return fmt.Errorf("unlink %s identity of user %s: %w", s.Provider, s.UserID, err)
	}

	if deleted == 0 {
		return fmt.Errorf("unlink %s identity of user %s: %w", s.Provider, s.UserID, ErrLastSignInMethod)
	}

	return nil
}

func (r *userIdentityDAO) DeleteAllForUser(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) (int64, error) {
	deleted, err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteUserIdentitiesForUser(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("delete identities of user %s: %w", userID, err)
	}

	return deleted, nil
}
