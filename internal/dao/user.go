package dao

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"retrotide/internal/dao/sqlcgen"
	"retrotide/internal/db"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/role"

	"github.com/google/uuid"
)

type (
	UserDAO interface {
		Create(ctx context.Context, s spec.NewUser, tx ...*sql.Tx) (*model.User, error)
		GetByID(ctx context.Context, id uuid.UUID, tx ...*sql.Tx) (*model.User, error)
		GetForUpdate(ctx context.Context, id uuid.UUID, tx ...*sql.Tx) (*model.User, error)
		GetByEmail(ctx context.Context, email string, tx ...*sql.Tx) (*model.User, error)
		EmailInUse(ctx context.Context, s spec.UserEmailFilter, tx ...*sql.Tx) (bool, error)
		SetPasswordHash(ctx context.Context, s spec.UserPasswordHashUpdate, tx ...*sql.Tx) error
		SetDisplayName(ctx context.Context, s spec.UserDisplayNameUpdate, tx ...*sql.Tx) error
		MarkEmailVerified(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) error
		SetVerifiedEmail(ctx context.Context, s spec.UserVerifiedEmailUpdate, tx ...*sql.Tx) error
		IsBanned(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) (bool, error)
		IsLocked(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) (bool, error)
		UpdateLastSeenIP(ctx context.Context, s spec.UserLastSeenIPUpdate, tx ...*sql.Tx) error
		ClearLastSeenIPsBefore(ctx context.Context, cutoff time.Time, tx ...*sql.Tx) (int64, error)
		Delete(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) error
		AnyExists(ctx context.Context, tx ...*sql.Tx) (bool, error)
		LockFirstAccountClaim(ctx context.Context, tx ...*sql.Tx) error
		ListWithRoles(ctx context.Context, s spec.UserListing, tx ...*sql.Tx) ([]model.UserWithRole, error)
		Count(ctx context.Context, search string, tx ...*sql.Tx) (int, error)
		Ban(ctx context.Context, s spec.UserBan, tx ...*sql.Tx) error
		Unban(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) error
		Lock(ctx context.Context, s spec.UserLock, tx ...*sql.Tx) error
		Unlock(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) error
		MarkEmailUnverified(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) error
	}

	userDAO struct {
		db *sql.DB
	}
)

func toUser(row sqlcgen.User) *model.User {
	return &model.User{
		ID:                 row.ID,
		Email:              row.Email,
		PasswordHash:       nullableString(row.PasswordHash),
		DisplayName:        row.DisplayName,
		EmailVerified:      row.EmailVerified,
		EmailNotifications: row.EmailNotifications,
		BannedAt:           nullableTime(row.BannedAt),
		BannedBy:           row.BannedBy,
		BanReason:          row.BanReason,
		LockedAt:           nullableTime(row.LockedAt),
		LockedBy:           row.LockedBy,
		LockReason:         row.LockReason,
		LastSeenIP:         nullableString(row.LastSeenIp),
		LastSeenIPAt:       nullableTime(row.LastSeenIpAt),
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}
}

func (r *userDAO) Create(ctx context.Context, s spec.NewUser, tx ...*sql.Tx) (*model.User, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CreateUser(ctx, sqlcgen.CreateUserParams{
		Email:         s.Email,
		PasswordHash:  toNullString(s.PasswordHash),
		DisplayName:   s.DisplayName,
		EmailVerified: s.EmailVerified,
	})
	if isUniqueViolation(err) {
		return nil, fmt.Errorf("create user: %w: %w", ErrConflict, err)
	}
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	return toUser(row), nil
}

func (r *userDAO) GetByID(ctx context.Context, id uuid.UUID, tx ...*sql.Tx) (*model.User, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetUserByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get user %s: %w", id, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get user %s: %w", id, err)
	}

	return toUser(row), nil
}

func (r *userDAO) GetForUpdate(ctx context.Context, id uuid.UUID, tx ...*sql.Tx) (*model.User, error) {
	joined := db.Joined(tx)
	if joined == nil {
		return nil, fmt.Errorf("lock user %s: %w", id, ErrTxRequired)
	}

	row, err := sqlcgen.New(joined).GetUserByIDForUpdate(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("lock user %s: %w", id, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("lock user %s: %w", id, err)
	}

	return toUser(row), nil
}

func (r *userDAO) GetByEmail(ctx context.Context, email string, tx ...*sql.Tx) (*model.User, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetUserByEmail(ctx, email)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get user by email: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}

	return toUser(row), nil
}

