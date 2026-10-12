package session

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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

		repo         repository.SessionRepository
		settingsSvc  settings.Service
		disconnector Disconnector
	}
)

const (
	cookieName        = "rt_session"
	sessionDay        = 24 * time.Hour
	TransientLifetime = 24 * time.Hour
)

func NewManager(repo repository.SessionRepository, settingsSvc settings.Service) *Manager {
	m := &Manager{repo: repo, settingsSvc: settingsSvc}
	m.Builder = cookie.NewBuilder(settingsSvc, cookieName, m.lifetime)

	return m
}

func (m *Manager) SetDisconnector(d Disconnector) {
	m.disconnector = d
}

func (m *Manager) Disconnect(userID uuid.UUID) {
	if m.disconnector == nil {
		return
	}

	m.disconnector.DisconnectUser(userID)
}

func (m *Manager) issue(ctx context.Context, remember bool) (string, time.Time, error) {
	drawn, err := token.New()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("draw a session token: %w", err)
	}

	if !remember {
		return drawn, time.Now().Add(TransientLifetime), nil
	}

	lifetime, err := m.lifetime(ctx)
	if err != nil {
		return "", time.Time{}, err
	}

	return drawn, time.Now().Add(lifetime), nil
}

func (m *Manager) Create(ctx context.Context, grant spec.SessionGrant) (string, error) {
	issued, expiresAt, err := m.issue(ctx, grant.Remember)
	if err != nil {
		return "", err
	}

	if err := m.repo.Create(ctx, spec.NewSession{Token: issued, UserID: grant.UserID, ExpiresAt: expiresAt}); err != nil {
		return "", err
	}

	return issued, nil
}

func (m *Manager) GrantCookie(ctx context.Context, value string, remember bool) (*http.Cookie, error) {
	if remember {
		return m.Cookie(ctx, value)
	}

	return m.BrowserCookie(ctx, value)
}

func (m *Manager) Validate(ctx context.Context, presented string) (uuid.UUID, error) {
	current, err := m.repo.Get(ctx, presented)
	if errors.Is(err, dao.ErrNotFound) {
		return uuid.Nil, fmt.Errorf("%w: %w", ErrInvalidSession, err)
	}
	if err != nil {
		return uuid.Nil, err
	}

	return current.UserID, nil
}

func (m *Manager) ValidatePrincipal(ctx context.Context, presented string) (model.Principal, error) {
	userID, err := m.Validate(ctx, presented)
	if errors.Is(err, ErrInvalidSession) {
		return model.Principal{}, fmt.Errorf("%w: %w", model.ErrInvalidPrincipal, err)
	}
	if err != nil {
		return model.Principal{}, err
	}

	return model.Principal{Kind: model.PrincipalUser, ID: userID}, nil
}

func (m *Manager) Delete(ctx context.Context, presented string) error {
	if presented == "" {
		return nil
	}

	return m.repo.Delete(ctx, presented)
}

func (m *Manager) DeleteAllForUser(ctx context.Context, userID uuid.UUID) error {
	if err := m.repo.DeleteAllForUser(ctx, userID); err != nil {
		return err
	}

	m.Disconnect(userID)

	return nil
}

func (m *Manager) DeleteAllForUserExcept(ctx context.Context, s spec.SessionDeletionExcept) error {
	if s.KeepToken == "" {
		return m.DeleteAllForUser(ctx, s.UserID)
	}

	if err := m.repo.DeleteAllForUserExcept(ctx, s); err != nil {
		return err
	}

	m.Disconnect(s.UserID)

	return nil
}

func (m *Manager) lifetime(ctx context.Context) (time.Duration, error) {
	days, err := m.settingsSvc.GetInt(ctx, config.SettingSessionDurationDays)
	if err != nil {
		return 0, fmt.Errorf("read the session duration: %w", err)
	}

	return time.Duration(days) * sessionDay, nil
}
