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
	InvitationRepository interface {
		dao.InvitationDAO

		Redeem(ctx context.Context, s spec.InvitationRedemption, tx ...*sql.Tx) (*model.User, error)
	}

	invitationRepository struct {
		dao.InvitationDAO

		db    *sql.DB
		users dao.UserDAO
	}
)

func NewInvitationRepo(database *sql.DB, invitationDAO dao.InvitationDAO, users dao.UserDAO) InvitationRepository {
	return &invitationRepository{InvitationDAO: invitationDAO, db: database, users: users}
}

func (r *invitationRepository) Redeem(ctx context.Context, s spec.InvitationRedemption, tx ...*sql.Tx) (*model.User, error) {
	var created *model.User

	err := db.WithTx(ctx, r.db, tx, func(tx *sql.Tx) error {
		invitation, err := r.InvitationDAO.Consume(ctx, s.TokenHash, tx)
		if err != nil {
			return err
		}

		account := s.User
		account.Email = invitation.Email
		account.EmailVerified = true

		user, err := r.users.Create(ctx, account, tx)
		if err != nil {
			return err
		}

		created = user

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("redeem invitation: %w", err)
	}

	return created, nil
}