func (r *userDAO) EmailInUse(ctx context.Context, s spec.UserEmailFilter, tx ...*sql.Tx) (bool, error) {
	inUse, err := sqlcgen.New(db.TxOrDB(r.db, tx)).UserEmailInUse(ctx, sqlcgen.UserEmailInUseParams{
		Email:         s.Email,
		ExcludeUserID: s.ExcludeUserID,
	})
	if err != nil {
		return false, fmt.Errorf("check email in use: %w", err)
	}

	return inUse, nil
}

func (r *userDAO) SetPasswordHash(ctx context.Context, s spec.UserPasswordHashUpdate, tx ...*sql.Tx) error {
	affected, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SetUserPasswordHash(ctx, sqlcgen.SetUserPasswordHashParams{
		ID:           s.UserID,
		PasswordHash: s.PasswordHash,
	})
	if err != nil {
		return fmt.Errorf("set password hash for user %s: %w", s.UserID, err)
	}

	if affected == 0 {
		return fmt.Errorf("set password hash for user %s: %w", s.UserID, ErrNotFound)
	}

	return nil
}

func (r *userDAO) SetDisplayName(ctx context.Context, s spec.UserDisplayNameUpdate, tx ...*sql.Tx) error {
	affected, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SetUserDisplayName(ctx, sqlcgen.SetUserDisplayNameParams{
		ID:          s.UserID,
		DisplayName: s.DisplayName,
	})
	if err != nil {
		return fmt.Errorf("set display name for user %s: %w", s.UserID, err)
	}

	if affected == 0 {
		return fmt.Errorf("set display name for user %s: %w", s.UserID, ErrNotFound)
	}

	return nil
}

func (r *userDAO) MarkEmailVerified(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) error {
	affected, err := sqlcgen.New(db.TxOrDB(r.db, tx)).MarkUserEmailVerified(ctx, userID)
	if err != nil {
		return fmt.Errorf("mark email verified for user %s: %w", userID, err)
	}

	if affected == 0 {
		return fmt.Errorf("mark email verified for user %s: %w", userID, ErrNotFound)
	}

	return nil
}

func (r *userDAO) SetVerifiedEmail(ctx context.Context, s spec.UserVerifiedEmailUpdate, tx ...*sql.Tx) error {
	affected, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SetUserVerifiedEmail(ctx, sqlcgen.SetUserVerifiedEmailParams{
		ID:    s.UserID,
		Email: s.Email,
	})
	if isUniqueViolation(err) {
		return fmt.Errorf("set verified email for user %s: %w: %w", s.UserID, ErrConflict, err)
	}
	if err != nil {
		return fmt.Errorf("set verified email for user %s: %w", s.UserID, err)
	}

	if affected == 0 {
		return fmt.Errorf("set verified email for user %s: %w", s.UserID, ErrNotFound)
	}

	return nil
}

func (r *userDAO) IsBanned(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) (bool, error) {
	banned, err := sqlcgen.New(db.TxOrDB(r.db, tx)).UserIsBanned(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("check ban for user %s: %w", userID, ErrNotFound)
	}
	if err != nil {
		return false, fmt.Errorf("check ban for user %s: %w", userID, err)
	}

	return banned, nil
}

func (r *userDAO) IsLocked(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) (bool, error) {
	locked, err := sqlcgen.New(db.TxOrDB(r.db, tx)).UserIsLocked(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("check lock for user %s: %w", userID, ErrNotFound)
	}
	if err != nil {
		return false, fmt.Errorf("check lock for user %s: %w", userID, err)
	}

	return locked, nil
}

func (r *userDAO) UpdateLastSeenIP(ctx context.Context, s spec.UserLastSeenIPUpdate, tx ...*sql.Tx) error {
	affected, err := sqlcgen.New(db.TxOrDB(r.db, tx)).UpdateUserLastSeenIP(ctx, sqlcgen.UpdateUserLastSeenIPParams{
		ID:         s.UserID,
		LastSeenIp: s.IP,
	})
	if err != nil {
		return fmt.Errorf("update last seen ip for user %s: %w", s.UserID, err)
	}

	if affected == 0 {
		return fmt.Errorf("update last seen ip for user %s: %w", s.UserID, ErrNotFound)
	}

	return nil
}

