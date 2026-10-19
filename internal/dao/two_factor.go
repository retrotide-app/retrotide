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
	TwoFactorDAO interface {
		GetTOTP(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) (*model.UserTOTP, error)
		BeginTOTP(ctx context.Context, s spec.TwoFactorEnrolment, tx ...*sql.Tx) (*model.UserTOTP, error)
		ConfirmTOTP(ctx context.Context, s spec.TwoFactorStepUse, tx ...*sql.Tx) (*model.UserTOTP, error)
		UseTOTPStep(ctx context.Context, s spec.TwoFactorStepUse, tx ...*sql.Tx) error
		DeleteTOTP(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) error
		CreateRecoveryCode(ctx context.Context, s spec.TwoFactorRecoveryCodeUse, tx ...*sql.Tx) error
		UseRecoveryCode(ctx context.Context, s spec.TwoFactorRecoveryCodeUse, tx ...*sql.Tx) error
		CreateChallenge(ctx context.Context, s spec.NewTwoFactorChallenge, tx ...*sql.Tx) error
		GetLiveChallenge(ctx context.Context, tokenHash string, tx ...*sql.Tx) (*model.TwoFactorChallenge, error)
		AttemptChallenge(ctx context.Context, s spec.TwoFactorChallengeAttempt, tx ...*sql.Tx) (*model.TwoFactorChallenge, error)
		ConsumeChallenge(ctx context.Context, tokenHash string, tx ...*sql.Tx) (*model.TwoFactorChallenge, error)
		CleanExpiredChallenges(ctx context.Context, tx ...*sql.Tx) (int64, error)
		CreateTrustedDevice(ctx context.Context, s spec.NewTwoFactorTrustedDevice, tx ...*sql.Tx) error
		TrustedDeviceLive(ctx context.Context, s spec.TwoFactorTrustedDeviceCheck, tx ...*sql.Tx) (bool, error)
		DeleteTrustedDevicesForUser(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) error
		CleanExpiredTrustedDevices(ctx context.Context, tx ...*sql.Tx) (int64, error)
	}

	twoFactorDAO struct {
		db *sql.DB
	}
)

func toUserTOTP(row sqlcgen.UserTotp) *model.UserTOTP {
	return &model.UserTOTP{
		UserID:       row.UserID,
		Secret:       row.Secret,
		ConfirmedAt:  nullableTime(row.ConfirmedAt),
		LastUsedStep: row.LastUsedStep,
		CreatedAt:    row.CreatedAt,
	}
}

func toTwoFactorChallenge(row sqlcgen.TwoFactorChallenge) *model.TwoFactorChallenge {
	return &model.TwoFactorChallenge{
		TokenHash: row.TokenHash,
		UserID:    row.UserID,
		ReturnTo:  nullableString(row.ReturnTo),
		Remember:  row.Remember,
		Attempts:  int(row.Attempts),
		CreatedAt: row.CreatedAt,
		ExpiresAt: row.ExpiresAt,
	}
}

func (r *twoFactorDAO) GetTOTP(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) (*model.UserTOTP, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetUserTOTP(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get two-factor secret of user %s: %w", userID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get two-factor secret of user %s: %w", userID, err)
	}

	return toUserTOTP(row), nil
}

