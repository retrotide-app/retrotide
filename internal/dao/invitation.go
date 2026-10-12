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
	InvitationDAO interface {
		Create(ctx context.Context, s spec.NewInvitation, tx ...*sql.Tx) (*model.Invitation, error)
		Renew(ctx context.Context, s spec.InvitationRenewal, tx ...*sql.Tx) (*model.Invitation, error)
		GetByID(ctx context.Context, id uuid.UUID, tx ...*sql.Tx) (*model.Invitation, error)
		Delete(ctx context.Context, id uuid.UUID, tx ...*sql.Tx) (*model.Invitation, error)
		GetLiveByTokenHash(ctx context.Context, tokenHash string, tx ...*sql.Tx) (*model.Invitation, error)
		Consume(ctx context.Context, tokenHash string, tx ...*sql.Tx) (*model.Invitation, error)
		List(ctx context.Context, s spec.InvitationListing, tx ...*sql.Tx) ([]model.Invitation, error)
		Count(ctx context.Context, tx ...*sql.Tx) (int, error)
	}

	invitationDAO struct {
		db *sql.DB
	}
)

func toInvitation(row sqlcgen.Invitation) *model.Invitation {
	return &model.Invitation{
		ID:        row.ID,
		Email:     row.Email,
		TokenHash: row.TokenHash,
		ExpiresAt: row.ExpiresAt,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

func (r *invitationDAO) Create(ctx context.Context, s spec.NewInvitation, tx ...*sql.Tx) (*model.Invitation, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CreateInvitation(ctx, sqlcgen.CreateInvitationParams{
		Email:     s.Email,
		TokenHash: s.TokenHash,
		ExpiresAt: s.ExpiresAt,
	})
	if isUniqueViolation(err) {
		return nil, fmt.Errorf("create invitation: %w: %w", ErrConflict, err)
	}
	if err != nil {
		return nil, fmt.Errorf("create invitation: %w", err)
	}

	return toInvitation(row), nil
}

func (r *invitationDAO) Renew(ctx context.Context, s spec.InvitationRenewal, tx ...*sql.Tx) (*model.Invitation, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).RenewInvitation(ctx, sqlcgen.RenewInvitationParams{
		ID:        s.ID,
		TokenHash: s.TokenHash,
		ExpiresAt: s.ExpiresAt,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("renew invitation %s: %w", s.ID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("renew invitation %s: %w", s.ID, err)
	}

	return toInvitation(row), nil
}

func (r *invitationDAO) GetByID(ctx context.Context, id uuid.UUID, tx ...*sql.Tx) (*model.Invitation, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetInvitationByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get invitation %s: %w", id, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get invitation %s: %w", id, err)
	}

	return toInvitation(row), nil
}

func (r *invitationDAO) Delete(ctx context.Context, id uuid.UUID, tx ...*sql.Tx) (*model.Invitation, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteInvitation(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("delete invitation %s: %w", id, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("delete invitation %s: %w", id, err)
	}

	return toInvitation(row), nil
}

func (r *invitationDAO) GetLiveByTokenHash(ctx context.Context, tokenHash string, tx ...*sql.Tx) (*model.Invitation, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetLiveInvitationByTokenHash(ctx, tokenHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get invitation by token: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get invitation by token: %w", err)
	}

	return toInvitation(row), nil
}

func (r *invitationDAO) Consume(ctx context.Context, tokenHash string, tx ...*sql.Tx) (*model.Invitation, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ConsumeInvitation(ctx, tokenHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("consume invitation: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("consume invitation: %w", err)
	}

	return toInvitation(row), nil
}

func (r *invitationDAO) List(ctx context.Context, s spec.InvitationListing, tx ...*sql.Tx) ([]model.Invitation, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListInvitations(ctx, sqlcgen.ListInvitationsParams{
		PageLimit:  int32(s.Limit),
		PageOffset: int32(s.Offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}

	invitations := make([]model.Invitation, 0, len(rows))
	for _, row := range rows {
		invitations = append(invitations, *toInvitation(row))
	}

	return invitations, nil
}

func (r *invitationDAO) Count(ctx context.Context, tx ...*sql.Tx) (int, error) {
	count, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CountInvitations(ctx)
	if err != nil {
		return 0, fmt.Errorf("count invitations: %w", err)
	}

	return int(count), nil
}
