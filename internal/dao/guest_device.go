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
	"retrotide/internal/token"

	"github.com/google/uuid"
)

type (
	GuestDeviceDAO interface {
		Create(ctx context.Context, s spec.NewGuestDevice, tx ...*sql.Tx) (*model.GuestDevice, error)
		Get(ctx context.Context, token string, tx ...*sql.Tx) (*model.GuestDevice, error)
		Touch(ctx context.Context, deviceID uuid.UUID, tx ...*sql.Tx) error
		Delete(ctx context.Context, token string, tx ...*sql.Tx) error
		DeleteByID(ctx context.Context, deviceID uuid.UUID, tx ...*sql.Tx) error
		CleanExpired(ctx context.Context, unseenBefore time.Time, tx ...*sql.Tx) (int64, error)
	}

	guestDeviceDAO struct {
		db *sql.DB
	}
)

func toGuestDevice(row sqlcgen.GuestDevice) *model.GuestDevice {
	return &model.GuestDevice{
		ID:         row.ID,
		CreatedAt:  row.CreatedAt,
		LastSeenAt: row.LastSeenAt,
		ExpiresAt:  row.ExpiresAt,
	}
}

func (r *guestDeviceDAO) Create(ctx context.Context, s spec.NewGuestDevice, tx ...*sql.Tx) (*model.GuestDevice, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CreateGuestDevice(ctx, sqlcgen.CreateGuestDeviceParams{
		TokenHash: token.Hash(s.Token),
		ExpiresAt: s.ExpiresAt,
	})
	if isUniqueViolation(err) {
		return nil, fmt.Errorf("create guest device: %w: %w", ErrConflict, err)
	}
	if err != nil {
		return nil, fmt.Errorf("create guest device: %w", err)
	}

	return toGuestDevice(row), nil
}

func (r *guestDeviceDAO) Get(ctx context.Context, presented string, tx ...*sql.Tx) (*model.GuestDevice, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetLiveGuestDevice(ctx, token.Hash(presented))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get guest device: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get guest device: %w", err)
	}

	return toGuestDevice(row), nil
}

func (r *guestDeviceDAO) Touch(ctx context.Context, deviceID uuid.UUID, tx ...*sql.Tx) error {
	if err := sqlcgen.New(db.TxOrDB(r.db, tx)).TouchGuestDevice(ctx, deviceID); err != nil {
		return fmt.Errorf("touch guest device %s: %w", deviceID, err)
	}

	return nil
}

func (r *guestDeviceDAO) Delete(ctx context.Context, presented string, tx ...*sql.Tx) error {
	if err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteGuestDevice(ctx, token.Hash(presented)); err != nil {
		return fmt.Errorf("delete guest device: %w", err)
	}

	return nil
}

func (r *guestDeviceDAO) DeleteByID(ctx context.Context, deviceID uuid.UUID, tx ...*sql.Tx) error {
	if err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteGuestDeviceByID(ctx, deviceID); err != nil {
		return fmt.Errorf("delete guest device %s: %w", deviceID, err)
	}

	return nil
}

func (r *guestDeviceDAO) CleanExpired(ctx context.Context, unseenBefore time.Time, tx ...*sql.Tx) (int64, error) {
	removed, err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteStaleGuestDevices(ctx, unseenBefore)
	if err != nil {
		return 0, fmt.Errorf("clean expired guest devices: %w", err)
	}

	return removed, nil
}
