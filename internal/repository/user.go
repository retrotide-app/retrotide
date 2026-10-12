package repository

import (
	"context"
	"database/sql"
	"fmt"

	"retrotide/internal/dao"
	"retrotide/internal/db"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/role"

	"github.com/google/uuid"
)

type (
	UserRepository interface {
		dao.UserDAO

		RegisterAccount(ctx context.Context, s spec.NewRegistration, tx ...*sql.Tx) (*model.User, error)
		ResetPassword(ctx context.Context, s spec.PasswordReset, tx ...*sql.Tx) (uuid.UUID, error)
		ChangePassword(ctx context.Context, s spec.PasswordChange, tx ...*sql.Tx) error
		ConfirmEmailVerification(ctx context.Context, tokenHash string, tx ...*sql.Tx) (*model.EmailVerificationToken, error)
		BanAccount(ctx context.Context, s spec.UserBan, tx ...*sql.Tx) error
		UnverifyAccount(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) error
		DeleteAccount(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) error
	}

	userRepository struct {
		dao.UserDAO

		db            *sql.DB
		roles         RoleRepository
		sessions      SessionRepository
		verifications EmailVerificationRepository
		resets        PasswordResetRepository
		identities    UserIdentityRepository
		organisations dao.OrganisationDAO
		twoFactor     dao.TwoFactorDAO
	}

	UserRepoDeps struct {
		Roles         RoleRepository
		Sessions      SessionRepository
		Verifications EmailVerificationRepository
		Resets        PasswordResetRepository
		Identities    UserIdentityRepository
		Organisations dao.OrganisationDAO
		TwoFactor     dao.TwoFactorDAO
	}
)

func NewUserRepo(database *sql.DB, userDAO dao.UserDAO, deps UserRepoDeps) UserRepository {
	return &userRepository{
		UserDAO:       userDAO,
		db:            database,
		roles:         deps.Roles,
		sessions:      deps.Sessions,
		verifications: deps.Verifications,
		resets:        deps.Resets,
		identities:    deps.Identities,
		organisations: deps.Organisations,
		twoFactor:     deps.TwoFactor,
	}
}

