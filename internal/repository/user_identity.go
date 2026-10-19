package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"retrotide/internal/dao"
	"retrotide/internal/db"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
)

type (
	UserIdentityRepository interface {
		dao.UserIdentityDAO

		RegisterWithIdentity(ctx context.Context, s spec.IdentityRegistration, tx ...*sql.Tx) (*model.User, error)
		LinkToAccount(ctx context.Context, s spec.IdentityLink, tx ...*sql.Tx) (*model.UserIdentity, error)
		Unlink(ctx context.Context, s spec.UserIdentityRemoval, tx ...*sql.Tx) (*model.UserIdentity, error)
	}

	userIdentityRepository struct {
		dao.UserIdentityDAO

		db       *sql.DB
		users    dao.UserDAO
		roles    RoleRepository
		sessions SessionRepository
	}

	UserIdentityRepoDeps struct {
		Users    dao.UserDAO
		Roles    RoleRepository
		Sessions SessionRepository
	}
)

func NewUserIdentityRepo(database *sql.DB, identityDAO dao.UserIdentityDAO, deps UserIdentityRepoDeps) UserIdentityRepository {
	return &userIdentityRepository{
		UserIdentityDAO: identityDAO,
		db:              database,
		users:           deps.Users,
		roles:           deps.Roles,
		sessions:        deps.Sessions,
	}
}

func (r *userIdentityRepository) RegisterWithIdentity(ctx context.Context, s spec.IdentityRegistration, tx ...*sql.Tx) (*model.User, error) {
	var created *model.User

	err := db.WithTx(ctx, r.db, tx, func(tx *sql.Tx) error {
		user, err := createAccount(ctx, r.users, r.roles, s.User, s.FirstAccount, tx)
		if err != nil {
			return err
		}

		identity := spec.NewUserIdentity{UserID: user.ID, Provider: s.Provider, Subject: s.Subject, Email: user.Email}
		if _, err := r.insert(ctx, identity, tx); err != nil {
			return err
		}

		created = user

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("register an account with a %s identity: %w", s.Provider, err)
	}

	return created, nil
}

func (r *userIdentityRepository) LinkToAccount(ctx context.Context, s spec.IdentityLink, tx ...*sql.Tx) (*model.UserIdentity, error) {
	var linked *model.UserIdentity

	err := db.WithTx(ctx, r.db, tx, func(tx *sql.Tx) error {
		account, err := r.users.GetForUpdate(ctx, s.Identity.UserID, tx)
		if err != nil {
			return err
		}

		banned, err := r.banHolds(ctx, *account, tx)
		if err != nil {
			return err
		}

		if banned {
			return ErrAccountBanned
		}

		if s.Automatic && (account.Email != s.MatchedEmail || !account.EmailVerified) {
			return ErrAccountChanged
		}

		held := spec.UserIdentityKey{UserID: account.ID, Provider: s.Identity.Provider}
		_, err = r.UserIdentityDAO.GetForUser(ctx, held, tx)
		if err == nil {
			return ErrAlreadyLinked
		}
		if !errors.Is(err, dao.ErrNotFound) {
			return err
		}

		identity, err := r.insert(ctx, s.Identity, tx)
		if err != nil {
			return err
		}

		if s.Automatic {
			if err := r.sessions.DeleteAllForUser(ctx, account.ID, tx); err != nil {
				return err
			}
		}

		linked = identity

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("link a %s identity to user %s: %w", s.Identity.Provider, s.Identity.UserID, err)
	}

	return linked, nil
}

func (r *userIdentityRepository) Unlink(ctx context.Context, s spec.UserIdentityRemoval, tx ...*sql.Tx) (*model.UserIdentity, error) {
	var removed *model.UserIdentity

	err := db.WithTx(ctx, r.db, tx, func(tx *sql.Tx) error {
		if _, err := r.users.GetForUpdate(ctx, s.UserID, tx); err != nil {
			return err
		}

		identity, err := r.UserIdentityDAO.GetForUser(ctx, s.UserIdentityKey, tx)
		if err != nil {
			return err
		}

		if err := r.UserIdentityDAO.DeleteUnlessLast(ctx, s, tx); err != nil {
			return err
		}

		removed = identity

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("unlink the %s identity of user %s: %w", s.Provider, s.UserID, err)
	}

	return removed, nil
}

func (r *userIdentityRepository) banHolds(ctx context.Context, account model.User, tx *sql.Tx) (bool, error) {
	if account.BannedAt == nil {
		return false, nil
	}

	held, err := r.roles.GetRole(ctx, account.ID, tx)
	if err != nil {
		return true, fmt.Errorf("read the role of banned user %s: %w", account.ID, err)
	}

	return !held.BypassesRestrictions(), nil
}

func (r *userIdentityRepository) insert(ctx context.Context, s spec.NewUserIdentity, tx *sql.Tx) (*model.UserIdentity, error) {
	created, err := r.UserIdentityDAO.Create(ctx, s, tx)
	if errors.Is(err, dao.ErrConflict) {
		return nil, fmt.Errorf("%w: %w", ErrIdentityInUse, err)
	}
	if err != nil {
		return nil, err
	}

	return created, nil
}