func (r *twoFactorDAO) BeginTOTP(ctx context.Context, s spec.TwoFactorEnrolment, tx ...*sql.Tx) (*model.UserTOTP, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).BeginUserTOTP(ctx, sqlcgen.BeginUserTOTPParams{
		UserID: s.UserID,
		Secret: s.Secret,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("begin two-factor enrolment of user %s: %w", s.UserID, ErrConflict)
	}
	if isForeignKeyViolation(err) {
		return nil, fmt.Errorf("begin two-factor enrolment of user %s: %w: %w", s.UserID, ErrNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("begin two-factor enrolment of user %s: %w", s.UserID, err)
	}

	return toUserTOTP(row), nil
}

func (r *twoFactorDAO) ConfirmTOTP(ctx context.Context, s spec.TwoFactorStepUse, tx ...*sql.Tx) (*model.UserTOTP, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ConfirmUserTOTP(ctx, sqlcgen.ConfirmUserTOTPParams{
		UserID:       s.UserID,
		LastUsedStep: s.Step,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("confirm two-factor enrolment of user %s: %w", s.UserID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("confirm two-factor enrolment of user %s: %w", s.UserID, err)
	}

	return toUserTOTP(row), nil
}

func (r *twoFactorDAO) UseTOTPStep(ctx context.Context, s spec.TwoFactorStepUse, tx ...*sql.Tx) error {
	used, err := sqlcgen.New(db.TxOrDB(r.db, tx)).UseUserTOTPStep(ctx, sqlcgen.UseUserTOTPStepParams{
		UserID:       s.UserID,
		LastUsedStep: s.Step,
	})
	if err != nil {
		return fmt.Errorf("use two-factor step %d of user %s: %w", s.Step, s.UserID, err)
	}
	if used == 0 {
		return fmt.Errorf("use two-factor step %d of user %s: %w", s.Step, s.UserID, ErrConflict)
	}

	return nil
}

func (r *twoFactorDAO) DeleteTOTP(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) error {
	removed, err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteUserTOTP(ctx, userID)
	if err != nil {
		return fmt.Errorf("delete two-factor secret of user %s: %w", userID, err)
	}
	if removed == 0 {
		return fmt.Errorf("delete two-factor secret of user %s: %w", userID, ErrNotFound)
	}

	return nil
}

func (r *twoFactorDAO) CreateRecoveryCode(ctx context.Context, s spec.TwoFactorRecoveryCodeUse, tx ...*sql.Tx) error {
	err := sqlcgen.New(db.TxOrDB(r.db, tx)).CreateTwoFactorRecoveryCode(ctx, sqlcgen.CreateTwoFactorRecoveryCodeParams{
		UserID:   s.UserID,
		CodeHash: s.CodeHash,
	})
	if isUniqueViolation(err) {
		return fmt.Errorf("create two-factor recovery code for user %s: %w: %w", s.UserID, ErrConflict, err)
	}
	if isForeignKeyViolation(err) {
		return fmt.Errorf("create two-factor recovery code for user %s: %w: %w", s.UserID, ErrNotFound, err)
	}
	if err != nil {
		return fmt.Errorf("create two-factor recovery code for user %s: %w", s.UserID, err)
	}

	return nil
}

func (r *twoFactorDAO) UseRecoveryCode(ctx context.Context, s spec.TwoFactorRecoveryCodeUse, tx ...*sql.Tx) error {
	used, err := sqlcgen.New(db.TxOrDB(r.db, tx)).UseTwoFactorRecoveryCode(ctx, sqlcgen.UseTwoFactorRecoveryCodeParams{
		UserID:   s.UserID,
		CodeHash: s.CodeHash,
	})
	if err != nil {
		return fmt.Errorf("use a two-factor recovery code of user %s: %w", s.UserID, err)
	}
	if used == 0 {
		return fmt.Errorf("use a two-factor recovery code of user %s: %w", s.UserID, ErrNotFound)
	}

	return nil
}

func (r *twoFactorDAO) CreateChallenge(ctx context.Context, s spec.NewTwoFactorChallenge, tx ...*sql.Tx) error {
	err := sqlcgen.New(db.TxOrDB(r.db, tx)).CreateTwoFactorChallenge(ctx, sqlcgen.CreateTwoFactorChallengeParams{
		TokenHash: s.TokenHash,
		UserID:    s.UserID,
		ReturnTo:  textOrNull(s.ReturnTo),
		Remember:  s.Remember,
		ExpiresAt: s.ExpiresAt,
	})
	if isUniqueViolation(err) {
		return fmt.Errorf("create two-factor challenge for user %s: %w: %w", s.UserID, ErrConflict, err)
	}
	if err != nil {
		return fmt.Errorf("create two-factor challenge for user %s: %w", s.UserID, err)
	}

	return nil
}

func (r *twoFactorDAO) GetLiveChallenge(ctx context.Context, tokenHash string, tx ...*sql.Tx) (*model.TwoFactorChallenge, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetLiveTwoFactorChallenge(ctx, tokenHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get two-factor challenge: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get two-factor challenge: %w", err)
	}

	return toTwoFactorChallenge(row), nil
}

func (r *twoFactorDAO) AttemptChallenge(ctx context.Context, s spec.TwoFactorChallengeAttempt, tx ...*sql.Tx) (*model.TwoFactorChallenge, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).AttemptTwoFactorChallenge(ctx, sqlcgen.AttemptTwoFactorChallengeParams{
		TokenHash: s.TokenHash,
		Attempts:  clampInt32(s.MaxAttempts),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("attempt two-factor challenge: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("attempt two-factor challenge: %w", err)
	}

	return toTwoFactorChallenge(row), nil
}

func (r *twoFactorDAO) ConsumeChallenge(ctx context.Context, tokenHash string, tx ...*sql.Tx) (*model.TwoFactorChallenge, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ConsumeTwoFactorChallenge(ctx, tokenHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("consume two-factor challenge: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("consume two-factor challenge: %w", err)
	}

	return toTwoFactorChallenge(row), nil
}

func (r *twoFactorDAO) CleanExpiredChallenges(ctx context.Context, tx ...*sql.Tx) (int64, error) {
	removed, err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteExpiredTwoFactorChallenges(ctx)
	if err != nil {
		return 0, fmt.Errorf("clean expired two-factor challenges: %w", err)
	}

	return removed, nil
}

func (r *twoFactorDAO) CreateTrustedDevice(ctx context.Context, s spec.NewTwoFactorTrustedDevice, tx ...*sql.Tx) error {
	err := sqlcgen.New(db.TxOrDB(r.db, tx)).CreateTwoFactorTrustedDevice(ctx, sqlcgen.CreateTwoFactorTrustedDeviceParams{
		TokenHash: s.TokenHash,
		UserID:    s.UserID,
		ExpiresAt: s.ExpiresAt,
	})
	if isUniqueViolation(err) {
		return fmt.Errorf("trust a device for user %s: %w: %w", s.UserID, ErrConflict, err)
	}
	if isForeignKeyViolation(err) {
		return fmt.Errorf("trust a device for user %s: %w: %w", s.UserID, ErrNotFound, err)
	}
	if err != nil {
		return fmt.Errorf("trust a device for user %s: %w", s.UserID, err)
	}

	return nil
}

func (r *twoFactorDAO) TrustedDeviceLive(ctx context.Context, s spec.TwoFactorTrustedDeviceCheck, tx ...*sql.Tx) (bool, error) {
	live, err := sqlcgen.New(db.TxOrDB(r.db, tx)).TwoFactorTrustedDeviceLive(ctx, sqlcgen.TwoFactorTrustedDeviceLiveParams{
		TokenHash: s.TokenHash,
		UserID:    s.UserID,
	})
	if err != nil {
		return false, fmt.Errorf("check a trusted device of user %s: %w", s.UserID, err)
	}

	return live, nil
}

func (r *twoFactorDAO) DeleteTrustedDevicesForUser(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) error {
	if err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteTwoFactorTrustedDevicesForUser(ctx, userID); err != nil {
		return fmt.Errorf("forget the trusted devices of user %s: %w", userID, err)
	}

	return nil
}

func (r *twoFactorDAO) CleanExpiredTrustedDevices(ctx context.Context, tx ...*sql.Tx) (int64, error) {
	removed, err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteExpiredTwoFactorTrustedDevices(ctx)
	if err != nil {
		return 0, fmt.Errorf("clean expired trusted devices: %w", err)
	}

	return removed, nil
}
