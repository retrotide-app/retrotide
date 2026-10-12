package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"retrotide/internal/authz"
	"retrotide/internal/config"
	"retrotide/internal/dao"
	"retrotide/internal/guest"
	"retrotide/internal/model"
	"retrotide/internal/repository"
	"retrotide/internal/session"
	"retrotide/internal/settings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	principalFixture struct {
		sessionRepo *repository.MockSessionRepository
		guestRepo   *repository.MockGuestDeviceRepository
		accounts    *authz.MockService
		guards      *Guards
		recorded    []error
	}
)

const (
	guestToken = "1a3c5e7f9b2d4f6a8c0e2b4d6f8a1c3e5b7d9f2a4c6e8b1d3f5a7c9e2b4d6f8a"
)

func newPrincipalFixture(t *testing.T) *principalFixture {
	t.Helper()

	settingsSvc := settings.NewMockService(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(testBaseURL, nil).Maybe()

	f := &principalFixture{
		sessionRepo: repository.NewMockSessionRepository(t),
		guestRepo:   repository.NewMockGuestDeviceRepository(t),
		accounts:    authz.NewMockService(t),
	}
	f.guards = NewGuards(GuardDeps{
		Settings: settingsSvc,
		Sessions: session.NewManager(f.sessionRepo, settingsSvc),
		Guests:   guest.NewManager(f.guestRepo, settingsSvc),
		Authz:    f.accounts,
	})

	return f
}

func (f *principalFixture) engine(t *testing.T, guard gin.HandlerFunc) *gin.Engine {
	t.Helper()

	engine := newTestEngine(t)
	engine.Use(func(c *gin.Context) {
		c.Next()

		for _, recorded := range c.Errors {
			f.recorded = append(f.recorded, recorded.Err)
		}
	})
	engine.GET(probeRoute, guard, principalProbe)
	engine.POST(probeRoute, guard, principalProbe)

	return engine
}

func (f *principalFixture) liveGuest() uuid.UUID {
	deviceID := uuid.New()
	f.guestRepo.EXPECT().Get(mock.Anything, guestToken).Return(&model.GuestDevice{ID: deviceID, LastSeenAt: time.Now()}, nil)

	return deviceID
}

func (f *principalFixture) liveSession() uuid.UUID {
	userID := uuid.New()
	f.sessionRepo.EXPECT().Get(mock.Anything, sessionToken).Return(&model.Session{UserID: userID}, nil)
	f.accounts.EXPECT().IsBanned(mock.Anything, userID).Return(false, nil)

	return userID
}

func principalRequest(t *testing.T, method string, session bool, guestCookie bool) *http.Request {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), method, probeRoute, nil)
	if session {
		req.AddCookie(&http.Cookie{Name: hostSessionCookie, Value: sessionToken})
	}

	if guestCookie {
		req.AddCookie(&http.Cookie{Name: hostGuestCookie, Value: guestToken})
	}

	return req
}

func TestPrincipal_PrefersTheSessionOverTheGuest(t *testing.T) {
	// given a browser holding both cookies, whose guest device the strict mock proves is never looked up
	f := newPrincipalFixture(t)
	userID := f.liveSession()
	engine := f.engine(t, f.guards.Principal)

	// when
	rec := serve(engine, principalRequest(t, http.MethodGet, true, true))

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, signedIn(userID), rec.Body.String())
}

func TestPrincipal_FallsBackToALiveGuest(t *testing.T) {
	// given
	f := newPrincipalFixture(t)
	deviceID := f.liveGuest()
	engine := f.engine(t, f.guards.Principal)

	// when
	rec := serve(engine, principalRequest(t, http.MethodPost, false, true))

	// then a guest writes without any lock gate, which belongs to accounts
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, string(model.PrincipalGuest)+":"+deviceID.String(), rec.Body.String())
}

func TestPrincipal_FallsBackToTheGuestBehindASessionThatIsGone(t *testing.T) {
	// given
	f := newPrincipalFixture(t)
	f.sessionRepo.EXPECT().Get(mock.Anything, sessionToken).Return(nil, fmt.Errorf("get session: %w", dao.ErrNotFound))
	deviceID := f.liveGuest()
	engine := f.engine(t, f.guards.Principal)

	// when
	rec := serve(engine, principalRequest(t, http.MethodGet, true, true))

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, string(model.PrincipalGuest)+":"+deviceID.String(), rec.Body.String())
}

