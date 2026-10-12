package guest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"retrotide/internal/config"
	"retrotide/internal/cookie"
	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"
	"retrotide/internal/settings"
	"retrotide/internal/token"

	"github.com/google/uuid"
)

type (
	Disconnector interface {
		DisconnectUser(userID uuid.UUID) int
	}

	Manager struct {
		*cookie.Builder

		repo         repository.GuestDeviceRepository
		settingsSvc  settings.Service
		disconnector Disconnector
	}
)

const (
	CookieName = "rt_guest"

	deviceDay  = 24 * time.Hour
	touchEvery = time.Hour
)

func NewManager(repo repository.GuestDeviceRepository, settingsSvc settings.Service) *Manager {
	m := &Manager{repo: repo, settingsSvc: settingsSvc}
	m.Builder = cookie.NewBuilder(settingsSvc, CookieName, m.lifetime)

	return m
}

func (m *Manager) SetDisconnector(d Disconnector) {
	m.disconnector = d
}

func (m *Manager) Disconnect(deviceID uuid.UUID) {
	if m.disconnector == nil {
		return
	}

	m.disconnector.DisconnectUser(deviceID)
}

func (m *Manager) Create(ctx context.Context) (model.GuestCredential, error) {
	drawn, err := token.New()
	if err != nil {
		return model.GuestCredential{}, fmt.Errorf("draw a guest device token: %w", err)
	}

	lifetime, err := m.lifetime(ctx)
	if err != nil {
		return model.GuestCredential{}, err
	}

	device, err := m.repo.Create(ctx, spec.NewGuestDevice{Token: drawn, ExpiresAt: time.Now().Add(lifetime)})
	if err != nil {
		return model.GuestCredential{}, err
	}

	return model.GuestCredential{Token: drawn, DeviceID: device.ID}, nil
}

func (m *Manager) Validate(ctx context.Context, presented string) (uuid.UUID, error) {
	device, err := m.repo.Get(ctx, presented)
	if errors.Is(err, dao.ErrNotFound) {
		return uuid.Nil, fmt.Errorf("%w: %w", ErrInvalidGuest, err)
	}
	if err != nil {
		return uuid.Nil, err
	}

	if time.Since(device.LastSeenAt) < touchEvery {
		return device.ID, nil
	}

	if err := m.repo.Touch(ctx, device.ID); err != nil {
		return uuid.Nil, err
	}

	return device.ID, nil
}

func (m *Manager) ValidatePrincipal(ctx context.Context, presented string) (model.Principal, error) {
	deviceID, err := m.Validate(ctx, presented)
	if errors.Is(err, ErrInvalidGuest) {
		return model.Principal{}, fmt.Errorf("%w: %w", model.ErrInvalidPrincipal, err)
	}
	if err != nil {
		return model.Principal{}, err
	}

	return model.Principal{Kind: model.PrincipalGuest, ID: deviceID}, nil
}

func (m *Manager) Delete(ctx context.Context, presented string) error {
	if presented == "" {
		return nil
	}

	return m.repo.Delete(ctx, presented)
}

func (m *Manager) Retire(ctx context.Context, deviceID uuid.UUID) error {
	if err := m.repo.DeleteByID(ctx, deviceID); err != nil {
		return err
	}

	m.Disconnect(deviceID)

	return nil
}

func (m *Manager) lifetime(ctx context.Context) (time.Duration, error) {
	days, err := m.settingsSvc.GetInt(ctx, config.SettingGuestDeviceDays)
	if err != nil {
		return 0, fmt.Errorf("read the guest device lifetime: %w", err)
	}

	return time.Duration(days) * deviceDay, nil
}