func (r *userRepository) RegisterAccount(ctx context.Context, s spec.NewRegistration, tx ...*sql.Tx) (*model.User, error) {
	var created *model.User

	err := db.WithTx(ctx, r.db, tx, func(tx *sql.Tx) error {
		user, err := createAccount(ctx, r.UserDAO, r.roles, s.User, s.FirstAccount, tx)
		if err != nil {
			return err
		}

		if !user.EmailVerified {
			verification := spec.NewEmailVerification{TokenHash: s.VerificationHash, UserID: user.ID, ExpiresAt: s.VerificationExpiresAt}
			if err := r.verifications.Create(ctx, verification, tx); err != nil {
				return err
			}
		}

		created = user

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("register account: %w", err)
	}

	return created, nil
}

func createAccount(ctx context.Context, users dao.UserDAO, roles RoleRepository, account spec.NewUser, firstAccount bool, tx *sql.Tx) (*model.User, error) {
	if !firstAccount {
		return users.Create(ctx, account, tx)
	}

	if err := claimFirstAccount(ctx, users, tx); err != nil {
		return nil, err
	}

	account.EmailVerified = true

	user, err := users.Create(ctx, account, tx)
	if err != nil {
		return nil, err
	}

	grant := spec.RoleAssignment{UserID: user.ID, Role: role.SuperAdmin}
	if err := roles.AssignToNewAccount(ctx, grant, tx); err != nil {
		return nil, err
	}

	return user, nil
}

func claimFirstAccount(ctx context.Context, users dao.UserDAO, tx *sql.Tx) error {
	if err := users.LockFirstAccountClaim(ctx, tx); err != nil {
		return err
	}

	exists, err := users.AnyExists(ctx, tx)
	if err != nil {
		return err
	}

	if exists {
		return ErrNotFirstAccount
	}

	return nil
}

func (r *userRepository) BanAccount(ctx context.Context, s spec.UserBan, tx ...*sql.Tx) error {
	err := db.WithTx(ctx, r.db, tx, func(tx *sql.Tx) error {
		if err := r.UserDAO.Ban(ctx, s, tx); err != nil {
			return err
		}

		return r.sessions.DeleteAllForUser(ctx, s.UserID, tx)
	})
	if err != nil {
		return fmt.Errorf("ban account: %w", err)
	}

	return nil
}

func (r *userRepository) UnverifyAccount(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) error {
	err := db.WithTx(ctx, r.db, tx, func(tx *sql.Tx) error {
		if err := r.UserDAO.MarkEmailUnverified(ctx, userID, tx); err != nil {
			return err
		}

		return r.sessions.DeleteAllForUser(ctx, userID, tx)
	})
	if err != nil {
		return fmt.Errorf("unverify account: %w", err)
	}

	return nil
}

func (r *userRepository) DeleteAccount(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) error {
	err := db.WithTx(ctx, r.db, tx, func(tx *sql.Tx) error {
		if _, err := r.organisations.LockAllOfMember(ctx, userID, tx); err != nil {
			return err
		}

		if _, err := r.UserDAO.GetForUpdate(ctx, userID, tx); err != nil {
			return err
		}

		shared, err := r.organisations.ListOwnedWithOtherMembers(ctx, userID, tx)
		if err != nil {
			return err
		}

		if len(shared) > 0 {
			return fmt.Errorf("%w: %s", ErrOwnsSharedOrganisation, shared[0].Slug)
		}

		if _, err := r.organisations.SoftDeleteOwnedAlone(ctx, userID, tx); err != nil {
			return err
		}

		return r.UserDAO.Delete(ctx, userID, tx)
	})
	if err != nil {
		return fmt.Errorf("delete account %s: %w", userID, err)
	}

	return nil
}

func (r *userRepository) ResetPassword(ctx context.Context, s spec.PasswordReset, tx ...*sql.Tx) (uuid.UUID, error) {
	var userID uuid.UUID

	err := db.WithTx(ctx, r.db, tx, func(tx *sql.Tx) error {
		token, err := r.resets.Consume(ctx, s.TokenHash, tx)
		if err != nil {
			return err
		}

		account, err := r.UserDAO.GetForUpdate(ctx, token.UserID, tx)
		if err != nil {
			return err
		}

		update := spec.UserPasswordHashUpdate{UserID: token.UserID, PasswordHash: s.PasswordHash}
		if err := r.UserDAO.SetPasswordHash(ctx, update, tx); err != nil {
			return err
		}

		if err := r.sessions.DeleteAllForUser(ctx, token.UserID, tx); err != nil {
			return err
		}

		if err := r.twoFactor.DeleteTrustedDevicesForUser(ctx, token.UserID, tx); err != nil {
			return err
		}

		if err := r.verifications.DeletePendingEmailChangesForUser(ctx, token.UserID, tx); err != nil {
			return err
		}

		if !account.EmailVerified {
			if _, err := r.identities.DeleteAllForUser(ctx, token.UserID, tx); err != nil {
				return err
			}
		}

		userID = token.UserID

		return nil
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("reset password: %w", err)
	}

	return userID, nil
}

func (r *userRepository) ChangePassword(ctx context.Context, s spec.PasswordChange, tx ...*sql.Tx) error {
	err := db.WithTx(ctx, r.db, tx, func(tx *sql.Tx) error {
		update := spec.UserPasswordHashUpdate{UserID: s.UserID, PasswordHash: s.PasswordHash}
		if err := r.UserDAO.SetPasswordHash(ctx, update, tx); err != nil {
			return err
		}

		keep := spec.SessionDeletionExcept{UserID: s.UserID, KeepToken: s.KeepToken}
		if err := r.sessions.DeleteAllForUserExcept(ctx, keep, tx); err != nil {
			return err
		}

		if err := r.twoFactor.DeleteTrustedDevicesForUser(ctx, s.UserID, tx); err != nil {
			return err
		}

		return r.verifications.DeletePendingEmailChangesForUser(ctx, s.UserID, tx)
	})
	if err != nil {
		return fmt.Errorf("change password: %w", err)
	}

	return nil
}

func (r *userRepository) ConfirmEmailVerification(ctx context.Context, tokenHash string, tx ...*sql.Tx) (*model.EmailVerificationToken, error) {
	var consumed *model.EmailVerificationToken

	err := db.WithTx(ctx, r.db, tx, func(tx *sql.Tx) error {
		token, err := r.verifications.Consume(ctx, tokenHash, tx)
		if err != nil {
			return err
		}

		if token.ChangesEmail() {
			err = r.UserDAO.SetVerifiedEmail(ctx, spec.UserVerifiedEmailUpdate{UserID: token.UserID, Email: *token.PendingEmail}, tx)
		} else {
			err = r.UserDAO.MarkEmailVerified(ctx, token.UserID, tx)
		}
		if err != nil {
			return err
		}

		consumed = token

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("confirm email verification: %w", err)
	}

	return consumed, nil
}