func TestPrincipal_RefusesWhatItCannotAdmit(t *testing.T) {
	tests := []struct {
		name    string
		session bool
		guest   bool
		arrange func(f *principalFixture)
		status  int
		body    string
	}{
		{name: "no cookie at all", status: http.StatusUnauthorized, body: authRequiredBody},
		{
			name:  "a guest device that is gone",
			guest: true,
			arrange: func(f *principalFixture) {
				f.guestRepo.EXPECT().Get(mock.Anything, guestToken).Return(nil, fmt.Errorf("get guest device: %w", dao.ErrNotFound))
			},
			status: http.StatusUnauthorized,
			body:   invalidBody,
		},
		{
			name:    "a session that is gone and no guest",
			session: true,
			arrange: func(f *principalFixture) {
				f.sessionRepo.EXPECT().Get(mock.Anything, sessionToken).Return(nil, fmt.Errorf("get session: %w", dao.ErrNotFound))
			},
			status: http.StatusUnauthorized,
			body:   invalidBody,
		},
		{
			name:    "a banned user, who never falls back to a guest",
			session: true,
			guest:   true,
			arrange: func(f *principalFixture) {
				userID := uuid.New()
				f.sessionRepo.EXPECT().Get(mock.Anything, sessionToken).Return(&model.Session{UserID: userID}, nil)
				f.accounts.EXPECT().IsBanned(mock.Anything, userID).Return(true, nil)
				f.sessionRepo.EXPECT().Delete(mock.Anything, sessionToken).Return(nil)
			},
			status: http.StatusForbidden,
			body:   bannedBody,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			f := newPrincipalFixture(t)
			if tc.arrange != nil {
				tc.arrange(f)
			}
			engine := f.engine(t, f.guards.Principal)

			// when
			rec := serve(engine, principalRequest(t, http.MethodGet, tc.session, tc.guest))

			// then
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
		})
	}
}

func TestPrincipal_AnswersServiceUnavailableWhenTheGuestCannotBeChecked(t *testing.T) {
	for _, guard := range []string{"Principal", "OptionalPrincipal"} {
		t.Run(guard, func(t *testing.T) {
			// given
			f := newPrincipalFixture(t)
			f.guestRepo.EXPECT().Get(mock.Anything, guestToken).Return(nil, errDatabaseDown)
			handler := f.guards.Principal
			if guard == "OptionalPrincipal" {
				handler = f.guards.OptionalPrincipal
			}
			engine := f.engine(t, handler)

			// when
			rec := serve(engine, principalRequest(t, http.MethodGet, false, true))

			// then a database blip never hands anyone a guest identity or none at all
			assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
			assert.JSONEq(t, unavailableBody, rec.Body.String())
			require.Len(t, f.recorded, 1)
			assert.ErrorIs(t, f.recorded[0], errDatabaseDown)
		})
	}
}

func TestOptionalPrincipal_LetsAnonymousVisitorsThrough(t *testing.T) {
	tests := []struct {
		name    string
		guest   bool
		arrange func(f *principalFixture)
	}{
		{name: "no cookie"},
		{
			name:  "a guest device that is gone",
			guest: true,
			arrange: func(f *principalFixture) {
				f.guestRepo.EXPECT().Get(mock.Anything, guestToken).Return(nil, fmt.Errorf("get guest device: %w", dao.ErrNotFound))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			f := newPrincipalFixture(t)
			if tc.arrange != nil {
				tc.arrange(f)
			}
			engine := f.engine(t, f.guards.OptionalPrincipal)

			// when
			rec := serve(engine, principalRequest(t, http.MethodPost, false, tc.guest))

			// then the join route decides, and may issue a fresh guest device
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, anonymousBody, rec.Body.String())
		})
	}
}

func TestOptionalPrincipal_SetsTheGuestOrTheUser(t *testing.T) {
	// given
	f := newPrincipalFixture(t)
	deviceID := f.liveGuest()
	engine := f.engine(t, f.guards.OptionalPrincipal)

	// when
	rec := serve(engine, principalRequest(t, http.MethodGet, false, true))

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, string(model.PrincipalGuest)+":"+deviceID.String(), rec.Body.String())
}

func TestOptionalPrincipal_HoldsALockedUserToTheLockGateOnWrites(t *testing.T) {
	// given
	f := newPrincipalFixture(t)
	userID := f.liveSession()
	f.accounts.EXPECT().IsLocked(mock.Anything, userID).Return(true, nil)
	engine := f.engine(t, f.guards.OptionalPrincipal)

	// when
	rec := serve(engine, principalRequest(t, http.MethodPost, true, false))

	// then
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.JSONEq(t, lockedBody, rec.Body.String())
}