func (r *userDAO) ClearLastSeenIPsBefore(ctx context.Context, cutoff time.Time, tx ...*sql.Tx) (int64, error) {
	cleared, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ClearUserLastSeenIPsBefore(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("clear last seen ips before %s: %w", cutoff.Format(time.RFC3339), err)
	}

	return cleared, nil
}

func (r *userDAO) Delete(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) error {
	affected, err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("delete user %s: %w", userID, err)
	}

	if affected == 0 {
		return fmt.Errorf("delete user %s: %w", userID, ErrNotFound)
	}

	return nil
}

func (r *userDAO) AnyExists(ctx context.Context, tx ...*sql.Tx) (bool, error) {
	exists, err := sqlcgen.New(db.TxOrDB(r.db, tx)).AnyUserExists(ctx)
	if err != nil {
		return false, fmt.Errorf("check whether any account exists: %w", err)
	}

	return exists, nil
}

func (r *userDAO) LockFirstAccountClaim(ctx context.Context, tx ...*sql.Tx) error {
	joined := db.Joined(tx)
	if joined == nil {
		return fmt.Errorf("lock the first account claim: %w", ErrTxRequired)
	}

	if err := sqlcgen.New(joined).LockFirstAccountClaim(ctx); err != nil {
		return fmt.Errorf("lock the first account claim: %w", err)
	}

	return nil
}

func (r *userDAO) ListWithRoles(ctx context.Context, s spec.UserListing, tx ...*sql.Tx) ([]model.UserWithRole, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListUsersWithRoles(ctx, sqlcgen.ListUsersWithRolesParams{
		Pattern:    containsPattern(s.Search),
		PageLimit:  int32(s.Limit),
		PageOffset: int32(s.Offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	users := make([]model.UserWithRole, 0, len(rows))
	for _, row := range rows {
		users = append(users, model.UserWithRole{User: *toUser(row.User), Role: role.Role(row.Role)})
	}

	return users, nil
}

func (r *userDAO) Count(ctx context.Context, search string, tx ...*sql.Tx) (int, error) {
	count, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CountUsers(ctx, containsPattern(search))
	if err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}

	return int(count), nil
}

func (r *userDAO) Ban(ctx context.Context, s spec.UserBan, tx ...*sql.Tx) error {
	affected, err := sqlcgen.New(db.TxOrDB(r.db, tx)).BanUser(ctx, sqlcgen.BanUserParams{
		ID:        s.UserID,
		BannedBy:  nullableUUID(s.BannedBy),
		BanReason: s.Reason,
	})
	if err != nil {
		return fmt.Errorf("ban user %s: %w", s.UserID, err)
	}

	if affected == 0 {
		return fmt.Errorf("ban user %s: %w", s.UserID, ErrNotFound)
	}

	return nil
}

func (r *userDAO) Unban(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) error {
	affected, err := sqlcgen.New(db.TxOrDB(r.db, tx)).UnbanUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("unban user %s: %w", userID, err)
	}

	if affected == 0 {
		return fmt.Errorf("unban user %s: %w", userID, ErrNotFound)
	}

	return nil
}

func (r *userDAO) Lock(ctx context.Context, s spec.UserLock, tx ...*sql.Tx) error {
	affected, err := sqlcgen.New(db.TxOrDB(r.db, tx)).LockUser(ctx, sqlcgen.LockUserParams{
		ID:         s.UserID,
		LockedBy:   nullableUUID(s.LockedBy),
		LockReason: s.Reason,
	})
	if err != nil {
		return fmt.Errorf("lock user %s: %w", s.UserID, err)
	}

	if affected == 0 {
		return fmt.Errorf("lock user %s: %w", s.UserID, ErrNotFound)
	}

	return nil
}

func (r *userDAO) Unlock(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) error {
	affected, err := sqlcgen.New(db.TxOrDB(r.db, tx)).UnlockUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("unlock user %s: %w", userID, err)
	}

	if affected == 0 {
		return fmt.Errorf("unlock user %s: %w", userID, ErrNotFound)
	}

	return nil
}

func (r *userDAO) MarkEmailUnverified(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) error {
	affected, err := sqlcgen.New(db.TxOrDB(r.db, tx)).MarkUserEmailUnverified(ctx, userID)
	if err != nil {
		return fmt.Errorf("mark email unverified for user %s: %w", userID, err)
	}

	if affected == 0 {
		return fmt.Errorf("mark email unverified for user %s: %w", userID, ErrNotFound)
	}

	return nil
}
